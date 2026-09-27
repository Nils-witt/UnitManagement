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

	"go-unit-mangement/internal/audit"
	"go-unit-mangement/internal/auth"
	"go-unit-mangement/internal/models"
	"go-unit-mangement/internal/units"
)

type positionJSON struct {
	Lat    float64  `json:"lat"`
	Lon    float64  `json:"lon"`
	Height *float64 `json:"height"`
	// Accuracy is the horizontal accuracy radius in meters, null when
	// unknown.
	Accuracy *float64 `json:"accuracy"`
	// Speed is meters per second, null when unknown.
	Speed *float64 `json:"speed"`
	// Course is degrees clockwise from true north, null when unknown.
	Course *float64 `json:"course"`
	// Timestamp is when the position was measured; requests may omit it to
	// mean "now".
	Timestamp *time.Time `json:"timestamp"`
}

// unitRequest is the body of both create and update; a null or missing
// symbol or tactical name clears it. The position is set separately (see
// handleSetUnitPosition) and is rejected here.
type unitRequest struct {
	Name     string          `json:"name"`
	Position json.RawMessage `json:"position"`
	// Symbol's and TacticalName's JSON shapes are defined by their struct tags.
	Symbol       *models.UnitSymbol   `json:"symbol"`
	TacticalName *models.TacticalName `json:"tacticalName"`
}

// unitPatchRequest is the body of a patch. A missing field is left
// unchanged; a null symbol or tactical name clears it. Symbol and tactical
// name are replaced as a whole, not merged. Position is only there to be
// rejected, as in unitRequest.
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
	// SyncedFrom names the instance the unit is mirrored from; null for
	// local units. Synced units can only be changed there.
	SyncedFrom *syncRemoteRefResponse `json:"syncedFrom"`
}

