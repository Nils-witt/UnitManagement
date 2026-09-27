package server

import (
	"errors"
	"log/slog"
	"net/http"

	"go-unit-mangement/internal/audit"
	"go-unit-mangement/internal/auth"
	"go-unit-mangement/internal/settings"
)

// settingsBody is both the response and the request body; empty fields mean
// the default.
type settingsBody struct {
	MapStyleURL string `json:"mapStyleUrl"`
}

func (s *Server) handleGetSettings(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, settingsBody{MapStyleURL: s.settings.Get().MapStyleURL})
}

func (s *Server) handleUpdateSettings(w http.ResponseWriter, r *http.Request) {
	var req settingsBody
	if !decodeJSON(w, r, &req) {
		return
	}
	changed, err := s.settings.Update(r.Context(), settings.Settings{MapStyleURL: req.MapStyleURL})
	switch {
	case errors.Is(err, settings.ErrInvalid):
		writeError(w, http.StatusBadRequest, err.Error())
		return
	case err != nil:
		slog.Error("update settings", "err", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	if len(changed) > 0 {
		slog.Info("settings updated", "by", auth.UserFromContext(r.Context()).Username, "changed", changed)
		s.record(r, audit.Entry{Action: audit.ActionSettingsUpdate, Details: map[string]any{"changed": changed}})
	}
	s.handleGetSettings(w, r)
}
