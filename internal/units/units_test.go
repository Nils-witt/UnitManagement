package units

import (
	"errors"
	"strings"
	"testing"

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
