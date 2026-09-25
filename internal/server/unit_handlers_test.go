package server

import (
	"encoding/json"
	"errors"
	"net/url"
	"testing"
	"time"

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
		{"null clears", `{"position":null,"symbol":null,"tacticalName":null}`, func(t *testing.T, in units.Input) {
			if in.Name != "Alpha" || in.Position != nil || in.Symbol != nil || in.TacticalName != nil {
				t.Errorf("got %+v, want name kept and rest cleared", in)
			}
		}},
		{"position replaced", `{"position":{"lat":5,"lon":6}}`, func(t *testing.T, in units.Input) {
			p := in.Position
			if p == nil || p.Latitude != 5 || p.Longitude != 6 || p.Height != nil || p.Accuracy != nil ||
				p.Speed != nil || p.Course != nil {
				t.Errorf("got position %+v, want 5/6 without height, accuracy, speed and course", p)
			}
			if in.Symbol == nil {
				t.Error("symbol cleared, want unchanged")
			}
		}},
		{"position with accuracy", `{"position":{"lat":5,"lon":6,"accuracy":8.5}}`, func(t *testing.T, in units.Input) {
			if p := in.Position; p == nil || p.Accuracy == nil || *p.Accuracy != 8.5 {
				t.Errorf("got position %+v, want accuracy 8.5", p)
			}
		}},
		{"position with speed and course", `{"position":{"lat":5,"lon":6,"speed":12.5,"course":270}}`, func(t *testing.T, in units.Input) {
			p := in.Position
			if p == nil || p.Speed == nil || *p.Speed != 12.5 || p.Course == nil || *p.Course != 270 {
				t.Errorf("got position %+v, want speed 12.5 and course 270", p)
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
	for _, body := range []string{`{"name":null}`, `{"name":5}`, `{"position":"x"}`} {
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
