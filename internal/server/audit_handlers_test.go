package server

import (
	"net/url"
	"slices"
	"testing"
	"time"

	"go-unit-mangement/internal/audit"
	"go-unit-mangement/internal/units"
)

func TestParseAuditFilter(t *testing.T) {
	f, err := parseAuditFilter(url.Values{
		"limit": {"20"}, "before": {"99"}, "action": {"unit.update"}, "actor": {"3"},
		"targetType": {"unit"}, "targetId": {"abc"}, "since": {"2026-01-01T00:00:00Z"}, "to": {"2026-02-01T00:00:00Z"},
	})
	if err != nil {
		t.Fatal(err)
	}
	want := audit.Filter{
		Limit: 20, BeforeID: 99, Action: audit.ActionUnitUpdate, ActorID: 3, TargetType: "unit", TargetID: "abc",
		Since: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), To: time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC),
	}
	if !f.Since.Equal(want.Since) || !f.To.Equal(want.To) {
		t.Errorf("got since %v to %v, want %v to %v", f.Since, f.To, want.Since, want.To)
	}
	f.Since, f.To, want.Since, want.To = time.Time{}, time.Time{}, time.Time{}, time.Time{}
	if f != want {
		t.Errorf("got %+v, want %+v", f, want)
	}

	for _, q := range []url.Values{
		{"limit": {"0"}},
		{"limit": {"501"}},
		{"before": {"x"}},
		{"actor": {"0"}},
		{"since": {"yesterday"}},
		{"since": {"2026-02-01T00:00:00Z"}, "to": {"2026-01-01T00:00:00Z"}},
	} {
		if _, err := parseAuditFilter(q); err == nil {
			t.Errorf("parseAuditFilter(%v) succeeded, want an error", q)
		}
	}
}

func TestTruncate(t *testing.T) {
	if got := truncate("äöü", 2); got != "äö" {
		t.Errorf("got %q, want %q", got, "äö")
	}
	if got := truncate("abc", 5); got != "abc" {
		t.Errorf("got %q, want %q", got, "abc")
	}
}

func TestAuditedUnitFields(t *testing.T) {
	tests := []struct {
		changed, want []units.Field
	}{
		{nil, nil},
		{[]units.Field{units.FieldPosition}, nil},
		{[]units.Field{units.FieldName, units.FieldPosition}, []units.Field{units.FieldName}},
		{[]units.Field{units.FieldSymbol, units.FieldTacticalName}, []units.Field{units.FieldSymbol, units.FieldTacticalName}},
	}
	for _, tt := range tests {
		in := slices.Clone(tt.changed)
		if got := auditedUnitFields(in); !slices.Equal(got, tt.want) {
			t.Errorf("auditedUnitFields(%v) = %v, want %v", tt.changed, got, tt.want)
		}
		if !slices.Equal(in, tt.changed) {
			t.Errorf("auditedUnitFields modified its argument to %v", in)
		}
	}
}
