package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"go-unit-mangement/internal/models"
)

func TestRequireAdmin(t *testing.T) {
	t.Parallel()

	ok := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) })
	tests := []struct {
		name string
		user *models.User
		want int
	}{
		{"no user", nil, http.StatusForbidden},
		{"regular user", &models.User{Username: "u"}, http.StatusForbidden},
		{"admin", &models.User{Username: "a", IsAdmin: true}, http.StatusOK},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			if tc.user != nil {
				req = req.WithContext(context.WithValue(req.Context(), ctxKey{}, tc.user))
			}
			rec := httptest.NewRecorder()
			RequireAdmin(ok).ServeHTTP(rec, req)
			if rec.Code != tc.want {
				t.Errorf("status = %d, want %d", rec.Code, tc.want)
			}
		})
	}
}
