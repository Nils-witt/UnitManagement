package server

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"slices"
	"testing"
	"time"

	"github.com/google/uuid"

	"go-unit-mangement/internal/models"
	"go-unit-mangement/internal/units"
)

func TestUnitPatchRequest(t *testing.T) {
	height := 12.5
	ts := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	current := func() units.Input {
		return units.Input{
			Name:         "Alpha",
			Position:     &units.Position{Latitude: 1, Longitude: 2, Height: &height, Timestamp: ts},
			Symbol:       &models.UnitSymbol{Grundzeichen: "fahrzeug"},
			TacticalName: &models.TacticalName{Number: "1"},
			GroupIDs:     []uint{1, 2},
		}
	}

	tests := []struct {
		name  string
		body  string
		check func(t *testing.T, in units.Input)
	}{
		{"empty body changes nothing", `{}`, func(t *testing.T, in units.Input) {
			want := current()
			if in.Name != want.Name || in.Position == nil || in.Symbol == nil || in.TacticalName == nil {
				t.Errorf("got %+v, want unchanged", in)
			}
		}},
		{"name only", `{"name":"Bravo"}`, func(t *testing.T, in units.Input) {
			if in.Name != "Bravo" || in.Position == nil || in.Symbol == nil || in.TacticalName == nil {
				t.Errorf("got %+v, want only name changed", in)
			}
		}},
		{"null clears", `{"symbol":null,"tacticalName":null,"groupIds":null}`, func(t *testing.T, in units.Input) {
			if in.Name != "Alpha" || in.Position == nil || in.Symbol != nil || in.TacticalName != nil || len(in.GroupIDs) != 0 {
				t.Errorf("got %+v, want name and position kept and rest cleared", in)
			}
		}},
		{"groups replaced", `{"groupIds":[3]}`, func(t *testing.T, in units.Input) {
			if !slices.Equal(in.GroupIDs, []uint{3}) || in.Symbol == nil {
				t.Errorf("got %+v, want only groups replaced", in)
			}
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var req unitPatchRequest
			if err := json.Unmarshal([]byte(tt.body), &req); err != nil {
				t.Fatal(err)
			}
			patch, err := req.patch()
			if err != nil {
				t.Fatal(err)
			}
			in := current()
			patch(&in)
			tt.check(t, in)
		})
	}
}

func TestUnitPatchRequestErrors(t *testing.T) {
	for _, body := range []string{`{"name":null}`, `{"name":5}`, `{"symbol":"x"}`, `{"groupIds":[-1]}`} {
		var req unitPatchRequest
		if err := json.Unmarshal([]byte(body), &req); err != nil {
			t.Fatal(err)
		}
		if _, err := req.patch(); err == nil {
			t.Errorf("%s: got no error", body)
		}
	}
	var req unitPatchRequest
	_ = json.Unmarshal([]byte(`{"name":null}`), &req)
	if _, err := req.patch(); !errors.Is(err, units.ErrInvalidName) {
		t.Errorf("null name: got %v, want ErrInvalidName", err)
	}
}

func TestRejectPosition(t *testing.T) {
	for _, tt := range []struct {
		body   string
		reject bool
	}{
		{`{"name":"Alpha"}`, false},
		{`{"name":"Alpha","position":{"lat":1,"lon":2}}`, true},
		{`{"position":null}`, true},
	} {
		var req unitRequest
		if err := json.Unmarshal([]byte(tt.body), &req); err != nil {
			t.Fatal(err)
		}
		w := httptest.NewRecorder()
		if got := rejectPosition(w, req.Position); got != tt.reject {
			t.Errorf("%s: rejected = %v, want %v", tt.body, got, tt.reject)
		}
		if tt.reject && w.Code != http.StatusBadRequest {
			t.Errorf("%s: status = %d, want 400", tt.body, w.Code)
		}
	}
}

func TestRejectStatus(t *testing.T) {
	rec := httptest.NewRecorder()
	if rejectStatus(rec, nil) {
		t.Error("rejected a body without status")
	}
	rec = httptest.NewRecorder()
	if !rejectStatus(rec, json.RawMessage(`{"code":3}`)) || rec.Code != http.StatusBadRequest {
		t.Errorf("status in body: code = %d, want 400", rec.Code)
	}
}

func TestStatusJSON(t *testing.T) {
	code := 4
	before := time.Now()
	st := (&statusJSON{Code: &code}).status()
	if st.Code != 4 || st.Timestamp.Before(before) {
		t.Errorf("without timestamp: %+v, want code 4 stamped now", st)
	}
	ts := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	if st := (&statusJSON{Code: &code, Timestamp: &ts}).status(); !st.Timestamp.Equal(ts) {
		t.Errorf("with timestamp: %+v, want %v", st, ts)
	}
}

