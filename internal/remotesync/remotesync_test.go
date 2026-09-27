package remotesync

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
	"github.com/go-jose/go-jose/v4"
	"github.com/go-jose/go-jose/v4/jwt"
	"github.com/google/uuid"

	"go-unit-mangement/internal/models"
	"go-unit-mangement/internal/units"
)

// fakeUnits records what a worker mirrors, keyed by unit ID.
type fakeUnits struct {
	mu       sync.Mutex
	mirrored map[uuid.UUID]units.Input
	// foreign are IDs taken by units not mirrored from the remote.
	foreign map[uuid.UUID]bool
	// changed receives each change as it is applied: the unit's input, or
	// nil when it was deleted.
	changed chan *units.Input
}

func newFakeUnits() *fakeUnits {
	return &fakeUnits{mirrored: map[uuid.UUID]units.Input{}, foreign: map[uuid.UUID]bool{}, changed: make(chan *units.Input, 16)}
}

func (f *fakeUnits) UpsertSynced(_ context.Context, _, id uuid.UUID, in units.Input) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.foreign[id] {
		return false, units.ErrNotMirrored
	}
	f.mirrored[id] = in
	f.changed <- &in
	return true, nil
}

func (f *fakeUnits) DeleteSynced(_ context.Context, _, id uuid.UUID) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	_, ok := f.mirrored[id]
	delete(f.mirrored, id)
	f.changed <- nil
	return ok, nil
}

func (f *fakeUnits) SyncedIDs(context.Context, uuid.UUID) ([]uuid.UUID, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var ids []uuid.UUID
	for id := range f.mirrored {
		ids = append(ids, id)
	}
	return ids, nil
}

func (f *fakeUnits) get(id uuid.UUID) (units.Input, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	in, ok := f.mirrored[id]
	return in, ok
}

func TestMain(m *testing.M) {
	// The log store writes every entry to slog too.
	slog.SetDefault(slog.New(slog.DiscardHandler))
	os.Exit(m.Run())
}

func testKey(t *testing.T) *rsa.PrivateKey {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	return key
}

// requireKey rejects requests without a valid RS256 JWT from key under keyID,
// like the remote's API key authentication does.
func requireKey(t *testing.T, keyID uuid.UUID, key *rsa.PrivateKey, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		raw, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
		if !ok {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		tok, err := jwt.ParseSigned(raw, []jose.SignatureAlgorithm{jose.RS256})
		if err != nil || tok.Headers[0].KeyID != keyID.String() {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		var claims jwt.Claims
		if err := tok.Claims(&key.PublicKey, &claims); err != nil || claims.Expiry == nil ||
			claims.Validate(jwt.Expected{Time: time.Now()}) != nil {
			t.Errorf("bad token: %v", err)
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		next(w, r)
	}
}

func remoteUnit(name string, lat float64) RemoteUnit {
	ts := time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC)
	return RemoteUnit{ID: uuid.New(), Name: name, Position: &RemotePosition{Lat: lat, Lon: 10, Timestamp: &ts}}
}

func newTestWorker(t *testing.T, url string, keyID uuid.UUID, key *rsa.PrivateKey, store unitStore) *worker {
	t.Helper()
	return &worker{
		remote:  models.SyncRemote{ID: uuid.New(), Name: "remote", BaseURL: url, PollIntervalSec: 3600, Enabled: true},
		client:  NewClient(url, keyID, key),
		units:   store,
		logs:    newLogStore(),
		trigger: make(chan struct{}, 1),
	}
}

func TestSyncOnceMirrorsAndDeletesVanished(t *testing.T) {
	t.Parallel()

	key, keyID := testKey(t), uuid.New()
	a, b, ours := remoteUnit("A", 51), remoteUnit("B", 52), remoteUnit("ours", 53)
	list := []RemoteUnit{a, b, ours}
	var mu sync.Mutex

	srv := httptest.NewServer(requireKey(t, keyID, key, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/units" {
			http.NotFound(w, r)
			return
		}
		mu.Lock()
		defer mu.Unlock()
		_ = json.NewEncoder(w).Encode(list)
	}))
	defer srv.Close()

	store := newFakeUnits()
	store.foreign[ours.ID] = true
	w := newTestWorker(t, srv.URL+"/", keyID, key, store)

	if err := w.syncOnce(t.Context()); err != nil {
		t.Fatalf("first sync: %v", err)
	}
	for _, u := range []RemoteUnit{a, b} {
		in, ok := store.get(u.ID)
		if !ok || in.Name != u.Name || in.Position == nil || in.Position.Latitude != u.Position.Lat {
			t.Errorf("unit %s: got %+v, %v", u.Name, in, ok)
		}
	}
	if _, ok := store.get(ours.ID); ok {
		t.Error("a unit not mirrored from the remote was taken over")
	}

	mu.Lock()
	list = []RemoteUnit{a}
	mu.Unlock()
	if err := w.syncOnce(t.Context()); err != nil {
		t.Fatalf("second sync: %v", err)
	}
	if _, ok := store.get(b.ID); ok {
		t.Error("unit gone from the remote was not deleted")
	}
	if _, ok := store.get(a.ID); !ok {
		t.Error("unit still on the remote was deleted")
	}
}

