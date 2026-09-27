package server

import (
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/google/uuid"

	"go-unit-mangement/internal/audit"
	"go-unit-mangement/internal/auth"
	"go-unit-mangement/internal/models"
	"go-unit-mangement/internal/remotesync"
)

// syncIdentityResponse is what an administrator of another instance needs to
// let this one sync from it: register PublicKeyPEM as an API key with ID
// ServerUUID.
type syncIdentityResponse struct {
	ServerUUID   uuid.UUID `json:"serverUuid"`
	PublicKeyPEM string    `json:"publicKeyPem"`
}

func (s *Server) handleSyncIdentity(w http.ResponseWriter, r *http.Request) {
	pem, err := s.sync.PublicKeyPEM()
	if err != nil {
		slog.Error("sync identity", "err", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusOK, syncIdentityResponse{ServerUUID: s.sync.ServerUUID(), PublicKeyPEM: pem})
}

type syncRemoteRequest struct {
	Name            string `json:"name"`
	BaseURL         string `json:"baseUrl"`
	PollIntervalSec int    `json:"pollIntervalSec"`
	Enabled         bool   `json:"enabled"`
}

func (req *syncRemoteRequest) input() remotesync.RemoteInput {
	return remotesync.RemoteInput{Name: req.Name, BaseURL: req.BaseURL, PollIntervalSec: req.PollIntervalSec, Enabled: req.Enabled}
}

type syncRemoteResponse struct {
	ID              uuid.UUID `json:"id"`
	Name            string    `json:"name"`
	BaseURL         string    `json:"baseUrl"`
	PollIntervalSec int       `json:"pollIntervalSec"`
	Enabled         bool      `json:"enabled"`
	// LastSyncAt is null before the first full sync; LastSyncStatus is then
	// empty, else "ok" or "error" with LastSyncError saying why.
	LastSyncAt     *time.Time `json:"lastSyncAt"`
	LastSyncStatus string     `json:"lastSyncStatus"`
	LastSyncError  string     `json:"lastSyncError"`
	CreatedAt      time.Time  `json:"createdAt"`
	UpdatedAt      time.Time  `json:"updatedAt"`
}

func toSyncRemoteResponse(r *models.SyncRemote) syncRemoteResponse {
	return syncRemoteResponse{
		ID: r.ID, Name: r.Name, BaseURL: r.BaseURL, PollIntervalSec: r.PollIntervalSec, Enabled: r.Enabled,
		LastSyncAt: r.LastSyncAt, LastSyncStatus: r.LastSyncStatus, LastSyncError: r.LastSyncError,
		CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt,
	}
}

func (s *Server) handleListSyncRemotes(w http.ResponseWriter, r *http.Request) {
	remotes, err := s.sync.List(r.Context())
	if err != nil {
		writeSyncError(w, err)
		return
	}
	resp := make([]syncRemoteResponse, len(remotes))
	for i := range remotes {
		resp[i] = toSyncRemoteResponse(&remotes[i])
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handleGetSyncRemote(w http.ResponseWriter, r *http.Request) {
	id, ok := syncRemoteIDFromPath(w, r)
	if !ok {
		return
	}
	remote, err := s.sync.Get(r.Context(), id)
	if err != nil {
		writeSyncError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, toSyncRemoteResponse(remote))
}

func (s *Server) handleCreateSyncRemote(w http.ResponseWriter, r *http.Request) {
	var req syncRemoteRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	remote, err := s.sync.Create(r.Context(), req.input())
	if err != nil {
		writeSyncError(w, err)
		return
	}
	slog.Info("sync remote created", "by", auth.UserFromContext(r.Context()).Username, "remote", remote.Name, "id", remote.ID)
	s.record(r, audit.Entry{
		Action: audit.ActionSyncRemoteCreate, TargetType: audit.TargetSyncRemote, TargetID: remote.ID.String(), TargetName: remote.Name,
		Details: map[string]any{"baseUrl": remote.BaseURL, "enabled": remote.Enabled},
	})
	writeJSON(w, http.StatusCreated, toSyncRemoteResponse(remote))
}

func (s *Server) handleUpdateSyncRemote(w http.ResponseWriter, r *http.Request) {
	id, ok := syncRemoteIDFromPath(w, r)
	if !ok {
		return
	}
	var req syncRemoteRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	remote, err := s.sync.Update(r.Context(), id, req.input())
	if err != nil {
		writeSyncError(w, err)
		return
	}
	slog.Info("sync remote updated", "by", auth.UserFromContext(r.Context()).Username, "remote", remote.Name, "id", remote.ID)
	s.record(r, audit.Entry{
		Action: audit.ActionSyncRemoteUpdate, TargetType: audit.TargetSyncRemote, TargetID: remote.ID.String(), TargetName: remote.Name,
		Details: map[string]any{"baseUrl": remote.BaseURL, "enabled": remote.Enabled},
	})
	writeJSON(w, http.StatusOK, toSyncRemoteResponse(remote))
}

func (s *Server) handleDeleteSyncRemote(w http.ResponseWriter, r *http.Request) {
	id, ok := syncRemoteIDFromPath(w, r)
	if !ok {
		return
	}
	remote, err := s.sync.Delete(r.Context(), id)
	if err != nil {
		writeSyncError(w, err)
		return
	}
	slog.Info("sync remote deleted", "by", auth.UserFromContext(r.Context()).Username, "remote", remote.Name, "id", remote.ID)
	s.record(r, audit.Entry{Action: audit.ActionSyncRemoteDelete, TargetType: audit.TargetSyncRemote, TargetID: remote.ID.String(), TargetName: remote.Name})
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleTriggerSyncRemote(w http.ResponseWriter, r *http.Request) {
	id, ok := syncRemoteIDFromPath(w, r)
	if !ok {
		return
	}
	if err := s.sync.Trigger(r.Context(), id); err != nil {
		writeSyncError(w, err)
		return
	}
	s.record(r, audit.Entry{Action: audit.ActionSyncRemoteTrigger, TargetType: audit.TargetSyncRemote, TargetID: id.String()})
	w.WriteHeader(http.StatusAccepted)
}

func (s *Server) handleSyncRemoteLogs(w http.ResponseWriter, r *http.Request) {
	id, ok := syncRemoteIDFromPath(w, r)
	if !ok {
		return
	}
	if _, err := s.sync.Get(r.Context(), id); err != nil {
		writeSyncError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, s.sync.Logs(id))
}

func syncRemoteIDFromPath(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusNotFound, remotesync.ErrRemoteNotFound.Error())
		return uuid.Nil, false
	}
	return id, true
}

// writeSyncError maps sync remote errors to responses; anything unexpected is
// logged and reported as a generic 500.
func writeSyncError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, remotesync.ErrRemoteNotFound):
		writeError(w, http.StatusNotFound, err.Error())
	case errors.Is(err, remotesync.ErrRemoteNotRunning):
		writeError(w, http.StatusConflict, err.Error())
	case errors.Is(err, remotesync.ErrInvalidName), errors.Is(err, remotesync.ErrInvalidBaseURL),
		errors.Is(err, remotesync.ErrInvalidPollInterval):
		writeError(w, http.StatusBadRequest, err.Error())
	default:
		slog.Error("sync remote management", "err", err)
		writeError(w, http.StatusInternalServerError, "internal error")
	}
}
