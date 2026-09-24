package auth

import (
	"context"
	"net/http"
	"strings"

	"go-unit-mangement/internal/models"
)

// WebSocketProtocol is the WebSocket subprotocol that marks the next one as
// an access token. Browsers can't set headers on a WebSocket handshake, so
// clients offer the subprotocols "bearer, <token>" instead of an
// Authorization header, and the server selects "bearer".
const WebSocketProtocol = "bearer"

type ctxKey struct{}

// UserFromContext returns the authenticated user set by RequireAuth.
func UserFromContext(ctx context.Context) *models.User {
	user, _ := ctx.Value(ctxKey{}).(*models.User)
	return user
}

// TokenFromRequest returns the access token of an "Authorization: Bearer"
// header or, on a WebSocket handshake, of the subprotocols (see
// WebSocketProtocol). It returns "" if there is none.
func TokenFromRequest(r *http.Request) string {
	if scheme, token, ok := strings.Cut(r.Header.Get("Authorization"), " "); ok && strings.EqualFold(scheme, "Bearer") {
		return strings.TrimSpace(token)
	}
	if strings.EqualFold(r.Header.Get("Upgrade"), "websocket") {
		var protocols []string
		for _, v := range r.Header.Values("Sec-WebSocket-Protocol") {
			for p := range strings.SplitSeq(v, ",") {
				protocols = append(protocols, strings.TrimSpace(p))
			}
		}
		if len(protocols) == 2 && protocols[0] == WebSocketProtocol {
			return protocols[1]
		}
	}
	return ""
}

// RequireAuth rejects requests without a valid access token and stores the
// authenticated user in the request context.
func (s *Service) RequireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := TokenFromRequest(r)
		if token == "" {
			w.Header().Set("WWW-Authenticate", "Bearer")
			writeAuthError(w, http.StatusUnauthorized, "unauthorized")
			return
		}
		user, err := s.UserForToken(r.Context(), token)
		if err != nil {
			w.Header().Set("WWW-Authenticate", `Bearer error="invalid_token"`)
			writeAuthError(w, http.StatusUnauthorized, "unauthorized")
			return
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), ctxKey{}, user)))
	})
}

// RequireAdmin rejects authenticated users without the administrator role.
// It must be wrapped by RequireAuth.
func RequireAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if user := UserFromContext(r.Context()); user == nil || !user.IsAdmin {
			writeAuthError(w, http.StatusForbidden, "forbidden")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func writeAuthError(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write([]byte(`{"error":"` + msg + `"}`))
}
