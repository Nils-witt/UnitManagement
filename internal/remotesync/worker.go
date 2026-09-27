package remotesync

import (
	"context"
	"errors"
	"fmt"
	"math/rand/v2"
	"sync"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
	"github.com/google/uuid"

	"go-unit-mangement/internal/models"
	"go-unit-mangement/internal/units"
)

// Bounds of the event stream's reconnect backoff, as in the web UI.
const (
	listenerMinRetry = time.Second
	listenerMaxRetry = 30 * time.Second
)

// unitStore is what a worker needs of units.Service.
type unitStore interface {
	UpsertSynced(ctx context.Context, remoteID, id uuid.UUID, in units.Input) (bool, error)
	DeleteSynced(ctx context.Context, remoteID, id uuid.UUID) (bool, error)
	SyncedIDs(ctx context.Context, remoteID uuid.UUID) ([]uuid.UUID, error)
}

// worker mirrors one remote's units: a full pass on start, every poll
// interval and on demand, and in between every change the remote pushes on
// its event stream.
type worker struct {
	remote models.SyncRemote
	client *Client
	units  unitStore
	logs   *logStore
	// status records a full pass's outcome; nil in tests.
	status func(ctx context.Context, err error)
	// trigger asks for a full pass; it holds at most one pending request.
	trigger chan struct{}

	// mu serializes full passes and applied events. An event waiting for a
	// pass to finish is applied after it, so the pass's older snapshot of the
	// unit can't overwrite the newer event.
	mu sync.Mutex
}

// run syncs until ctx is canceled.
func (w *worker) run(ctx context.Context) {
	interval := time.Duration(w.remote.PollIntervalSec) * time.Second
	w.logs.infof(w.remote.ID, "worker for %s started, full sync every %s", w.remote.Name, interval)

	var wg sync.WaitGroup
	wg.Go(func() { w.listen(ctx) })
	defer wg.Wait()

	w.fullSync(ctx)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			w.logs.infof(w.remote.ID, "worker for %s stopped", w.remote.Name)
			return
		case <-ticker.C:
			w.fullSync(ctx)
		case <-w.trigger:
			w.fullSync(ctx)
		}
	}
}

// requestSync queues a full pass unless one is already pending.
func (w *worker) requestSync() bool {
	select {
	case w.trigger <- struct{}{}:
		return true
	default:
		return false
	}
}

// fullSync mirrors the remote's complete unit list and records the outcome.
func (w *worker) fullSync(ctx context.Context) {
	start := time.Now()
	err := w.syncOnce(ctx)
	if ctx.Err() != nil {
		return
	}
	if err != nil {
		w.logs.errorf(w.remote.ID, "sync failed after %s: %v", time.Since(start).Round(time.Millisecond), err)
	} else {
		w.logs.infof(w.remote.ID, "sync complete in %s", time.Since(start).Round(time.Millisecond))
	}
	if w.status != nil {
		w.status(ctx, err)
	}
}

// syncOnce upserts every unit the remote lists and deletes the mirrored ones
// it no longer lists. A unit that fails doesn't stop the others; the errors
// are returned together.
func (w *worker) syncOnce(ctx context.Context) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	list, err := w.client.ListUnits(ctx)
	if err != nil {
		return fmt.Errorf("list remote units: %w", err)
	}

	var errs []error
	listed := make(map[uuid.UUID]bool, len(list))
	changed := 0
	for i := range list {
		u := &list[i]
		listed[u.ID] = true
		ok, err := w.units.UpsertSynced(ctx, w.remote.ID, u.ID, u.Input())
		switch {
		case errors.Is(err, units.ErrNotMirrored):
			// The remote lists a unit of ours, or of another remote, e.g.
			// because it syncs from here too. Not an error.
		case err != nil:
			errs = append(errs, fmt.Errorf("unit %q (%s): %w", u.Name, u.ID, err))
			w.logs.errorf(w.remote.ID, "unit %q (%s): %v", u.Name, u.ID, err)
		case ok:
			changed++
		}
	}

	mirrored, err := w.units.SyncedIDs(ctx, w.remote.ID)
	if err != nil {
		return errors.Join(append(errs, err)...)
	}
	deleted := 0
	for _, id := range mirrored {
		if listed[id] {
			continue
		}
		if _, err := w.units.DeleteSynced(ctx, w.remote.ID, id); err != nil {
			errs = append(errs, err)
			w.logs.errorf(w.remote.ID, "delete unit %s: %v", id, err)
			continue
		}
		deleted++
	}
	if changed > 0 || deleted > 0 {
		w.logs.infof(w.remote.ID, "%d unit(s) listed, %d created or updated, %d deleted", len(list), changed, deleted)
	}
	return errors.Join(errs...)
}

// listen follows the remote's unit event stream until ctx is canceled,
// applying each change as it arrives and reconnecting with backoff when the
// connection fails. Every (re)connect requests a full pass, which catches up
// on whatever changed while it was down. The poll interval stays the
// fallback when the stream is unavailable.
func (w *worker) listen(ctx context.Context) {
	backoff := listenerMinRetry
	for ctx.Err() == nil {
		conn, err := w.client.DialEvents(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			w.logs.errorf(w.remote.ID, "event stream: %v", err)
			if !sleep(ctx, jitter(backoff)) {
				return
			}
			backoff = min(backoff*2, listenerMaxRetry)
			continue
		}
		backoff = listenerMinRetry
		w.requestSync()
		err = w.readEvents(ctx, conn)
		_ = conn.CloseNow()
		if ctx.Err() != nil {
			return
		}
		// The remote ends the stream when the JWT it was opened with expires
		// (see tokenTTL); that's routine, so it isn't logged.
		if websocket.CloseStatus(err) != websocket.StatusPolicyViolation {
			w.logs.infof(w.remote.ID, "event stream closed (%v), reconnecting", err)
		}
		if !sleep(ctx, jitter(listenerMinRetry)) {
			return
		}
	}
}

// readEvents applies the events read from conn until it fails.
func (w *worker) readEvents(ctx context.Context, conn *websocket.Conn) error {
	for {
		var ev remoteEvent
		if err := wsjson.Read(ctx, conn, &ev); err != nil {
			return err
		}
		w.apply(ctx, ev)
	}
}

// apply mirrors a single change the remote pushed.
func (w *worker) apply(ctx context.Context, ev remoteEvent) {
	w.mu.Lock()
	defer w.mu.Unlock()

	var err error
	switch ev.Type {
	case units.EventCreated, units.EventUpdated:
		if ev.Unit == nil {
			return
		}
		_, err = w.units.UpsertSynced(ctx, w.remote.ID, ev.Unit.ID, ev.Unit.Input())
		if errors.Is(err, units.ErrNotMirrored) {
			err = nil
		}
	case units.EventDeleted:
		_, err = w.units.DeleteSynced(ctx, w.remote.ID, ev.ID)
	default:
		return
	}
	if err != nil && ctx.Err() == nil {
		w.logs.errorf(w.remote.ID, "apply %s event for unit %s: %v", ev.Type, ev.ID, err)
	}
}

// sleep waits for d, reporting false if ctx is canceled first.
func sleep(ctx context.Context, d time.Duration) bool {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-t.C:
		return true
	}
}

// jitter adds up to 20% to d, so workers reconnecting to a restarted remote
// don't retry in lockstep.
func jitter(d time.Duration) time.Duration {
	return d + rand.N(d/5+1) //nolint:gosec // timing jitter, not security-sensitive
}
