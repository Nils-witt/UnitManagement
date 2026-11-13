package units

import (
	"crypto/rand"
	"errors"
	"os"
	"slices"
	"testing"
	"time"

	"go-unit-mangement/internal/database"
	"go-unit-mangement/internal/models"
)

// newDBService needs a scratch PostgreSQL database in TEST_DATABASE_URL,
// e.g. postgres://app:app@localhost:5432/app_test?sslmode=disable.
func newDBService(t *testing.T) (*Service, *models.User) {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}
	db, err := database.Connect(dsn)
	if err != nil {
		t.Fatal(err)
	}
	user := &models.User{Username: rand.Text()[:8] + "-units"}
	if err := db.Create(user).Error; err != nil {
		t.Fatal(err)
	}
	return NewService(db), user
}

func TestPatchWithoutChangesWritesNothing(t *testing.T) {
	s, user := newDBService(t)
	ctx := t.Context()

	ts := time.Now().UTC().Truncate(time.Second)
	in := Input{Name: rand.Text()[:8] + "-noop", Position: &Position{Latitude: 51, Longitude: 10, Timestamp: ts}}
	unit, err := s.Create(ctx, in, user)
	if err != nil {
		t.Fatal(err)
	}

	events, unsubscribe := s.Subscribe()
	defer unsubscribe()

	updated, changed, err := s.Update(ctx, unit.ID, in, user)
	if err != nil {
		t.Fatal(err)
	}
	if len(changed) != 0 {
		t.Errorf("changed = %v, want none", changed)
	}
	if !updated.UpdatedAt.Equal(unit.UpdatedAt) {
		t.Errorf("updatedAt = %s, want unchanged %s", updated.UpdatedAt, unit.UpdatedAt)
	}
	select {
	case ev := <-events:
		t.Errorf("got event %+v, want none", ev)
	default:
	}
	history, err := s.History(ctx, unit.ID, 0, time.Time{}, time.Time{})
	if err != nil {
		t.Fatal(err)
	}
	if len(history) != 1 {
		t.Errorf("history has %d entries, want 1", len(history))
	}

	in.Name += "-renamed"
	if _, changed, err := s.Update(ctx, unit.ID, in, user); err != nil || len(changed) != 1 {
		t.Fatalf("rename: changed = %v, err = %v; want [name]", changed, err)
	}
	select {
	case ev := <-events:
		if ev.Type != EventUpdated {
			t.Errorf("event type = %s, want %s", ev.Type, EventUpdated)
		}
	default:
		t.Error("rename published no event")
	}
}

func TestSetPosition(t *testing.T) {
	s, user := newDBService(t)
	ctx := t.Context()

	ts := time.Now().UTC().Truncate(time.Second)
	unit, err := s.Create(ctx, Input{Name: rand.Text()[:8] + "-position"}, user)
	if err != nil {
		t.Fatal(err)
	}
	p := &Position{Latitude: 51, Longitude: 10, Timestamp: ts}
	updated, err := s.SetPosition(ctx, unit.ID, p, user)
	if err != nil || !updated.HasPosition() || *updated.Latitude != 51 {
		t.Fatalf("set: unit = %+v, err = %v; want position 51/10", updated, err)
	}

	// Update leaves the position alone, even when its input has none.
	updated, _, err = s.Update(ctx, unit.ID, Input{Name: unit.Name + "-renamed"}, user)
	if err != nil || !updated.HasPosition() {
		t.Fatalf("update: unit = %+v, err = %v; want the position kept", updated, err)
	}

	if _, err := s.SetPosition(ctx, unit.ID, &Position{Latitude: 91, Longitude: 0, Timestamp: ts}, user); !errors.Is(err, ErrInvalidPosition) {
		t.Errorf("invalid: err = %v, want ErrInvalidPosition", err)
	}

	updated, err = s.SetPosition(ctx, unit.ID, nil, user)
	if err != nil || updated.HasPosition() {
		t.Fatalf("clear: unit = %+v, err = %v; want no position", updated, err)
	}
	history, err := s.History(ctx, unit.ID, 0, time.Time{}, time.Time{})
	if err != nil || len(history) != 1 {
		t.Fatalf("history = %+v, %v; want the one position set", history, err)
	}
}

