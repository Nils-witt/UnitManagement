package server

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/google/uuid"
)

// groupRef is how a user or unit response names one of its groups.
type groupRef struct {
	ID   uint   `json:"id"`
	Name string `json:"name"`
}

type memberRef struct {
	ID       uint   `json:"id"`
	Username string `json:"username"`
}

// unitRef is how a group response names one of its units.
type unitRef struct {
	ID   uuid.UUID `json:"id"`
	Name string    `json:"name"`
}

type groupResponse struct {
	ID      uint        `json:"id"`
	Name    string      `json:"name"`
	Members []memberRef `json:"members"`
	// Units are the units assigned to the group, by name.
	Units     []unitRef `json:"units"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
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
		assigned := make([]unitRef, len(g.Units))
		for j, u := range g.Units {
			assigned[j] = unitRef{ID: u.ID, Name: u.Name}
		}
		resp[i] = groupResponse{ID: g.ID, Name: g.Name, Members: members, Units: assigned, CreatedAt: g.CreatedAt, UpdatedAt: g.UpdatedAt}
	}
	writeJSON(w, http.StatusOK, resp)
}