func TestRejectGroups(t *testing.T) {
	for _, tt := range []struct {
		body    string
		isAdmin bool
		reject  bool
	}{
		{`{"name":"Alpha"}`, false, false},
		{`{"name":"Alpha","groupIds":null}`, false, false},
		{`{"name":"Alpha","groupIds":[1]}`, false, true},
		{`{"name":"Alpha","groupIds":[]}`, false, true},
		{`{"name":"Alpha","groupIds":[1]}`, true, false},
	} {
		var req unitRequest
		if err := json.Unmarshal([]byte(tt.body), &req); err != nil {
			t.Fatal(err)
		}
		w := httptest.NewRecorder()
		if got := rejectGroups(w, req.GroupIDs != nil, tt.isAdmin); got != tt.reject {
			t.Errorf("%s (admin %v): rejected = %v, want %v", tt.body, tt.isAdmin, got, tt.reject)
		}
		if tt.reject && w.Code != http.StatusForbidden {
			t.Errorf("%s: status = %d, want 403", tt.body, w.Code)
		}
	}
}

func TestPositionJSON(t *testing.T) {
	ts := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	var req positionJSON
	if err := json.Unmarshal([]byte(`{"lat":5,"lon":6,"accuracy":8.5,"speed":12.5,"course":270,"timestamp":"2026-01-02T03:04:05Z"}`), &req); err != nil {
		t.Fatal(err)
	}
	p := req.position()
	if p.Latitude != 5 || p.Longitude != 6 || p.Height != nil || p.Accuracy == nil || *p.Accuracy != 8.5 ||
		p.Speed == nil || *p.Speed != 12.5 || p.Course == nil || *p.Course != 270 || !p.Timestamp.Equal(ts) {
		t.Errorf("got %+v, want every field passed through", p)
	}

	before := time.Now()
	p = (&positionJSON{Lat: 1, Lon: 2}).position()
	if p.Timestamp.Before(before) || p.Timestamp.After(time.Now()) {
		t.Errorf("timestamp = %s, want now", p.Timestamp)
	}
}

func TestParseHistoryQuery(t *testing.T) {
	since := time.Date(2026, 9, 24, 8, 0, 0, 0, time.UTC)
	to := time.Date(2026, 9, 24, 12, 0, 0, 0, time.UTC)
	var none time.Time
	tests := []struct {
		query     string
		wantLimit int
		wantSince time.Time
		wantTo    time.Time
		wantErr   bool
	}{
		{"", units.MaxHistoryLimit, none, none, false},
		{"limit=10", 10, none, none, false},
		{"since=2026-09-24T08:00:00Z", units.MaxHistoryLimit, since, none, false},
		{"since=2026-09-24T10:00:00%2B02:00&limit=5", 5, since, none, false},
		{"to=2026-09-24T12:00:00Z", units.MaxHistoryLimit, none, to, false},
		{"since=2026-09-24T08:00:00Z&to=2026-09-24T12:00:00Z", units.MaxHistoryLimit, since, to, false},
		{"since=2026-09-24T08:00:00Z&to=2026-09-24T08:00:00Z", units.MaxHistoryLimit, since, since, false},
		{"limit=0", 0, none, none, true},
		{"limit=1001", 0, none, none, true},
		{"since=yesterday", 0, none, none, true},
		{"since=2026-09-24", 0, none, none, true},
		{"to=tomorrow", 0, none, none, true},
		{"since=2026-09-24T12:00:00Z&to=2026-09-24T08:00:00Z", 0, none, none, true},
	}
	for _, tt := range tests {
		t.Run(tt.query, func(t *testing.T) {
			q, err := url.ParseQuery(tt.query)
			if err != nil {
				t.Fatal(err)
			}
			limit, gotSince, gotTo, err := parseHistoryQuery(q)
			if (err != nil) != tt.wantErr {
				t.Fatalf("err = %v, want error %v", err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}
			if limit != tt.wantLimit || !gotSince.Equal(tt.wantSince) || !gotTo.Equal(tt.wantTo) {
				t.Errorf("got limit %d, since %v, to %v; want %d, %v, %v",
					limit, gotSince, gotTo, tt.wantLimit, tt.wantSince, tt.wantTo)
			}
		})
	}
}

func TestAcknowledgeStatusRequestRejectsOtherCodes(t *testing.T) {
	s := &Server{}
	for _, code := range []string{"3", "x", "10"} {
		r := httptest.NewRequest(http.MethodPost, "/api/units/"+uuid.NewString()+"/status-requests/"+code+"/acknowledge", nil)
		r.SetPathValue("id", uuid.NewString())
		r.SetPathValue("code", code)
		w := httptest.NewRecorder()
		s.handleAcknowledgeStatusRequest(w, r)
		if w.Code != http.StatusNotFound {
			t.Errorf("code %s: status = %d, want 404", code, w.Code)
		}
	}
}

func TestUnitResponseRequests(t *testing.T) {
	ts := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	resp := toUnitResponse(&models.Unit{Name: "Alpha", StatusRequests: []models.UnitStatusRequest{{Code: 5, Timestamp: ts}}})
	if len(resp.Requests) != 1 || resp.Requests[0].Code != 5 || resp.Requests[0].AcknowledgedAt != nil {
		t.Errorf("requests = %+v", resp.Requests)
	}
	if resp := toUnitResponse(&models.Unit{Name: "Bravo"}); resp.Requests == nil {
		t.Error("requests = null, want []")
	}
}
