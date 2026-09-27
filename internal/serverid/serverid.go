// Package serverid gives this instance a persistent identifier of its own,
// generated once on first startup and stored in the keys directory next to
// the key pair of internal/serverkey. Other instances register this server's
// public key under this ID, and it signs its sync requests with it as the
// JWT "kid" (see internal/sync).
package serverid

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
)

// FileName is the file EnsureServerUUID reads and writes in the keys
// directory.
const FileName = "server.uuid"

// EnsureServerUUID returns the UUID stored in dir's server.uuid, generating
// and writing a random one first if the file doesn't exist yet. An existing
// file is never changed, so every startup returns the same UUID.
func EnsureServerUUID(dir string) (uuid.UUID, error) {
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return uuid.UUID{}, fmt.Errorf("create keys dir: %w", err)
	}

	path := filepath.Join(dir, FileName)

	existing, err := os.ReadFile(path) //nolint:gosec // dir is operator configuration, not request input
	if err == nil {
		id, err := uuid.Parse(strings.TrimSpace(string(existing)))
		if err != nil {
			return uuid.UUID{}, fmt.Errorf("parse %s: %w", path, err)
		}
		return id, nil
	} else if !os.IsNotExist(err) {
		return uuid.UUID{}, fmt.Errorf("read %s: %w", path, err)
	}

	id := uuid.New()
	if err := os.WriteFile(path, []byte(id.String()+"\n"), 0o644); err != nil { //nolint:gosec // an identifier, not a secret
		return uuid.UUID{}, fmt.Errorf("write %s: %w", path, err)
	}
	return id, nil
}
