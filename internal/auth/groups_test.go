package auth

import (
	"crypto/rand"
	"os"
	"slices"
	"strings"
	"testing"
	"time"

	"go-unit-mangement/internal/database"
	"go-unit-mangement/internal/models"
)

// TestSyncOIDCGroups needs a scratch PostgreSQL database in
// TEST_DATABASE_URL, e.g. postgres://app:app@localhost:5432/app_test?sslmode=disable.
func TestSyncOIDCGroups(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}
	db, err := database.Connect(dsn)
	if err != nil {
		t.Fatal(err)
	}
	s, err := NewService(db, time.Hour, []byte(strings.Repeat("k", MinJWTSecretLength)))
	if err != nil {
		t.Fatal(err)
	}
	ctx := t.Context()

	// Unique names keep reruns against the same database independent.
	p := rand.Text()[:8] + "-"
	a, b, c := p+"a", p+"b", p+"c"
	id := OIDCIdentity{Issuer: "https://issuer.example", Subject: p + "sub", PreferredUsername: p + "jane"}

	memberOf := func(want ...string) *models.User {
		t.Helper()
		_, user, _, err := s.LoginOIDC(ctx, id)
		if err != nil {
			t.Fatal(err)
		}
		stored, err := s.GetUser(ctx, user.ID)
		if err != nil {
			t.Fatal(err)
		}
		for _, u := range []*models.User{user, stored} {
			var got []string
			for _, g := range u.Groups {
				got = append(got, g.Name)
			}
			if !slices.Equal(got, want) {
				t.Fatalf("groups = %q, want %q", got, want)
			}
		}
		return stored
	}

	id.Groups = []string{a, b}
	memberOf(a, b)
	id.Groups = []string{b, c}
	memberOf(b, c)
	id.Groups = []string{}
	memberOf()

	// Groups outlive their last member.
	var names []string
	if err := db.Model(&models.Group{}).Where("name IN ?", []string{a, b, c}).Order("name").Pluck("name", &names).Error; err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(names, []string{a, b, c}) {
		t.Errorf("groups after leaving all = %q", names)
	}

	// The admin role follows the admin group.
	id.Groups, id.IsAdmin = []string{a}, new(true)
	if u := memberOf(a); !u.IsAdmin {
		t.Error("not an administrator after joining the admin group")
	}
	id.IsAdmin = new(false)
	if u := memberOf(a); u.IsAdmin {
		t.Error("still an administrator after leaving the admin group")
	}

	// Memberships go with the user and with the group.
	user := memberOf(a)
	id.Groups = []string{a, b}
	memberOf(a, b)
	if err := db.Where("name = ?", b).Delete(&models.Group{}).Error; err != nil {
		t.Fatal(err)
	}
	if u, _ := s.GetUser(ctx, user.ID); len(u.Groups) != 1 {
		t.Errorf("groups after deleting %s = %v", b, u.Groups)
	}
	if err := s.DeleteUser(ctx, user.ID); err != nil {
		t.Fatal(err)
	}
	var left int64
	if err := db.Model(&models.UserGroup{}).Where("user_id = ?", user.ID).Count(&left).Error; err != nil {
		t.Fatal(err)
	}
	if left != 0 {
		t.Errorf("%d memberships left after deleting the user", left)
	}

	groups, err := s.ListGroups(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.ContainsFunc(groups, func(g models.Group) bool { return g.Name == a && len(g.Users) == 0 }) {
		t.Errorf("ListGroups lacks empty group %s", a)
	}
}
