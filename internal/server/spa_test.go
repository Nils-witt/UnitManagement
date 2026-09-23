package server

import (
	"compress/gzip"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"
)

func TestSPAHandler(t *testing.T) {
	js := strings.Repeat("console.log('hello');\n", 200)
	dist := fstest.MapFS{
		"index.html":      {Data: []byte("<!doctype html><div id=root></div>" + strings.Repeat(" ", 200))},
		"assets/app-1.js": {Data: []byte(js)},
		"assets/logo.png": {Data: []byte("\x89PNG not really")},
	}
	h := spaHandler(dist)

	do := func(path string, header map[string]string) *http.Response {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		for k, v := range header {
			req.Header.Set(k, v)
		}
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		return rec.Result()
	}

	t.Run("gzips compressible asset when accepted", func(t *testing.T) {
		res := do("/assets/app-1.js", map[string]string{"Accept-Encoding": "br, gzip"})
		if res.Header.Get("Content-Encoding") != "gzip" {
			t.Fatalf("Content-Encoding = %q, want gzip", res.Header.Get("Content-Encoding"))
		}
		if ct := res.Header.Get("Content-Type"); !strings.HasPrefix(ct, "text/javascript") {
			t.Errorf("Content-Type = %q, want text/javascript", ct)
		}
		if !strings.Contains(res.Header.Get("Cache-Control"), "immutable") {
			t.Errorf("Cache-Control = %q, want immutable", res.Header.Get("Cache-Control"))
		}
		zr, err := gzip.NewReader(res.Body)
		if err != nil {
			t.Fatal(err)
		}
		body, _ := io.ReadAll(zr)
		if string(body) != js {
			t.Error("decompressed body does not match original")
		}
	})

	t.Run("serves identity when gzip not accepted", func(t *testing.T) {
		for _, ae := range []string{"", "gzip;q=0", "br"} {
			res := do("/assets/app-1.js", map[string]string{"Accept-Encoding": ae})
			body, _ := io.ReadAll(res.Body)
			if res.Header.Get("Content-Encoding") != "" || string(body) != js {
				t.Errorf("Accept-Encoding %q: got encoded or wrong body", ae)
			}
			if res.Header.Get("Vary") != "Accept-Encoding" {
				t.Errorf("Accept-Encoding %q: missing Vary header", ae)
			}
		}
	})

	t.Run("does not gzip binary files", func(t *testing.T) {
		res := do("/assets/logo.png", map[string]string{"Accept-Encoding": "gzip"})
		if res.Header.Get("Content-Encoding") != "" {
			t.Error("png should not be gzipped")
		}
	})

	t.Run("index.html revalidates with 304", func(t *testing.T) {
		for _, ae := range []string{"gzip", ""} {
			first := do("/some/route", map[string]string{"Accept-Encoding": ae})
			etag := first.Header.Get("ETag")
			if etag == "" {
				t.Fatalf("Accept-Encoding %q: missing ETag", ae)
			}
			second := do("/some/route", map[string]string{"Accept-Encoding": ae, "If-None-Match": etag})
			if second.StatusCode != http.StatusNotModified {
				t.Errorf("Accept-Encoding %q: status = %d, want 304", ae, second.StatusCode)
			}
		}
	})

	t.Run("missing asset is 404", func(t *testing.T) {
		if res := do("/assets/missing.js", nil); res.StatusCode != http.StatusNotFound {
			t.Errorf("status = %d, want 404", res.StatusCode)
		}
	})
}
