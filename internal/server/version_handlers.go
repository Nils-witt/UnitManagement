package server

import (
	"net/http"

	"go-unit-mangement/internal/version"
)

type versionResponse struct {
	Commit  string `json:"commit"`
	Version string `json:"version,omitempty"`
}

// handleVersion returns build metadata (git commit, and tag if the build was
// made from a tagged release). Public, so the login page footer can show it.
func (s *Server) handleVersion(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, versionResponse{Commit: version.Commit, Version: version.Version})
}