func TestSyncOnceKeepsUnitsWhenListingFails(t *testing.T) {
	t.Parallel()

	key, keyID := testKey(t), uuid.New()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":"unauthorized"}`))
	}))
	defer srv.Close()

	store := newFakeUnits()
	kept := uuid.New()
	store.mirrored[kept] = units.Input{Name: "kept"}
	w := newTestWorker(t, srv.URL, keyID, key, store)

	err := w.syncOnce(t.Context())
	var statusErr *StatusError
	if !errors.As(err, &statusErr) || statusErr.Status != http.StatusUnauthorized || statusErr.Message != "unauthorized" {
		t.Fatalf("err = %v, want a 401 StatusError", err)
	}
	if _, ok := store.get(kept); !ok {
		t.Error("a failed listing deleted mirrored units")
	}
}

func TestListenerAppliesEvents(t *testing.T) {
	t.Parallel()

	key, keyID := testKey(t), uuid.New()
	created := remoteUnit("new", 50)
	moved := created
	moved.Position = &RemotePosition{Lat: 49, Lon: 9, Timestamp: created.Position.Timestamp}
	script := []any{
		map[string]any{"type": "created", "id": created.ID, "unit": created},
		map[string]any{"type": "updated", "id": moved.ID, "unit": moved},
		map[string]any{"type": "deleted", "id": created.ID},
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/units", requireKey(t, keyID, key, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("[]"))
	}))
	mux.HandleFunc("/api/units/events", requireKey(t, keyID, key, func(w http.ResponseWriter, r *http.Request) {
		c, err := websocket.Accept(w, r, nil)
		if err != nil {
			return
		}
		defer func() { _ = c.CloseNow() }()
		for _, msg := range script {
			if err := wsjson.Write(r.Context(), c, msg); err != nil {
				return
			}
		}
		<-r.Context().Done()
	}))
	srv := httptest.NewServer(mux)
	defer srv.Close()

	store := newFakeUnits()
	w := newTestWorker(t, srv.URL, keyID, key, store)
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	go w.listen(ctx)

	var applied []*units.Input
	for i := range 3 {
		select {
		case in := <-store.changed:
			applied = append(applied, in)
		case <-time.After(5 * time.Second):
			t.Fatalf("timed out waiting for event %d", i+1)
		}
	}
	if in := applied[0]; in == nil || in.Name != "new" || in.Position.Latitude != 50 {
		t.Errorf("created: %+v", in)
	}
	if in := applied[1]; in == nil || in.Position.Latitude != 49 {
		t.Errorf("updated: %+v", in)
	}
	if applied[2] != nil {
		t.Errorf("deleted: got %+v, want a deletion", applied[2])
	}
	if _, ok := store.get(created.ID); ok {
		t.Error("deleted event did not delete the unit")
	}
	select {
	case <-w.trigger:
	default:
		t.Error("connecting did not request a full sync")
	}
}

func TestNormalizeBaseURL(t *testing.T) {
	t.Parallel()

	tests := []struct {
		in, want string
		wantErr  bool
	}{
		{"https://units.example.com/", "https://units.example.com", false},
		{" http://10.0.0.1:8080/sub/ ", "http://10.0.0.1:8080/sub", false},
		{"ftp://units.example.com", "", true},
		{"units.example.com", "", true},
		{"https://units.example.com/?a=b", "", true},
		{"https://user:pw@units.example.com", "", true},
	}
	for _, tt := range tests {
		got, err := normalizeBaseURL(tt.in)
		if (err != nil) != tt.wantErr || got != tt.want {
			t.Errorf("normalizeBaseURL(%q) = %q, %v; want %q, err %v", tt.in, got, err, tt.want, tt.wantErr)
		}
	}
}

func TestLogStoreBounded(t *testing.T) {
	t.Parallel()

	s := newLogStore()
	id := uuid.New()
	for i := range maxLogEntries + 10 {
		s.infof(id, "line %d", i)
	}
	logs := s.logs(id)
	if len(logs) != maxLogEntries || logs[len(logs)-1].Message != "line 209" {
		t.Fatalf("got %d entries, last %q", len(logs), logs[len(logs)-1].Message)
	}
	if got := s.logs(uuid.New()); got == nil || len(got) != 0 {
		t.Errorf("unknown remote: %v, want empty non-nil", got)
	}
}
