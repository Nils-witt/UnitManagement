package server

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"time"
	"unicode/utf8"

	"go-unit-mangement/internal/audit"
	"go-unit-mangement/internal/auth"
	"go-unit-mangement/internal/models"
)

type auditLogResponse struct {
	ID        uint      `json:"id"`
	CreatedAt time.Time `json:"createdAt"`
	Action    string    `json:"action"`
	// Actor is null for anonymous actions and once the user is deleted;
	// ActorName still names them as of the action.
	Actor      *userRefResponse `json:"actor"`
	ActorName  string           `json:"actorName"`
	TargetType string           `json:"targetType"`
	TargetID   string           `json:"targetId"`
	TargetName string           `json:"targetName"`
	Details    map[string]any   `json:"details"`
	RemoteAddr string           `json:"remoteAddr"`
}

func toAuditLogResponse(e *models.AuditLog) auditLogResponse {
	resp := auditLogResponse{
		ID:         e.ID,
		CreatedAt:  e.CreatedAt,
		Action:     e.Action,
		ActorName:  e.ActorName,
		TargetType: e.TargetType,
		TargetID:   e.TargetID,
		TargetName: e.TargetName,
		Details:    e.Details,
		RemoteAddr: e.RemoteAddr,
	}
	if e.ActorID != nil {
		resp.Actor = &userRefResponse{ID: *e.ActorID, Username: e.ActorName}
	}
	if resp.Details == nil {
		resp.Details = map[string]any{}
	}
	return resp
}

// record adds e to the audit log with the request's client address and, unless
// e names one, the signed-in user as the actor.
func (s *Server) record(r *http.Request, e audit.Entry) {
	if e.Actor == nil {
		e.Actor = auth.UserFromContext(r.Context())
	}
	if addr, ok := parseIP(r.RemoteAddr); ok {
		e.RemoteAddr = addr.String()
	}
	s.audit.Record(r.Context(), e)
}

// truncate cuts s to at most n runes, so an anonymous client can't store
// arbitrarily long values in the audit log.
func truncate(s string, n int) string {
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	return string([]rune(s)[:n])
}

// parseAuditFilter reads the optional limit, before, action, actor,
// targetType, targetId, since and to parameters of the audit log.
func parseAuditFilter(q url.Values) (audit.Filter, error) {
	f := audit.Filter{
		Action:     audit.Action(q.Get("action")),
		TargetType: q.Get("targetType"),
		TargetID:   q.Get("targetId"),
	}
	if v := q.Get("limit"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 || n > audit.MaxLimit {
			return f, fmt.Errorf("limit must be between 1 and %d", audit.MaxLimit)
		}
		f.Limit = n
	}
	for _, p := range []struct {
		name string
		dst  *uint
	}{{"before", &f.BeforeID}, {"actor", &f.ActorID}} {
		if v := q.Get(p.name); v != "" {
			n, err := strconv.ParseUint(v, 10, 0)
			if err != nil || n == 0 {
				return f, fmt.Errorf("%s must be a positive integer", p.name)
			}
			*p.dst = uint(n)
		}
	}
	for _, p := range []struct {
		name string
		dst  *time.Time
	}{{"since", &f.Since}, {"to", &f.To}} {
		if v := q.Get(p.name); v != "" {
			t, err := time.Parse(time.RFC3339, v)
			if err != nil {
				return f, fmt.Errorf("%s must be an RFC 3339 timestamp", p.name)
			}
			*p.dst = t
		}
	}
	if !f.Since.IsZero() && !f.To.IsZero() && f.To.Before(f.Since) {
		return f, errors.New("to must not be before since")
	}
	return f, nil
}

func (s *Server) handleListAuditLog(w http.ResponseWriter, r *http.Request) {
	f, err := parseAuditFilter(r.URL.Query())
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	entries, err := s.audit.List(r.Context(), f)
	if err != nil {
		slog.Error("list audit log", "err", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	resp := make([]auditLogResponse, len(entries))
	for i := range entries {
		resp[i] = toAuditLogResponse(&entries[i])
	}
	writeJSON(w, http.StatusOK, resp)
}
