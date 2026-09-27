package settings

import (
	"context"
	"os"
	"slices"
	"testing"

	"go-unit-mangement/internal/database"
)

// Needs a scratch PostgreSQL database in TEST_DATABASE_URL, e.g.
// postgres://app:app@localhost:5432/app_test?sslmode=disable.
func TestUpdatePersists(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}
	db, err := database.Connect(dsn)
	if err != nil {
		t.Fatal(err)
	}
	ctx := t.Context()
	s, err := NewService(ctx, db)
	if err != nil {
		t.Fatal(err)
	}
	original := s.Get()
	// t.Context is already canceled when cleanups run.
	t.Cleanup(func() { _, _ = s.Update(context.Background(), original) })

	want := Settings{MapStyleURL: "https://tiles.example.com/style-" + t.Name() + ".json"}
	changed, err := s.Update(ctx, want)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(changed, []string{"mapStyleUrl"}) {
		t.Errorf("changed = %v", changed)
	}
	if changed, _ := s.Update(ctx, want); len(changed) != 0 {
		t.Errorf("unchanged update: changed = %v", changed)
	}

	reloaded, err := NewService(ctx, db)
	if err != nil {
		t.Fatal(err)
	}
	if got := reloaded.Get(); got != want {
		t.Errorf("reloaded %+v, want %+v", got, want)
	}
}
