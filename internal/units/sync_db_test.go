package units

import (
	"crypto/rand"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"go-unit-mangement/internal/models"
)

func TestUpsertSynced(t *testing.T) {
	s, user := newDBService(t)
	ctx := t.Context()

	remote := models.SyncRemote{ID: uuid.New(), Name: "remote", BaseURL: "https://remote.example", PollIntervalSec: 60, Enabled: true}
	if err := s.db.Create(&remote).Error; err != nil {
		t.Fatal(err)
	}
	events, unsubscribe := s.Subscribe()
	defer unsubscribe()

	id := uuid.New()
	ts := time.Now().UTC().Truncate(time.Second)
	in := Input{Name: rand.Text()[:8] + "-synced", Position: &Position{Latitude: 51, Longitude: 10, Timestamp: ts}}
	if changed, err := s.UpsertSynced(ctx, remote.ID, id, in); err != nil || !changed {
		t.Fatalf("create: changed = %v, err = %v", changed, err)
	}
	if ev := <-events; ev.Type != EventCreated || ev.Unit.SyncRemote == nil || ev.Unit.SyncRemote.Name != "remote" {
		t.Errorf("create event = %+v", ev)
	}

	if changed, err := s.UpsertSynced(ctx, remote.ID, id, in); err != nil || changed {
		t.Fatalf("unchanged: changed = %v, err = %v", changed, err)
	}

	in.Position = &Position{Latitude: 52, Longitude: 10, Timestamp: ts.Add(time.Minute)}
	if changed, err := s.UpsertSynced(ctx, remote.ID, id, in); err != nil || !changed {
		t.Fatalf("move: changed = %v, err = %v", changed, err)
	}
	history, err := s.History(ctx, id, 0, time.Time{}, time.Time{})
	if err != nil || len(history) != 2 || history[0].RecordedByID != nil {
		t.Fatalf("history = %+v, %v; want 2 entries without a user", history, err)
	}

	// Only the remote changes it.
	if _, _, err := s.Update(ctx, id, in, user); !errors.Is(err, ErrUnitSynced) {
		t.Errorf("local update: err = %v, want ErrUnitSynced", err)
	}
	if _, err := s.SetPosition(ctx, id, nil, user); !errors.Is(err, ErrUnitSynced) {
		t.Errorf("local position change: err = %v, want ErrUnitSynced", err)
	}
	if _, err := s.Delete(ctx, id); !errors.Is(err, ErrUnitSynced) {
		t.Errorf("local delete: err = %v, want ErrUnitSynced", err)
	}

	// Another remote, or a local unit, is never taken over.
	local, err := s.Create(ctx, Input{Name: rand.Text()[:8] + "-local"}, user)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.UpsertSynced(ctx, remote.ID, local.ID, Input{Name: local.Name + "-x"}); !errors.Is(err, ErrNotMirrored) {
		t.Errorf("local unit: err = %v, want ErrNotMirrored", err)
	}
	if _, err := s.UpsertSynced(ctx, uuid.New(), id, in); !errors.Is(err, ErrNotMirrored) {
		t.Errorf("other remote: err = %v, want ErrNotMirrored", err)
	}

	ids, err := s.SyncedIDs(ctx, remote.ID)
	if err != nil || len(ids) != 1 || ids[0] != id {
		t.Fatalf("synced IDs = %v, %v", ids, err)
	}
	if deleted, err := s.DeleteSynced(ctx, remote.ID, local.ID); err != nil || deleted {
		t.Errorf("delete local via sync: deleted = %v, err = %v", deleted, err)
	}
	if deleted, err := s.DeleteSynced(ctx, remote.ID, id); err != nil || !deleted {
		t.Errorf("delete synced: deleted = %v, err = %v", deleted, err)
	}
	if _, err := s.Delete(ctx, local.ID); err != nil {
		t.Errorf("local delete of local unit: %v", err)
	}
}

func TestDeletingRemoteKeepsUnitsAsLocal(t *testing.T) {
	s, _ := newDBService(t)
	ctx := t.Context()

	remote := models.SyncRemote{ID: uuid.New(), Name: "gone", BaseURL: "https://remote.example", PollIntervalSec: 60}
	if err := s.db.Create(&remote).Error; err != nil {
		t.Fatal(err)
	}
	id := uuid.New()
	if _, err := s.UpsertSynced(ctx, remote.ID, id, Input{Name: rand.Text()[:8] + "-orphan"}); err != nil {
		t.Fatal(err)
	}
	if err := s.db.Delete(&remote).Error; err != nil {
		t.Fatal(err)
	}
	unit, err := s.Get(ctx, id)
	if err != nil || unit.SyncRemoteID != nil {
		t.Fatalf("unit = %+v, %v; want a local unit", unit, err)
	}
	if _, err := s.Delete(ctx, id); err != nil {
		t.Errorf("delete: %v", err)
	}
}
