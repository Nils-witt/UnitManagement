package serverid

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/google/uuid"
)

func TestEnsureServerUUIDGeneratesWhenMissing(t *testing.T) {
	t.Parallel()

	dir := filepath.Join(t.TempDir(), "keys")

	id, err := EnsureServerUUID(dir)
	if err != nil {
		t.Fatalf("EnsureServerUUID: %v", err)
	}

	if id == uuid.Nil {
		t.Fatal("EnsureServerUUID returned the nil UUID")
	}

	raw, err := os.ReadFile(filepath.Join(dir, FileName))
	if err != nil {
		t.Fatalf("read %s: %v", FileName, err)
	}

	stored, err := uuid.Parse(string(raw[:len(raw)-1])) // trailing newline
	if err != nil {
		t.Fatalf("parse stored uuid: %v", err)
	}

	if stored != id {
		t.Fatalf("stored uuid %s does not match returned uuid %s", stored, id)
	}
}

func TestEnsureServerUUIDIsStableAcrossCalls(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	first, err := EnsureServerUUID(dir)
	if err != nil {
		t.Fatalf("EnsureServerUUID (first call): %v", err)
	}

	second, err := EnsureServerUUID(dir)
	if err != nil {
		t.Fatalf("EnsureServerUUID (second call): %v", err)
	}

	if first != second {
		t.Fatalf("EnsureServerUUID returned different UUIDs across calls: %s vs %s", first, second)
	}
}

func TestEnsureServerUUIDRejectsCorruptFile(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	if err := os.WriteFile(filepath.Join(dir, FileName), []byte("not-a-uuid\n"), 0o600); err != nil {
		t.Fatalf("write corrupt uuid file: %v", err)
	}

	if _, err := EnsureServerUUID(dir); err == nil {
		t.Fatal("EnsureServerUUID with a corrupt server.uuid file should return an error")
	}
}
