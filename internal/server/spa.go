package server

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"io/fs"
	"mime"
	"net/http"
	"path"
	"strconv"
	"strings"
	"time"
)

// compressibleExts are text formats worth gzipping; images and fonts are
// already compressed.
var compressibleExts = map[string]bool{
	".html": true, ".js": true, ".css": true, ".svg": true,
	".json": true, ".map": true, ".txt": true, ".webmanifest": true,
}

// staticFile holds the precomputed metadata for one embedded file.
type staticFile struct {
	etag   string
	gzip   []byte // nil if the file is not worth compressing
	gzEtag string
}

// spaHandler serves static files from the built frontend and falls back to
// index.html for unknown paths so client-side routing works.
//
// The embedded files never change at runtime, so they are gzipped and hashed
// once at startup: this cuts the ~220 KB JS bundle to ~70 KB on the wire
// without per-request compression cost, and the ETags let browsers revalidate
// index.html with a 304 instead of re-downloading it.
func spaHandler(dist fs.FS) http.Handler {
	fileServer := http.FileServerFS(dist)
	files := precompute(dist)

	serve := func(w http.ResponseWriter, r *http.Request, name string, fallback func()) {
		f, ok := files[name]
		if !ok {
			fallback()
			return
		}
		if f.gzip != nil {
			w.Header().Add("Vary", "Accept-Encoding")
			if acceptsGzip(r) {
				if ctype := mime.TypeByExtension(path.Ext(name)); ctype != "" {
					w.Header().Set("Content-Type", ctype)
				}
				w.Header().Set("Content-Encoding", "gzip")
				w.Header().Set("ETag", f.gzEtag)
				http.ServeContent(w, r, name, time.Time{}, bytes.NewReader(f.gzip))
				return
			}
		}
		w.Header().Set("ETag", f.etag)
		fallback()
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		name := strings.TrimPrefix(path.Clean(r.URL.Path), "/")
		if name == "" {
			name = "index.html"
		}

		if info, err := fs.Stat(dist, name); err != nil || info.IsDir() {
			// Missing files that look like assets are real 404s, not routes.
			if path.Ext(name) != "" {
				http.NotFound(w, r)
				return
			}
			w.Header().Set("Cache-Control", "no-cache")
			serve(w, r, "index.html", func() { http.ServeFileFS(w, r, dist, "index.html") })
			return
		}

		if strings.HasPrefix(name, "assets/") {
			// Vite puts content hashes in these file names.
			w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		} else {
			w.Header().Set("Cache-Control", "no-cache")
		}
		serve(w, r, name, func() { fileServer.ServeHTTP(w, r) })
	})
}

// precompute hashes every embedded file and gzips the compressible ones.
// Errors are skipped: an affected file is simply served uncompressed.
func precompute(dist fs.FS) map[string]staticFile {
	files := make(map[string]staticFile)
	_ = fs.WalkDir(dist, ".", func(name string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		raw, err := fs.ReadFile(dist, name)
		if err != nil {
			return nil
		}
		sum := sha256.Sum256(raw)
		hash := hex.EncodeToString(sum[:8])
		f := staticFile{etag: strconv.Quote(hash)}

		if compressibleExts[path.Ext(name)] {
			var buf bytes.Buffer
			zw, _ := gzip.NewWriterLevel(&buf, gzip.BestCompression)
			_, werr := zw.Write(raw)
			if cerr := zw.Close(); werr == nil && cerr == nil && buf.Len() < len(raw) {
				f.gzip = buf.Bytes()
				f.gzEtag = strconv.Quote(hash + "-gzip")
			}
		}
		files[name] = f
		return nil
	})
	return files
}

// acceptsGzip reports whether the request's Accept-Encoding allows gzip.
func acceptsGzip(r *http.Request) bool {
	for _, part := range strings.Split(r.Header.Get("Accept-Encoding"), ",") {
		coding, params, _ := strings.Cut(strings.TrimSpace(part), ";")
		if coding != "gzip" && coding != "*" {
			continue
		}
		q := strings.TrimSpace(params)
		if strings.HasPrefix(q, "q=") {
			if v, err := strconv.ParseFloat(q[2:], 64); err == nil && v == 0 {
				return false
			}
		}
		return true
	}
	return false
}
