package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"go-unit-mangement/internal/version"
)

func TestHandleVersion(t *testing.T) {
	origVersion, origCommit := version.Version, version.Commit
	t.Cleanup(func() { version.Version, version.Commit = origVersion, origCommit })

	get := func() map[string]any {
		rec := httptest.NewRecorder()
		(&Server{}).handleVersion(rec, httptest.NewRequest(http.MethodGet, "/api/version", nil))
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
		}
		var body map[string]any
		if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
			t.Fatalf("decode response: %v", err)
		}
		return body
	}

	version.Version, version.Commit = "", "abc1234"
	if body := get(); body["commit"] != "abc1234" || body["version"] != nil {
		t.Fatalf("untagged build: body = %v, want commit only", body)
	}

	version.Version = "v1.2.3"
	if body := get(); body["commit"] != "abc1234" || body["version"] != "v1.2.3" {
		t.Fatalf("tagged build: body = %v, want commit and version", body)
	}
}
