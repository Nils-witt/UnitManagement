package server

import (
	"log/slog"
	"net/http"
	"time"
)

// groupRef is how a user response names one of their groups.
type groupRef struct {
	ID   uint   `json:"id"`
	Name string `json:"name"`
}

type memberRef struct {
	ID       uint   `json:"id"`
	Username string `json:"username"`
}

type groupResponse struct {
	ID        uint        `json:"id"`
	Name      string      `json:"name"`
	Members   []memberRef `json:"members"`
	CreatedAt time.Time   `json:"createdAt"`
	UpdatedAt time.Time   `json:"updatedAt"`
}

func (s *Server) handleListGroups(w http.ResponseWriter, r *http.Request) {
	groups, err := s.auth.ListGroups(r.Context())
	if err != nil {
		slog.Error("list groups", "err", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	resp := make([]groupResponse, len(groups))
	for i, g := range groups {
		members := make([]memberRef, len(g.Users))
		for j, u := range g.Users {
			members[j] = memberRef{ID: u.ID, Username: u.Username}
		}
		resp[i] = groupResponse{ID: g.ID, Name: g.Name, Members: members, CreatedAt: g.CreatedAt, UpdatedAt: g.UpdatedAt}
	}
	writeJSON(w, http.StatusOK, resp)
}
