package audit

import (
	"crypto/rand"
	"os"
	"testing"

	"go-unit-mangement/internal/database"
	"go-unit-mangement/internal/models"
)

// TestRecordAndList needs a scratch PostgreSQL database in
// TEST_DATABASE_URL, e.g. postgres://app:app@localhost:5432/app_test?sslmode=disable.
func TestRecordAndList(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}
	db, err := database.Connect(dsn)
	if err != nil {
		t.Fatal(err)
	}
	s := NewService(db)
	ctx := t.Context()

	// A unique target keeps reruns against the same database independent.
	target := rand.Text()
	actor := &models.User{Username: "audit-" + target, PasswordHash: "x"}
	if err := db.Create(actor).Error; err != nil {
		t.Fatal(err)
	}
	s.Record(ctx, Entry{Action: ActionUnitCreate, Actor: actor, TargetType: TargetUnit, TargetID: target, TargetName: "Alpha"})
	s.Record(ctx, Entry{Action: ActionUnitUpdate, Actor: actor, TargetType: TargetUnit, TargetID: target, TargetName: "Alpha", Details: map[string]any{"changed": []string{"name"}}})
	s.Record(ctx, Entry{Action: ActionUnitDelete, Actor: actor, TargetType: TargetUnit, TargetID: target, TargetName: "Alpha", RemoteAddr: "192.0.2.1"})

	all, err := s.List(ctx, Filter{TargetType: TargetUnit, TargetID: target})
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 3 || all[0].Action != string(ActionUnitDelete) || all[2].Action != string(ActionUnitCreate) {
		t.Fatalf("got %+v, want delete, update, create", all)
	}
	if all[0].ActorID == nil || *all[0].ActorID != actor.ID || all[0].ActorName != actor.Username || all[0].RemoteAddr != "192.0.2.1" {
		t.Errorf("got actor %v %q from %q, want %d %q from 192.0.2.1", all[0].ActorID, all[0].ActorName, all[0].RemoteAddr, actor.ID, actor.Username)
	}
	if changed, _ := all[1].Details["changed"].([]any); len(changed) != 1 || changed[0] != "name" {
		t.Errorf("got details %v, want changed [name]", all[1].Details)
	}

	page, err := s.List(ctx, Filter{TargetID: target, Limit: 1, BeforeID: all[0].ID})
	if err != nil {
		t.Fatal(err)
	}
	if len(page) != 1 || page[0].ID != all[1].ID {
		t.Errorf("got %+v, want only the update", page)
	}
	byAction, err := s.List(ctx, Filter{TargetID: target, Action: ActionUnitCreate, ActorID: actor.ID})
	if err != nil {
		t.Fatal(err)
	}
	if len(byAction) != 1 || byAction[0].ID != all[2].ID {
		t.Errorf("got %+v, want only the create", byAction)
	}

	// Deleting the actor keeps their entries under their name.
	if err := db.Delete(actor).Error; err != nil {
		t.Fatal(err)
	}
	all, err = s.List(ctx, Filter{TargetID: target})
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 3 || all[0].ActorID != nil || all[0].ActorName != actor.Username {
		t.Errorf("got actor %v %q after deleting them, want nil %q", all[0].ActorID, all[0].ActorName, actor.Username)
	}
}
