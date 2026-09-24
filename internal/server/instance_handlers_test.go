package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"go-unit-mangement/internal/config"
)

func TestHandleInstance(t *testing.T) {
	get := func(name string) map[string]any {
		rec := httptest.NewRecorder()
		s := &Server{cfg: &config.Config{InstanceName: name}}
		s.handleInstance(rec, httptest.NewRequest(http.MethodGet, "/api/instance", nil))
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
		}
		var body map[string]any
		if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
			t.Fatalf("decode response: %v", err)
		}
		return body
	}

	if body := get(""); body["name"] != nil {
		t.Fatalf("unset: body = %v, want no name", body)
	}
	if body := get("Kreis Nord"); body["name"] != "Kreis Nord" {
		t.Fatalf("set: body = %v, want name", body)
	}
}
