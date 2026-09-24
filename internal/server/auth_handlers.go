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
	ID          uint   `json:"id"`
	Username    string `json:"username"`
	IsAdmin     bool   `json:"isAdmin"`
	SSO         bool   `json:"sso"`
	HasPassword bool   `json:"hasPassword"`
	// AdminManaged means the SSO provider's groups decide IsAdmin.
	AdminManaged bool `json:"adminManaged"`
	// Groups are the SSO provider's groups as of the last sign-in.
	Groups    []groupRef `json:"groups"`
	CreatedAt time.Time  `json:"createdAt"`
	UpdatedAt time.Time  `json:"updatedAt"`
}

func (s *Server) toUserResponse(u *models.User) userResponse {
	return userResponse{
		ID:           u.ID,
		Username:     u.Username,
		IsAdmin:      u.IsAdmin,
		SSO:          u.SSO(),
		HasPassword:  u.HasPassword(),
		AdminManaged: s.adminManaged(u),
		Groups:       groupRefs(u.Groups),
		CreatedAt:    u.CreatedAt,
		UpdatedAt:    u.UpdatedAt,
	}
}

func groupRefs(groups []models.Group) []groupRef {
	refs := make([]groupRef, len(groups))
	for i, g := range groups {
		refs[i] = groupRef{ID: g.ID, Name: g.Name}
	}
	return refs
}

// adminManaged reports whether u's administrator role is synced from the SSO
// provider's groups; a change made in the app would be undone at their next
// sign-in.
func (s *Server) adminManaged(u *models.User) bool {
	return u.SSO() && s.oidc.ManagesAdminRole()
}

// tokenResponse is the result of a successful sign-in: an access token to
// send as "Authorization: Bearer <token>" until it expires.
type tokenResponse struct {
	Token     string       `json:"token"`
	TokenType string       `json:"tokenType"`
	ExpiresAt time.Time    `json:"expiresAt"`
	User      userResponse `json:"user"`
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

	writeJSON(w, http.StatusOK, tokenResponse{
		Token:     token,
		TokenType: "Bearer",
		ExpiresAt: expires,
		User:      s.toUserResponse(user),
	})
}

// handleLogout ends the session of the request's access token, if any. It
// always succeeds: the client discards the token either way.
func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	if token := auth.TokenFromRequest(r); token != "" {
		if err := s.auth.Logout(r.Context(), token); err != nil {
			slog.Error("logout", "err", err)
		}
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleMe(w http.ResponseWriter, r *http.Request) {
	// The session's user comes without groups; loading them here keeps that
	// query off every other request.
	user, err := s.auth.GetUser(r.Context(), auth.UserFromContext(r.Context()).ID)
	if err != nil {
		slog.Error("load current user", "err", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusOK, s.toUserResponse(user))
}
