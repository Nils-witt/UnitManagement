package server

import "net/http"

type instanceResponse struct {
	Name string `json:"name,omitempty"`
}

// handleInstance returns the configured instance name, if any. Public, so the
// login page can show it too.
func (s *Server) handleInstance(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, instanceResponse{Name: s.cfg.InstanceName})
}
