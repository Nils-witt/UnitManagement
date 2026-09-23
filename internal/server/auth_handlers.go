package server

import (
	"errors"
	"log/slog"
	"net/http"
	"time"

	"go-unit-mangement/internal/auth"
	"go-unit-mangement/internal/models"
)

// userResponse is the JSON shape of a user; it never includes the password
// hash or the raw SSO identity.
type userResponse struct {
	ID          uint      `json:"id"`
	Username    string    `json:"username"`
	IsAdmin     bool      `json:"isAdmin"`
	SSO         bool      `json:"sso"`
	HasPassword bool      `json:"hasPassword"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

func toUserResponse(u *models.User) userResponse {
	return userResponse{
		ID:          u.ID,
		Username:    u.Username,
		IsAdmin:     u.IsAdmin,
		SSO:         u.SSO(),
		HasPassword: u.HasPassword(),
		CreatedAt:   u.CreatedAt,
		UpdatedAt:   u.UpdatedAt,
	}
}

type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// handleAuthMethods tells the login page which sign-in options to offer.
func (s *Server) handleAuthMethods(w http.ResponseWriter, r *http.Request) {
	resp := map[string]any{"oidc": s.oidc != nil}
	if s.oidc != nil {
		resp["oidcName"] = s.cfg.OIDCDisplayName
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if req.Username == "" || req.Password == "" {
		writeError(w, http.StatusBadRequest, "username and password are required")
		return
	}

	token, user, expires, err := s.auth.Login(r.Context(), req.Username, req.Password)
	if errors.Is(err, auth.ErrInvalidCredentials) {
		writeError(w, http.StatusUnauthorized, err.Error())
		return
	}
	if err != nil {
		slog.Error("login", "err", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	s.setSessionCookie(w, token, expires)
	writeJSON(w, http.StatusOK, toUserResponse(user))
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	if cookie, err := r.Cookie(auth.CookieName); err == nil && cookie.Value != "" {
		if err := s.auth.Logout(r.Context(), cookie.Value); err != nil {
			slog.Error("logout", "err", err)
		}
	}
	s.setSessionCookie(w, "", time.Unix(0, 0))
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleMe(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, toUserResponse(auth.UserFromContext(r.Context())))
}

// setSessionCookie sets the session cookie; an expiry in the past clears it.
func (s *Server) setSessionCookie(w http.ResponseWriter, token string, expires time.Time) {
	cookie := &http.Cookie{
		Name:     auth.CookieName,
		Value:    token,
		Path:     "/",
		Expires:  expires,
		HttpOnly: true,
		Secure:   s.cfg.CookieSecure,
		SameSite: http.SameSiteLaxMode,
	}
	if token == "" {
		cookie.MaxAge = -1
	}
	http.SetCookie(w, cookie)
}
