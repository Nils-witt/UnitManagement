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
)

type createAPIKeyRequest struct {
	// ID is the JWT "kid" the key's owner signs with; for another instance
	// syncing from this one, its server UUID.
	ID           string `json:"id"`
	Name         string `json:"name"`
	PublicKeyPEM string `json:"publicKeyPem"`
}

type apiKeyResponse struct {
	ID           uuid.UUID  `json:"id"`
	Name         string     `json:"name"`
	PublicKeyPEM string     `json:"publicKeyPem"`
	CreatedAt    time.Time  `json:"createdAt"`
	LastUsedAt   *time.Time `json:"lastUsedAt"`
}

func toAPIKeyResponse(k *models.APIKey) apiKeyResponse {
	return apiKeyResponse{ID: k.ID, Name: k.Name, PublicKeyPEM: k.PublicKeyPEM, CreatedAt: k.CreatedAt, LastUsedAt: k.LastUsedAt}
}

func (s *Server) handleListAPIKeys(w http.ResponseWriter, r *http.Request) {
	id, ok := userIDFromPath(w, r)
	if !ok {
		return
	}
	keys, err := s.auth.ListAPIKeys(r.Context(), id)
	if err != nil {
		writeAPIKeyError(w, err)
		return
	}
	resp := make([]apiKeyResponse, len(keys))
	for i := range keys {
		resp[i] = toAPIKeyResponse(&keys[i])
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handleCreateAPIKey(w http.ResponseWriter, r *http.Request) {
	id, ok := userIDFromPath(w, r)
	if !ok {
		return
	}
	var req createAPIKeyRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	keyID, err := uuid.Parse(req.ID)
	if err != nil {
		writeAPIKeyError(w, auth.ErrInvalidAPIKeyID)
		return
	}
	key, err := s.auth.CreateAPIKey(r.Context(), id, keyID, req.Name, req.PublicKeyPEM)
	if err != nil {
		writeAPIKeyError(w, err)
		return
	}
	details := map[string]any{"userId": id}
	if user, err := s.auth.GetUser(r.Context(), id); err == nil {
		details["username"] = user.Username
	}
	slog.Info("api key created", "by", auth.UserFromContext(r.Context()).Username, "user", id, "key", key.ID, "name", key.Name)
	s.record(r, audit.Entry{
		Action: audit.ActionAPIKeyCreate, TargetType: audit.TargetAPIKey, TargetID: key.ID.String(), TargetName: key.Name,
		Details: details,
	})
	writeJSON(w, http.StatusCreated, toAPIKeyResponse(key))
}

func (s *Server) handleDeleteAPIKey(w http.ResponseWriter, r *http.Request) {
	id, ok := userIDFromPath(w, r)
	if !ok {
		return
	}
	keyID, err := uuid.Parse(r.PathValue("keyId"))
	if err != nil {
		writeAPIKeyError(w, auth.ErrAPIKeyNotFound)
		return
	}
	key, err := s.auth.DeleteAPIKey(r.Context(), id, keyID)
	if err != nil {
		writeAPIKeyError(w, err)
		return
	}
	details := map[string]any{"userId": id}
	if user, err := s.auth.GetUser(r.Context(), id); err == nil {
		details["username"] = user.Username
	}
	slog.Info("api key deleted", "by", auth.UserFromContext(r.Context()).Username, "user", id, "key", key.ID)
	s.record(r, audit.Entry{
		Action: audit.ActionAPIKeyDelete, TargetType: audit.TargetAPIKey, TargetID: key.ID.String(), TargetName: key.Name,
		Details: details,
	})
	w.WriteHeader(http.StatusNoContent)
}

func writeAPIKeyError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, auth.ErrAPIKeyNotFound):
		writeError(w, http.StatusNotFound, err.Error())
	case errors.Is(err, auth.ErrAPIKeyExists):
		writeError(w, http.StatusConflict, err.Error())
	case errors.Is(err, auth.ErrInvalidAPIKeyID), errors.Is(err, auth.ErrInvalidAPIKeyName), errors.Is(err, auth.ErrInvalidAPIKeyPublic):
		writeError(w, http.StatusBadRequest, err.Error())
	default:
		writeUserError(w, err)
	}
}
