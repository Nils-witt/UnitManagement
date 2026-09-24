package server

import (
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"go-unit-mangement/internal/auth"
	"go-unit-mangement/internal/models"
)

type createUserRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
	IsAdmin  bool   `json:"isAdmin"`
}

type updateUserRequest struct {
	IsAdmin bool `json:"isAdmin"`
	// Password replaces the current one when not empty.
	Password string `json:"password"`
}

type createTokenRequest struct {
	Name string `json:"name"`
	// TTLSeconds is how long the token is valid.
	TTLSeconds int64 `json:"ttlSeconds"`
}

// apiTokenResponse describes an API token without its value, which is only
// known when it is created.
type apiTokenResponse struct {
	ID        uint      `json:"id"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"createdAt"`
	ExpiresAt time.Time `json:"expiresAt"`
}

func toAPITokenResponse(t *models.Session) apiTokenResponse {
	return apiTokenResponse{ID: t.ID, Name: t.Name, CreatedAt: t.CreatedAt, ExpiresAt: t.ExpiresAt}
}

// createdTokenResponse is a new API token, including the only copy of its value.
type createdTokenResponse struct {
	apiTokenResponse
	Token     string `json:"token"`
	TokenType string `json:"tokenType"`
}

func (s *Server) handleListUsers(w http.ResponseWriter, r *http.Request) {
	users, err := s.auth.ListUsers(r.Context())
	if err != nil {
		writeUserError(w, err)
		return
	}
	resp := make([]userResponse, len(users))
	for i := range users {
		resp[i] = s.toUserResponse(&users[i])
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handleCreateUser(w http.ResponseWriter, r *http.Request) {
	var req createUserRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	user, err := s.auth.CreateUser(r.Context(), req.Username, req.Password, req.IsAdmin)
	if err != nil {
		writeUserError(w, err)
		return
	}
	slog.Info("user created", "by", auth.UserFromContext(r.Context()).Username, "user", user.Username, "isAdmin", user.IsAdmin)
	writeJSON(w, http.StatusCreated, s.toUserResponse(user))
}

func (s *Server) handleUpdateUser(w http.ResponseWriter, r *http.Request) {
	id, ok := userIDFromPath(w, r)
	if !ok {
		return
	}
	var req updateUserRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	current := auth.UserFromContext(r.Context())
	// Otherwise the last administrator could lock everyone out of user management.
	if id == current.ID && !req.IsAdmin {
		writeError(w, http.StatusBadRequest, "you cannot remove your own administrator role")
		return
	}
	if s.oidc.ManagesAdminRole() {
		target, err := s.auth.GetUser(r.Context(), id)
		if err != nil {
			writeUserError(w, err)
			return
		}
		if s.adminManaged(target) && target.IsAdmin != req.IsAdmin {
			writeError(w, http.StatusBadRequest, "the administrator role of SSO accounts is managed by the provider's groups")
			return
		}
	}
	user, err := s.auth.UpdateUser(r.Context(), id, req.IsAdmin, req.Password)
	if err != nil {
		writeUserError(w, err)
		return
	}
	slog.Info("user updated", "by", current.Username, "user", user.Username, "isAdmin", user.IsAdmin, "passwordChanged", req.Password != "")
	writeJSON(w, http.StatusOK, s.toUserResponse(user))
}

func (s *Server) handleDeleteUser(w http.ResponseWriter, r *http.Request) {
	id, ok := userIDFromPath(w, r)
	if !ok {
		return
	}
	current := auth.UserFromContext(r.Context())
	if id == current.ID {
		writeError(w, http.StatusBadRequest, "you cannot delete your own account")
		return
	}
	if err := s.auth.DeleteUser(r.Context(), id); err != nil {
		writeUserError(w, err)
		return
	}
	slog.Info("user deleted", "by", current.Username, "id", id)
	w.WriteHeader(http.StatusNoContent)
}

// handleCreateToken issues an access token in the name of another user (or
// the administrator themselves) with a lifetime of the administrator's choice.
func (s *Server) handleCreateToken(w http.ResponseWriter, r *http.Request) {
	id, ok := userIDFromPath(w, r)
	if !ok {
		return
	}
	var req createTokenRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	// Checked here as well so a huge value can't overflow the Duration.
	if req.TTLSeconds < int64(auth.MinTokenTTL/time.Second) || req.TTLSeconds > int64(auth.MaxTokenTTL/time.Second) {
		writeUserError(w, auth.ErrInvalidTokenTTL)
		return
	}
	token, session, err := s.auth.CreateToken(r.Context(), id, req.Name, time.Duration(req.TTLSeconds)*time.Second)
	if err != nil {
		writeUserError(w, err)
		return
	}
	slog.Info("api token created", "by", auth.UserFromContext(r.Context()).Username, "user", session.User.Username, "token", session.ID, "name", session.Name, "expiresAt", session.ExpiresAt)
	writeJSON(w, http.StatusCreated, createdTokenResponse{
		apiTokenResponse: toAPITokenResponse(session),
		Token:            token,
		TokenType:        "Bearer",
	})
}

func (s *Server) handleListTokens(w http.ResponseWriter, r *http.Request) {
	id, ok := userIDFromPath(w, r)
	if !ok {
		return
	}
	tokens, err := s.auth.ListTokens(r.Context(), id)
	if err != nil {
		writeUserError(w, err)
		return
	}
	resp := make([]apiTokenResponse, len(tokens))
	for i := range tokens {
		resp[i] = toAPITokenResponse(&tokens[i])
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handleRevokeToken(w http.ResponseWriter, r *http.Request) {
	id, ok := userIDFromPath(w, r)
	if !ok {
		return
	}
	tokenID, err := strconv.ParseUint(r.PathValue("tokenId"), 10, 0)
	if err != nil || tokenID == 0 {
		writeError(w, http.StatusNotFound, auth.ErrTokenNotFound.Error())
		return
	}
	if err := s.auth.RevokeToken(r.Context(), id, uint(tokenID)); err != nil {
		writeUserError(w, err)
		return
	}
	slog.Info("api token revoked", "by", auth.UserFromContext(r.Context()).Username, "user", id, "token", tokenID)
	w.WriteHeader(http.StatusNoContent)
}

func userIDFromPath(w http.ResponseWriter, r *http.Request) (uint, bool) {
	id, err := strconv.ParseUint(r.PathValue("id"), 10, 0)
	if err != nil || id == 0 {
		writeError(w, http.StatusNotFound, "user not found")
		return 0, false
	}
	return uint(id), true
}

// writeUserError maps user-management errors to responses; anything
// unexpected is logged and reported as a generic 500.
func writeUserError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, auth.ErrUserNotFound), errors.Is(err, auth.ErrTokenNotFound):
		writeError(w, http.StatusNotFound, err.Error())
	case errors.Is(err, auth.ErrUsernameTaken):
		writeError(w, http.StatusConflict, err.Error())
	case errors.Is(err, auth.ErrInvalidUsername), errors.Is(err, auth.ErrPasswordTooShort), errors.Is(err, auth.ErrInvalidTokenTTL), errors.Is(err, auth.ErrInvalidTokenName):
		writeError(w, http.StatusBadRequest, err.Error())
	default:
		slog.Error("user management", "err", err)
		writeError(w, http.StatusInternalServerError, "internal error")
	}
}
