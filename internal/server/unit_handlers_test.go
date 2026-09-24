package server

import (
	"encoding/json"
	"errors"
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
			if p == nil || p.Latitude != 5 || p.Longitude != 6 || p.Height != nil {
				t.Errorf("got position %+v, want 5/6 without height", p)
			}
			if in.Symbol == nil {
				t.Error("symbol cleared, want unchanged")
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