func TestDeleteHistoryBeforeKeepsNewest(t *testing.T) {
	s, user := newDBService(t)
	ctx := t.Context()

	old := time.Now().UTC().Add(-48 * time.Hour).Truncate(time.Second)
	in := Input{Name: rand.Text()[:8] + "-history", Position: &Position{Latitude: 1, Longitude: 1, Timestamp: old}}
	unit, err := s.Create(ctx, in, user)
	if err != nil {
		t.Fatal(err)
	}
	for i := 1; i <= 2; i++ {
		p := &Position{Latitude: 1, Longitude: float64(1 + i), Timestamp: old.Add(time.Duration(i) * time.Hour)}
		if _, err := s.SetPosition(ctx, unit.ID, p, user); err != nil {
			t.Fatal(err)
		}
	}

	// All three entries are older than the cutoff; only the newest survives.
	if _, err := s.deleteHistoryBefore(ctx, time.Now()); err != nil {
		t.Fatal(err)
	}
	history, err := s.History(ctx, unit.ID, 0, time.Time{}, time.Time{})
	if err != nil {
		t.Fatal(err)
	}
	if len(history) != 1 || history[0].Longitude != 3 {
		t.Fatalf("history = %+v, want only the newest entry (lon 3)", history)
	}
}

func TestGroups(t *testing.T) {
	s, user := newDBService(t)
	ctx := t.Context()

	prefix := rand.Text()[:8]
	a := models.Group{Name: prefix + "-a"}
	b := models.Group{Name: prefix + "-b"}
	for _, g := range []*models.Group{&a, &b} {
		if err := s.db.Create(g).Error; err != nil {
			t.Fatal(err)
		}
	}
	names := func(u *models.Unit) []string {
		var out []string
		for _, g := range u.Groups {
			out = append(out, g.Name)
		}
		return out
	}

	unit, err := s.Create(ctx, Input{Name: rand.Text()[:8] + "-groups", GroupIDs: []uint{b.ID, a.ID, a.ID}}, user)
	if err != nil {
		t.Fatal(err)
	}
	if got := names(unit); !slices.Equal(got, []string{a.Name, b.Name}) {
		t.Fatalf("create: groups = %v, want [%s %s]", got, a.Name, b.Name)
	}

	// Update keeps the groups when its input has none.
	updated, changed, err := s.Update(ctx, unit.ID, Input{Name: unit.Name}, user)
	if err != nil || len(changed) != 0 || len(updated.Groups) != 2 {
		t.Fatalf("update without groups: groups = %v, changed = %v, err = %v; want both kept", names(updated), changed, err)
	}

	updated, changed, err = s.Update(ctx, unit.ID, Input{Name: unit.Name, GroupIDs: []uint{b.ID}}, user)
	if err != nil || !slices.Equal(changed, []Field{FieldGroups}) || !slices.Equal(names(updated), []string{b.Name}) {
		t.Fatalf("update: groups = %v, changed = %v, err = %v; want [%s], [groups]", names(updated), changed, err, b.Name)
	}

	if _, _, err := s.Update(ctx, unit.ID, Input{Name: unit.Name, GroupIDs: []uint{0}}, user); !errors.Is(err, ErrGroupNotFound) {
		t.Errorf("unknown group: err = %v, want ErrGroupNotFound", err)
	}

	// Setting the position leaves the groups alone.
	updated, err = s.SetPosition(ctx, unit.ID, &Position{Latitude: 1, Longitude: 1, Timestamp: time.Now()}, user)
	if err != nil || !slices.Equal(names(updated), []string{b.Name}) {
		t.Fatalf("set position: groups = %v, err = %v; want [%s]", names(updated), err, b.Name)
	}

	updated, changed, err = s.Patch(ctx, unit.ID, func(in *Input) { in.GroupIDs = nil }, user)
	if err != nil || !slices.Equal(changed, []Field{FieldGroups}) || len(updated.Groups) != 0 {
		t.Fatalf("clear: groups = %v, changed = %v, err = %v; want none", names(updated), changed, err)
	}

	// Deleting a group removes it from its units.
	if _, _, err := s.Update(ctx, unit.ID, Input{Name: unit.Name, GroupIDs: []uint{a.ID}}, user); err != nil {
		t.Fatal(err)
	}
	if err := s.db.Delete(&a).Error; err != nil {
		t.Fatal(err)
	}
	if got, err := s.Get(ctx, unit.ID); err != nil || len(got.Groups) != 0 {
		t.Fatalf("after group delete: groups = %v, err = %v; want none", names(got), err)
	}
}
