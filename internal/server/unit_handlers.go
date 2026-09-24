package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
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
// position, symbol or tactical name clears it.
type unitRequest struct {
	Name     string        `json:"name"`
	Position *positionJSON `json:"position"`
	// Symbol's and TacticalName's JSON shapes are defined by their struct tags.
	Symbol       *models.UnitSymbol   `json:"symbol"`
	TacticalName *models.TacticalName `json:"tacticalName"`
}

// unitPatchRequest is the body of a patch. A missing field is left
// unchanged; a null position, symbol or tactical name clears it. Position,
// symbol and tactical name are replaced as a whole, not merged.
type unitPatchRequest struct {
	Name         json.RawMessage `json:"name"`
	Position     json.RawMessage `json:"position"`
	Symbol       json.RawMessage `json:"symbol"`
	TacticalName json.RawMessage `json:"tacticalName"`
}

// userRefResponse names the user who created or last changed something; it is
// null once that user is deleted.
type userRefResponse struct {
	ID       uint   `json:"id"`
	Username string `json:"username"`
}

type unitResponse struct {
	ID           uuid.UUID            `json:"id"`
	Name         string               `json:"name"`
	Position     *positionJSON        `json:"position"`
	Symbol       *models.UnitSymbol   `json:"symbol"`
	TacticalName *models.TacticalName `json:"tacticalName"`
	CreatedAt    time.Time            `json:"createdAt"`
	UpdatedAt    time.Time            `json:"updatedAt"`
	CreatedBy    *userRefResponse     `json:"createdBy"`
	UpdatedBy    *userRefResponse     `json:"updatedBy"`
}

func toUnitResponse(u *models.Unit) unitResponse {
	resp := unitResponse{
		ID:           u.ID,
		Name:         u.Name,
		Symbol:       u.Symbol,
		TacticalName: u.TacticalName,
		CreatedAt:    u.CreatedAt,
		UpdatedAt:    u.UpdatedAt,
		CreatedBy:    toUserRef(u.CreatedBy),
		UpdatedBy:    toUserRef(u.UpdatedBy),
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

// positionHistoryEntry is one entry of a unit's position history.
type positionHistoryEntry struct {
	positionJSON
	RecordedAt time.Time        `json:"recordedAt"`
	RecordedBy *userRefResponse `json:"recordedBy"`
}

func toPositionHistoryEntry(p *models.UnitPosition) positionHistoryEntry {
	ts := p.Timestamp
	return positionHistoryEntry{
		positionJSON: positionJSON{Lat: p.Latitude, Lon: p.Longitude, Height: p.Height, Timestamp: &ts},
		RecordedAt:   p.CreatedAt,
		RecordedBy:   toUserRef(p.RecordedBy),
	}
}

func toUserRef(u *models.User) *userRefResponse {
	if u == nil {
		return nil
	}
	return &userRefResponse{ID: u.ID, Username: u.Username}
}

func (req *unitRequest) input() units.Input {
	in := units.Input{Name: req.Name, Symbol: req.Symbol, TacticalName: req.TacticalName}
	if p := req.Position; p != nil {
		ts := time.Now()
		if p.Timestamp != nil {
			ts = *p.Timestamp
		}
		in.Position = &units.Position{Latitude: p.Lat, Longitude: p.Lon, Height: p.Height, Timestamp: ts}
	}
	return in
}

// patch returns the function applying req to a unit's current input, or an
// error when a field has the wrong type or name is null.
func (req *unitPatchRequest) patch() (func(*units.Input), error) {
	var fields unitRequest
	for _, f := range []struct {
		raw json.RawMessage
		v   any
	}{
		{req.Name, &fields.Name},
		{req.Position, &fields.Position},
		{req.Symbol, &fields.Symbol},
		{req.TacticalName, &fields.TacticalName},
	} {
		if f.raw == nil {
			continue
		}
		if err := json.Unmarshal(f.raw, f.v); err != nil {
			return nil, err
		}
	}
	if string(req.Name) == "null" {
		return nil, units.ErrInvalidName
	}
	patched := fields.input()
	return func(in *units.Input) {
		if req.Name != nil {
			in.Name = patched.Name
		}
		if req.Position != nil {
			in.Position = patched.Position
		}
		if req.Symbol != nil {
			in.Symbol = patched.Symbol
		}
		if req.TacticalName != nil {
			in.TacticalName = patched.TacticalName
		}
	}, nil
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

// parseHistoryQuery reads the optional limit, since and to parameters of the
// position history. since and to are zero when absent.
func parseHistoryQuery(q url.Values) (limit int, since, to time.Time, err error) {
	limit = units.MaxHistoryLimit
	if v := q.Get("limit"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 || n > units.MaxHistoryLimit {
			return 0, time.Time{}, time.Time{}, fmt.Errorf("limit must be between 1 and %d", units.MaxHistoryLimit)
		}
		limit = n
	}
	if v := q.Get("since"); v != "" {
		since, err = time.Parse(time.RFC3339, v)
		if err != nil {
			return 0, time.Time{}, time.Time{}, errors.New("since must be an RFC 3339 timestamp")
		}
	}
	if v := q.Get("to"); v != "" {
		to, err = time.Parse(time.RFC3339, v)
		if err != nil {
			return 0, time.Time{}, time.Time{}, errors.New("to must be an RFC 3339 timestamp")
		}
		if !since.IsZero() && to.Before(since) {
			return 0, time.Time{}, time.Time{}, errors.New("to must not be before since")
		}
	}
	return limit, since, to, nil
}

func (s *Server) handleUnitPositions(w http.ResponseWriter, r *http.Request) {
	id, ok := unitIDFromPath(w, r)
	if !ok {
		return
	}
	limit, since, to, err := parseHistoryQuery(r.URL.Query())
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	history, err := s.units.History(r.Context(), id, limit, since, to)
	if err != nil {
		writeUnitError(w, err)
		return
	}
	resp := make([]positionHistoryEntry, len(history))
	for i := range history {
		resp[i] = toPositionHistoryEntry(&history[i])
	}
	writeJSON(w, http.StatusOK, resp)
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

func (s *Server) handlePatchUnit(w http.ResponseWriter, r *http.Request) {
	id, ok := unitIDFromPath(w, r)
	if !ok {
		return
	}
	var req unitPatchRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	patch, err := req.patch()
	if errors.Is(err, units.ErrInvalidName) {
		writeUnitError(w, err)
		return
	}
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	current := auth.UserFromContext(r.Context())
	unit, err := s.units.Patch(r.Context(), id, patch, current)
	if err != nil {
		writeUnitError(w, err)
		return
	}
	slog.Info("unit patched", "by", current.Username, "unit", unit.Name, "id", unit.ID)
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
		errors.Is(err, units.ErrInvalidSymbol), errors.Is(err, units.ErrInvalidTacticalName):
		writeError(w, http.StatusBadRequest, err.Error())
	default:
		slog.Error("unit management", "err", err)
		writeError(w, http.StatusInternalServerError, "internal error")
	}
}
