package server

import (
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/google/uuid"

	"go-unit-mangement/internal/auth"
	"go-unit-mangement/internal/models"
	"go-unit-mangement/internal/units"
)

type positionJSON struct {
	Lat    float64  `json:"lat"`
	Lon    float64  `json:"lon"`
	Height *float64 `json:"height"`
	// Timestamp is when the position was measured; requests may omit it to
	// mean "now".
	Timestamp *time.Time `json:"timestamp"`
}

// unitRequest is the body of both create and update; a null or missing
// position or symbol clears it.
type unitRequest struct {
	Name     string        `json:"name"`
	Position *positionJSON `json:"position"`
	// Symbol's JSON shape is defined by its struct tags.
	Symbol *models.UnitSymbol `json:"symbol"`
}

// userRefResponse names the user who created or last changed something; it is
// null once that user is deleted.
type userRefResponse struct {
	ID       uint   `json:"id"`
	Username string `json:"username"`
}

type unitResponse struct {
	ID        uuid.UUID          `json:"id"`
	Name      string             `json:"name"`
	Position  *positionJSON      `json:"position"`
	Symbol    *models.UnitSymbol `json:"symbol"`
	CreatedAt time.Time          `json:"createdAt"`
	UpdatedAt time.Time          `json:"updatedAt"`
	CreatedBy *userRefResponse   `json:"createdBy"`
	UpdatedBy *userRefResponse   `json:"updatedBy"`
}

func toUnitResponse(u *models.Unit) unitResponse {
	resp := unitResponse{
		ID:        u.ID,
		Name:      u.Name,
		Symbol:    u.Symbol,
		CreatedAt: u.CreatedAt,
		UpdatedAt: u.UpdatedAt,
		CreatedBy: toUserRef(u.CreatedBy),
		UpdatedBy: toUserRef(u.UpdatedBy),
	}
	if u.HasPosition() {
		resp.Position = &positionJSON{
			Lat:       *u.Latitude,
			Lon:       *u.Longitude,
			Height:    u.Height,
			Timestamp: u.PositionTimestamp,
		}
	}
	return resp
}

func toUserRef(u *models.User) *userRefResponse {
	if u == nil {
		return nil
	}
	return &userRefResponse{ID: u.ID, Username: u.Username}
}

func (req *unitRequest) input() units.Input {
	in := units.Input{Name: req.Name, Symbol: req.Symbol}
	if p := req.Position; p != nil {
		ts := time.Now()
		if p.Timestamp != nil {
			ts = *p.Timestamp
		}
		in.Position = &units.Position{Latitude: p.Lat, Longitude: p.Lon, Height: p.Height, Timestamp: ts}
	}
	return in
}

func (s *Server) handleListUnits(w http.ResponseWriter, r *http.Request) {
	list, err := s.units.List(r.Context())
	if err != nil {
		writeUnitError(w, err)
		return
	}
	resp := make([]unitResponse, len(list))
	for i := range list {
		resp[i] = toUnitResponse(&list[i])
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handleGetUnit(w http.ResponseWriter, r *http.Request) {
	id, ok := unitIDFromPath(w, r)
	if !ok {
		return
	}
	unit, err := s.units.Get(r.Context(), id)
	if err != nil {
		writeUnitError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, toUnitResponse(unit))
}

func (s *Server) handleCreateUnit(w http.ResponseWriter, r *http.Request) {
	var req unitRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	current := auth.UserFromContext(r.Context())
	unit, err := s.units.Create(r.Context(), req.input(), current)
	if err != nil {
		writeUnitError(w, err)
		return
	}
	slog.Info("unit created", "by", current.Username, "unit", unit.Name, "id", unit.ID)
	writeJSON(w, http.StatusCreated, toUnitResponse(unit))
}

func (s *Server) handleUpdateUnit(w http.ResponseWriter, r *http.Request) {
	id, ok := unitIDFromPath(w, r)
	if !ok {
		return
	}
	var req unitRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	current := auth.UserFromContext(r.Context())
	unit, err := s.units.Update(r.Context(), id, req.input(), current)
	if err != nil {
		writeUnitError(w, err)
		return
	}
	slog.Info("unit updated", "by", current.Username, "unit", unit.Name, "id", unit.ID)
	writeJSON(w, http.StatusOK, toUnitResponse(unit))
}

func (s *Server) handleDeleteUnit(w http.ResponseWriter, r *http.Request) {
	id, ok := unitIDFromPath(w, r)
	if !ok {
		return
	}
	if err := s.units.Delete(r.Context(), id); err != nil {
		writeUnitError(w, err)
		return
	}
	slog.Info("unit deleted", "by", auth.UserFromContext(r.Context()).Username, "id", id)
	w.WriteHeader(http.StatusNoContent)
}

func unitIDFromPath(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusNotFound, units.ErrUnitNotFound.Error())
		return uuid.Nil, false
	}
	return id, true
}

// writeUnitError maps unit errors to responses; anything unexpected is
// logged and reported as a generic 500.
func writeUnitError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, units.ErrUnitNotFound):
		writeError(w, http.StatusNotFound, err.Error())
	case errors.Is(err, units.ErrNameTaken):
		writeError(w, http.StatusConflict, err.Error())
	case errors.Is(err, units.ErrInvalidName), errors.Is(err, units.ErrInvalidPosition),
		errors.Is(err, units.ErrInvalidSymbol):
		writeError(w, http.StatusBadRequest, err.Error())
	default:
		slog.Error("unit management", "err", err)
		writeError(w, http.StatusInternalServerError, "internal error")
	}
}
