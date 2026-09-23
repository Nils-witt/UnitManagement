package units

import (
	"errors"
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
