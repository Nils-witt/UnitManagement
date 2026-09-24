package server

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
	"github.com/google/uuid"

	"go-unit-mangement/internal/auth"
	"go-unit-mangement/internal/units"
)

const (
	eventsWriteTimeout = 10 * time.Second
	eventsPingInterval = 30 * time.Second
	// eventsSessionCheck is how often a stream re-validates its session, so
	// a logout or expiry ends it too.
	eventsSessionCheck = time.Minute
)

// unitEventMessage is one message on the unit event stream. Unit is set for
// created and updated, ID for every type.
type unitEventMessage struct {
	Type units.EventType `json:"type"`
	ID   uuid.UUID       `json:"id"`
	Unit *unitResponse   `json:"unit,omitempty"`
}

// handleUnitEvents upgrades to a WebSocket and pushes every unit change until
// the client leaves, the session ends or the server shuts down. The client
// only receives; anything it sends is ignored.
func (s *Server) handleUnitEvents(w http.ResponseWriter, r *http.Request) {
	s.streams.Add(1)
	defer s.streams.Done()

	log := slog.With("user", auth.UserFromContext(r.Context()).Username, "remote", r.RemoteAddr)

	// Subscribe before the upgrade so no change between the client's list
	// fetch and the first message is missed.
	events, unsubscribe := s.units.Subscribe()
	defer unsubscribe()

	// The client authenticates with the subprotocols "bearer, <token>"; the
	// browser fails the handshake unless the server selects one of them.
	// Accept also rejects cross-origin upgrades.
	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{Subprotocols: []string{auth.WebSocketProtocol}})
	if err != nil {
		log.Warn("unit event stream rejected", "origin", r.Header.Get("Origin"), "err", err)
		return
	}
	// Closing twice is harmless and a failed close has no one left to tell.
	defer func() { _ = conn.CloseNow() }()

	start := time.Now()
	log.Info("unit event stream opened")
	end := s.streamUnitEvents(conn, r, events)
	attrs := []any{"reason", end.reason, "duration", time.Since(start), "events", end.sent}
	if end.err != nil && !clientGone(end.err) {
		log.Warn("unit event stream closed", append(attrs, "err", end.err)...)
		return
	}
	log.Info("unit event stream closed", attrs...)
}

// streamEnd tells why a unit event stream ended.
type streamEnd struct {
	reason string
	// sent counts the events delivered before the end.
	sent int
	err  error
}

func (s *Server) streamUnitEvents(conn *websocket.Conn, r *http.Request, events <-chan units.Event) streamEnd {
	// ctx ends when the client closes or drops the connection.
	ctx := conn.CloseRead(r.Context())

	token := auth.TokenFromRequest(r)
	ping := time.NewTicker(eventsPingInterval)
	defer ping.Stop()
	sessionCheck := time.NewTicker(eventsSessionCheck)
	defer sessionCheck.Stop()

	sent := 0
	for {
		select {
		case <-ctx.Done():
			return streamEnd{reason: "client closed", sent: sent}
		case <-s.shutdown:
			_ = conn.Close(websocket.StatusGoingAway, "server shutting down")
			return streamEnd{reason: "server shutting down", sent: sent}
		case ev, ok := <-events:
			if !ok {
				// Fell behind and missed events; the client resyncs on reconnect.
				_ = conn.Close(websocket.StatusTryAgainLater, "too slow")
				return streamEnd{reason: "client too slow", sent: sent}
			}
			if err := s.writeUnitEvent(ctx, conn, ev); err != nil {
				return streamEnd{reason: "write failed", sent: sent, err: err}
			}
			sent++
		case <-ping.C:
			pingCtx, cancelPing := context.WithTimeout(ctx, eventsWriteTimeout)
			err := conn.Ping(pingCtx)
			cancelPing()
			if err != nil {
				return streamEnd{reason: "ping failed", sent: sent, err: err}
			}
		case <-sessionCheck.C:
			if _, err := s.auth.UserForToken(ctx, token); err != nil {
				_ = conn.Close(websocket.StatusPolicyViolation, "session ended")
				return streamEnd{reason: "session ended", sent: sent}
			}
		}
	}
}

func (s *Server) writeUnitEvent(ctx context.Context, conn *websocket.Conn, ev units.Event) error {
	msg := unitEventMessage{Type: ev.Type, ID: ev.ID}
	if ev.Unit != nil {
		resp := toUnitResponse(ev.Unit)
		msg.Unit = &resp
	}
	ctx, cancel := context.WithTimeout(ctx, eventsWriteTimeout)
	defer cancel()
	return wsjson.Write(ctx, conn, msg)
}

// clientGone reports whether err only means the client went away.
func clientGone(err error) bool {
	return errors.Is(err, context.Canceled) || websocket.CloseStatus(err) != -1
}