type syncRemoteRefResponse struct {
	ID   uuid.UUID `json:"id"`
	Name string    `json:"name"`
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
	if u.SyncRemote != nil {
		resp.SyncedFrom = &syncRemoteRefResponse{ID: u.SyncRemote.ID, Name: u.SyncRemote.Name}
	}
	if u.HasPosition() {
		resp.Position = &positionJSON{
			Lat:       *u.Latitude,
			Lon:       *u.Longitude,
			Height:    u.Height,
			Accuracy:  u.Accuracy,
			Speed:     u.Speed,
			Course:    u.Course,
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
		positionJSON: positionJSON{
			Lat: p.Latitude, Lon: p.Longitude, Height: p.Height, Accuracy: p.Accuracy,
			Speed: p.Speed, Course: p.Course, Timestamp: &ts,
		},
		RecordedAt: p.CreatedAt,
		RecordedBy: toUserRef(p.RecordedBy),
	}
}

func toUserRef(u *models.User) *userRefResponse {
	if u == nil {
		return nil
	}
	return &userRefResponse{ID: u.ID, Username: u.Username}
}

func (req *unitRequest) input() units.Input {
	return units.Input{Name: req.Name, Symbol: req.Symbol, TacticalName: req.TacticalName}
}

// position converts p, stamping it with the current time unless it has a
// timestamp.
func (p *positionJSON) position() *units.Position {
	ts := time.Now()
	if p.Timestamp != nil {
		ts = *p.Timestamp
	}
	return &units.Position{
		Latitude: p.Lat, Longitude: p.Lon, Height: p.Height, Accuracy: p.Accuracy,
		Speed: p.Speed, Course: p.Course, Timestamp: ts,
	}
}

// errPositionInUnitBody is returned for unit bodies that still carry a
// position.
var errPositionInUnitBody = errors.New("position is set via PUT /api/units/{id}/position")

// rejectPosition writes a 400 response and returns true when a unit body
// contains a position.
func rejectPosition(w http.ResponseWriter, position json.RawMessage) bool {
	if position == nil {
		return false
	}
	writeError(w, http.StatusBadRequest, errPositionInUnitBody.Error())
	return true
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
	if !decodeJSON(w, r, &req) || rejectPosition(w, req.Position) {
		return
	}
	current := auth.UserFromContext(r.Context())
	unit, err := s.units.Create(r.Context(), req.input(), current)
	if err != nil {
		writeUnitError(w, err)
		return
	}
	slog.Info("unit created", "by", current.Username, "unit", unit.Name, "id", unit.ID)
	s.record(r, audit.Entry{Action: audit.ActionUnitCreate, TargetType: audit.TargetUnit, TargetID: unit.ID.String(), TargetName: unit.Name})
	writeJSON(w, http.StatusCreated, toUnitResponse(unit))
}

func (s *Server) handleUpdateUnit(w http.ResponseWriter, r *http.Request) {
	id, ok := unitIDFromPath(w, r)
	if !ok {
		return
	}
	var req unitRequest
	if !decodeJSON(w, r, &req) || rejectPosition(w, req.Position) {
		return
	}
	current := auth.UserFromContext(r.Context())
	unit, changed, err := s.units.Update(r.Context(), id, req.input(), current)
	if err != nil {
		writeUnitError(w, err)
		return
	}
	slog.Info("unit updated", "by", current.Username, "unit", unit.Name, "id", unit.ID)
	s.recordUnitUpdate(r, unit, changed)
	writeJSON(w, http.StatusOK, toUnitResponse(unit))
}

func (s *Server) handlePatchUnit(w http.ResponseWriter, r *http.Request) {
	id, ok := unitIDFromPath(w, r)
	if !ok {
		return
	}
	var req unitPatchRequest
	if !decodeJSON(w, r, &req) || rejectPosition(w, req.Position) {
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
	unit, changed, err := s.units.Patch(r.Context(), id, patch, current)
	if err != nil {
		writeUnitError(w, err)
		return
	}
	slog.Info("unit patched", "by", current.Username, "unit", unit.Name, "id", unit.ID)
	s.recordUnitUpdate(r, unit, changed)
	writeJSON(w, http.StatusOK, toUnitResponse(unit))
}

// handleSetUnitPosition sets the unit's position. Like every position change
// it is recorded in the position history, not the audit log.
func (s *Server) handleSetUnitPosition(w http.ResponseWriter, r *http.Request) {
	id, ok := unitIDFromPath(w, r)
	if !ok {
		return
	}
	var req positionJSON
	if !decodeJSON(w, r, &req) {
		return
	}
	s.setUnitPosition(w, r, id, req.position())
}

func (s *Server) handleClearUnitPosition(w http.ResponseWriter, r *http.Request) {
	id, ok := unitIDFromPath(w, r)
	if !ok {
		return
	}
	s.setUnitPosition(w, r, id, nil)
}

// setUnitPosition sets the unit's position, or clears it when p is nil, and
// responds with the unit.
func (s *Server) setUnitPosition(w http.ResponseWriter, r *http.Request, id uuid.UUID, p *units.Position) {
	current := auth.UserFromContext(r.Context())
	unit, err := s.units.SetPosition(r.Context(), id, p, current)
	if err != nil {
		writeUnitError(w, err)
		return
	}
	slog.Debug("unit position set", "by", current.Username, "id", id, "cleared", p == nil)
	writeJSON(w, http.StatusOK, toUnitResponse(unit))
}

func (s *Server) handleDeleteUnit(w http.ResponseWriter, r *http.Request) {
	id, ok := unitIDFromPath(w, r)
	if !ok {
		return
	}
	unit, err := s.units.Delete(r.Context(), id)
	if err != nil {
		writeUnitError(w, err)
		return
	}
	slog.Info("unit deleted", "by", auth.UserFromContext(r.Context()).Username, "id", id)
	s.record(r, audit.Entry{Action: audit.ActionUnitDelete, TargetType: audit.TargetUnit, TargetID: id.String(), TargetName: unit.Name})
	w.WriteHeader(http.StatusNoContent)
}

// recordUnitUpdate adds an update of unit that changed the given fields to
// the audit log, unless nothing changed. Position changes never get here: the
// position history records them.
func (s *Server) recordUnitUpdate(r *http.Request, unit *models.Unit, changed []units.Field) {
	if len(changed) == 0 {
		return
	}
	s.record(r, audit.Entry{
		Action: audit.ActionUnitUpdate, TargetType: audit.TargetUnit, TargetID: unit.ID.String(), TargetName: unit.Name,
		Details: map[string]any{"changed": changed},
	})
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
	case errors.Is(err, units.ErrNameTaken), errors.Is(err, units.ErrUnitSynced):
		writeError(w, http.StatusConflict, err.Error())
	case errors.Is(err, units.ErrInvalidName), errors.Is(err, units.ErrInvalidPosition),
		errors.Is(err, units.ErrInvalidSymbol), errors.Is(err, units.ErrInvalidTacticalName):
		writeError(w, http.StatusBadRequest, err.Error())
	default:
		slog.Error("unit management", "err", err)
		writeError(w, http.StatusInternalServerError, "internal error")
	}
}
