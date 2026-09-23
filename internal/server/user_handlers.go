package server

import (
	"errors"
	"log/slog"
	"net/http"
	"strconv"

	"go-unit-mangement/internal/auth"
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

func (s *Server) handleListUsers(w http.ResponseWriter, r *http.Request) {
	users, err := s.auth.ListUsers(r.Context())
	if err != nil {
		writeUserError(w, err)
		return
	}
	resp := make([]userResponse, len(users))
	for i := range users {
		resp[i] = toUserResponse(&users[i])
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
	writeJSON(w, http.StatusCreated, toUserResponse(user))
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
	user, err := s.auth.UpdateUser(r.Context(), id, req.IsAdmin, req.Password)
	if err != nil {
		writeUserError(w, err)
		return
	}
	slog.Info("user updated", "by", current.Username, "user", user.Username, "isAdmin", user.IsAdmin, "passwordChanged", req.Password != "")
	writeJSON(w, http.StatusOK, toUserResponse(user))
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
	case errors.Is(err, auth.ErrUserNotFound):
		writeError(w, http.StatusNotFound, err.Error())
	case errors.Is(err, auth.ErrUsernameTaken):
		writeError(w, http.StatusConflict, err.Error())
	case errors.Is(err, auth.ErrInvalidUsername), errors.Is(err, auth.ErrPasswordTooShort):
		writeError(w, http.StatusBadRequest, err.Error())
	default:
		slog.Error("user management", "err", err)
		writeError(w, http.StatusInternalServerError, "internal error")
	}
}
