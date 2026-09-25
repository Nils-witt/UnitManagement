package units

import (
	"errors"
	"math"
	"strings"
	"testing"
	"time"

	"go-unit-mangement/internal/models"
)

func TestNormalizeSymbol(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		in      *models.UnitSymbol
		want    *models.UnitSymbol
		wantErr error
	}{
		{name: "nil", in: nil, want: nil},
		{name: "empty", in: &models.UnitSymbol{Grundzeichen: " "}, want: nil},
		{
			name: "trimmed",
			in:   &models.UnitSymbol{Grundzeichen: " fahrzeug ", Organisation: "feuerwehr"},
			want: &models.UnitSymbol{Grundzeichen: "fahrzeug", Organisation: "feuerwehr"},
		},
		{name: "invalid", in: &models.UnitSymbol{Einheit: "<svg>"}, wantErr: ErrInvalidSymbol},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := normalizeSymbol(tt.in)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("err = %v, want %v", err, tt.wantErr)
			}
			if (got == nil) != (tt.want == nil) || (got != nil && *got != *tt.want) {
				t.Errorf("got %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestNormalizeTacticalName(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		in      *models.TacticalName
		want    *models.TacticalName
		wantErr error
	}{
		{name: "nil", in: nil, want: nil},
		{name: "empty", in: &models.TacticalName{Organisation: " "}, want: nil},
		{
			name: "trimmed",
			in:   &models.TacticalName{Organisation: " Rotkreuz ", RegionalAssociation: "Musterstadt", Number: "1"},
			want: &models.TacticalName{Organisation: "Rotkreuz", RegionalAssociation: "Musterstadt", Number: "1"},
		},
		{name: "too long", in: &models.TacticalName{Function: strings.Repeat("x", MaxTacticalNamePartLength+1)}, wantErr: ErrInvalidTacticalName},
		{name: "control character", in: &models.TacticalName{Number: "1\n2"}, wantErr: ErrInvalidTacticalName},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := normalizeTacticalName(tt.in)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("err = %v, want %v", err, tt.wantErr)
			}
			if (got == nil) != (tt.want == nil) || (got != nil && *got != *tt.want) {
				t.Errorf("got %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestSamePosition(t *testing.T) {
	t.Parallel()

	f := func(v float64) *float64 { return &v }
	ts := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	at := func(lat, lon float64, height *float64, ts time.Time) *models.Unit {
		return &models.Unit{Latitude: &lat, Longitude: &lon, Height: height, PositionTimestamp: &ts}
	}

	withAccuracy := func(u *models.Unit, accuracy *float64) *models.Unit {
		u.Accuracy = accuracy
		return u
	}

	tests := []struct {
		name string
		a, b *models.Unit
		want bool
	}{
		{name: "both none", a: &models.Unit{}, b: &models.Unit{}, want: true},
		{name: "set", a: &models.Unit{}, b: at(1, 2, nil, ts), want: false},
		{name: "cleared", a: at(1, 2, nil, ts), b: &models.Unit{}, want: false},
		{name: "equal", a: at(1, 2, f(3), ts), b: at(1, 2, f(3), ts.In(time.Local)), want: true},
		{name: "moved", a: at(1, 2, nil, ts), b: at(1, 2.5, nil, ts), want: false},
		{name: "height added", a: at(1, 2, nil, ts), b: at(1, 2, f(3), ts), want: false},
		{name: "remeasured", a: at(1, 2, nil, ts), b: at(1, 2, nil, ts.Add(time.Second)), want: false},
		{name: "accuracy changed", a: withAccuracy(at(1, 2, nil, ts), f(5)), b: withAccuracy(at(1, 2, nil, ts), f(10)), want: false},
		{name: "accuracy equal", a: withAccuracy(at(1, 2, nil, ts), f(5)), b: withAccuracy(at(1, 2, nil, ts), f(5)), want: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := samePosition(tt.a, tt.b); got != tt.want {
				t.Errorf("samePosition = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestInputOfRoundTrips(t *testing.T) {
	lat, lon, height, accuracy := 1.0, 2.0, 3.0, 4.0
	ts := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	unit := models.Unit{
		Name: "Alpha", Latitude: &lat, Longitude: &lon, Height: &height, Accuracy: &accuracy, PositionTimestamp: &ts,
		Symbol: &models.UnitSymbol{Grundzeichen: "fahrzeug"},
	}
	var got models.Unit
	if err := apply(&got, inputOf(&unit)); err != nil {
		t.Fatal(err)
	}
	if got.Name != unit.Name || !samePosition(&got, &unit) || *got.Symbol != *unit.Symbol || got.TacticalName != nil {
		t.Errorf("got %+v, want %+v", got, unit)
	}

	if in := inputOf(&models.Unit{Name: "Bravo"}); in.Position != nil {
		t.Errorf("unit without position: got position %+v", in.Position)
	}
}

func TestApplyValidatesAccuracy(t *testing.T) {
	t.Parallel()

	f := func(v float64) *float64 { return &v }
	tests := []struct {
		name     string
		accuracy *float64
		wantErr  error
	}{
		{name: "unknown", accuracy: nil},
		{name: "zero", accuracy: f(0)},
		{name: "positive", accuracy: f(12.5)},
		{name: "negative", accuracy: f(-1), wantErr: ErrInvalidPosition},
		{name: "infinite", accuracy: f(math.Inf(1)), wantErr: ErrInvalidPosition},
		{name: "nan", accuracy: f(math.NaN()), wantErr: ErrInvalidPosition},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var unit models.Unit
			in := Input{Name: "Alpha", Position: &Position{Latitude: 1, Longitude: 2, Accuracy: tt.accuracy, Timestamp: time.Now()}}
			err := apply(&unit, in)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("err = %v, want %v", err, tt.wantErr)
			}
			if err == nil && !equalPtr(unit.Accuracy, tt.accuracy) {
				t.Errorf("accuracy = %v, want %v", unit.Accuracy, tt.accuracy)
			}
		})
	}
}
