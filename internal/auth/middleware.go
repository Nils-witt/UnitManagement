package auth

import (
	"context"
	"net/http"

	"go-unit-mangement/internal/models"
)

const CookieName = "session"

type ctxKey struct{}

// UserFromContext returns the authenticated user set by RequireAuth.
func UserFromContext(ctx context.Context) *models.User {
	user, _ := ctx.Value(ctxKey{}).(*models.User)
	return user
}

// RequireAuth rejects requests without a valid session cookie and stores the
// authenticated user in the request context.
func (s *Service) RequireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie(CookieName)
		if err != nil || cookie.Value == "" {
			writeAuthError(w, http.StatusUnauthorized, "unauthorized")
			return
		}
		user, err := s.UserForToken(r.Context(), cookie.Value)
		if err != nil {
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
