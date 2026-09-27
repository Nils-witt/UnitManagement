// Package remotesync mirrors the units of other instances ("sync remotes")
// into this one. Each enabled remote gets a worker that lists the remote's
// units on start and every poll interval, and applies the changes the remote
// pushes on its unit event stream in between. Mirrored units keep the
// remote's IDs and can only be changed there (see units.ErrUnitSynced).
//
// This server authenticates to every remote with its own persistent key pair
// (internal/serverkey) and server UUID (internal/serverid): an administrator
// of the remote registers the public key as an API key under that UUID for a
// user allowed to read units.
package remotesync

import (
	"context"
	"crypto/rsa"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"go-unit-mangement/internal/models"
	"go-unit-mangement/internal/serverkey"
)

const (
	// reconcileInterval is how often the manager re-reads the configured
	// remotes and starts, stops and restarts workers to match, besides right
	// after every change made through it.
	reconcileInterval = 30 * time.Second

	MaxNameLength      = 64
	MinPollIntervalSec = 10
	MaxPollIntervalSec = 24 * 60 * 60
)

var (
	ErrRemoteNotFound      = errors.New("sync remote not found")
	ErrRemoteNotRunning    = errors.New("sync remote is not enabled")
	ErrInvalidName         = fmt.Errorf("name must be 1 to %d characters", MaxNameLength)
	ErrInvalidBaseURL      = errors.New("base URL must be an http or https URL without query, e.g. https://units.example.com")
	ErrInvalidPollInterval = fmt.Errorf("poll interval must be between %d and %d seconds", MinPollIntervalSec, MaxPollIntervalSec)
)

// RemoteInput holds a sync remote's editable fields.
type RemoteInput struct {
	Name            string
	BaseURL         string
	PollIntervalSec int
	Enabled         bool
}

// Manager stores the sync remotes and runs one worker per enabled one.
type Manager struct {
	db         *gorm.DB
	units      unitStore
	key        *rsa.PrivateKey
	serverUUID uuid.UUID
	logs       *logStore

	// reconcileMu serializes reconcile, so two runs can't both start a
	// worker for the same new remote.
	reconcileMu sync.Mutex
	mu          sync.Mutex
	workers     map[uuid.UUID]*runningWorker
}

type runningWorker struct {
	w      *worker
	cancel context.CancelFunc
	done   chan struct{}
}

// NewManager returns a manager mirroring into unitStore (a *units.Service),
// authenticating as serverUUID with key.
func NewManager(db *gorm.DB, unitStore unitStore, key *rsa.PrivateKey, serverUUID uuid.UUID) *Manager {
	return &Manager{
		db: db, units: unitStore, key: key, serverUUID: serverUUID,
		logs:    newLogStore(),
		workers: make(map[uuid.UUID]*runningWorker),
	}
}

// ServerUUID is the ID other instances register this server's public key
// under.
func (m *Manager) ServerUUID() uuid.UUID { return m.serverUUID }

// PublicKeyPEM is this server's public key, which other instances register
// as an API key.
func (m *Manager) PublicKeyPEM() (string, error) { return serverkey.PublicKeyPEM(m.key) }

// Run reconciles workers with the configured remotes until ctx is canceled,
// then stops every worker and waits for them.
func (m *Manager) Run(ctx context.Context) {
	m.reconcile(ctx)
	ticker := time.NewTicker(reconcileInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			m.stopAll()
			return
		case <-ticker.C:
			m.reconcile(ctx)
		}
	}
}

// reconcile starts a worker for every enabled remote without one, stops the
// workers of removed or disabled remotes, and restarts those whose
// connection settings changed.
func (m *Manager) reconcile(ctx context.Context) {
	m.reconcileMu.Lock()
	defer m.reconcileMu.Unlock()

	remotes, err := m.List(ctx)
	if err != nil {
		if ctx.Err() == nil {
			slog.Error("sync: list remotes", "err", err)
		}
		return
	}
	desired := make(map[uuid.UUID]models.SyncRemote, len(remotes))
	for _, r := range remotes {
		if r.Enabled {
			desired[r.ID] = r
		}
	}

	m.mu.Lock()
	var stopped []*runningWorker
	for id, rw := range m.workers {
		r, ok := desired[id]
		if ok && r.BaseURL == rw.w.remote.BaseURL && r.PollIntervalSec == rw.w.remote.PollIntervalSec {
			rw.w.remote.Name = r.Name // only used in log lines
			continue
		}
		rw.cancel()
		stopped = append(stopped, rw)
		delete(m.workers, id)
	}
	m.mu.Unlock()

	// A restarted remote's old worker must be gone before the new one starts,
	// or both could write its units at once.
	for _, rw := range stopped {
		<-rw.done
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	for id, r := range desired {
		if _, ok := m.workers[id]; !ok {
			m.startLocked(r)
		}
	}
}

// startLocked starts a worker for r; m.mu must be held. The worker runs until
// canceled by reconcile or stopAll, independent of the context that started
// it (which may be an HTTP request's).
func (m *Manager) startLocked(r models.SyncRemote) {
	ctx, cancel := context.WithCancel(context.Background())
	w := &worker{
		remote:  r,
		client:  NewClient(r.BaseURL, m.serverUUID, m.key),
		units:   m.units,
		logs:    m.logs,
		status:  func(ctx context.Context, err error) { m.setStatus(ctx, r.ID, err) },
		trigger: make(chan struct{}, 1),
	}
	rw := &runningWorker{w: w, cancel: cancel, done: make(chan struct{})}
	m.workers[r.ID] = rw
	go func() {
		defer close(rw.done)
		w.run(ctx)
	}()
}

func (m *Manager) stopAll() {
	m.mu.Lock()
	workers := m.workers
	m.workers = make(map[uuid.UUID]*runningWorker)
	m.mu.Unlock()
	for _, rw := range workers {
		rw.cancel()
	}
	for _, rw := range workers {
		<-rw.done
	}
}

// Trigger starts a full sync of the remote now, outside its poll interval.
func (m *Manager) Trigger(ctx context.Context, id uuid.UUID) error {
	m.mu.Lock()
	rw, ok := m.workers[id]
	m.mu.Unlock()
	if !ok {
		if _, err := m.Get(ctx, id); err != nil {
			return err
		}
		return ErrRemoteNotRunning
	}
	if rw.w.requestSync() {
		m.logs.infof(id, "sync requested")
	}
	return nil
}

// Logs returns the remote's recent sync activity, oldest first.
func (m *Manager) Logs(id uuid.UUID) []LogEntry { return m.logs.logs(id) }

func (m *Manager) setStatus(ctx context.Context, id uuid.UUID, syncErr error) {
	status, msg := "ok", ""
	if syncErr != nil {
		status, msg = "error", syncErr.Error()
	}
	err := m.db.WithContext(ctx).Model(&models.SyncRemote{}).Where("id = ?", id).Updates(map[string]any{
		"last_sync_at": time.Now(), "last_sync_status": status, "last_sync_error": msg,
	}).Error
	if err != nil && ctx.Err() == nil {
		slog.Error("sync: record status", "remote", id, "err", err)
	}
}

// List returns every sync remote, by name.
func (m *Manager) List(ctx context.Context) ([]models.SyncRemote, error) {
	var remotes []models.SyncRemote
	if err := m.db.WithContext(ctx).Order("name").Find(&remotes).Error; err != nil {
		return nil, fmt.Errorf("list sync remotes: %w", err)
	}
	return remotes, nil
}

func (m *Manager) Get(ctx context.Context, id uuid.UUID) (*models.SyncRemote, error) {
	var r models.SyncRemote
	err := m.db.WithContext(ctx).First(&r, "id = ?", id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrRemoteNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get sync remote %s: %w", id, err)
	}
	return &r, nil
}

// Create adds a remote; an enabled one starts syncing right away.
func (m *Manager) Create(ctx context.Context, in RemoteInput) (*models.SyncRemote, error) {
	r := &models.SyncRemote{ID: uuid.New()}
	if err := applyRemote(r, in); err != nil {
		return nil, err
	}
	if err := m.db.WithContext(ctx).Create(r).Error; err != nil {
		return nil, fmt.Errorf("create sync remote: %w", err)
	}
	m.reconcile(ctx)
	return r, nil
}

// Update replaces the remote's settings; its worker restarts if the URL or
// interval changed, and stops or starts with Enabled.
func (m *Manager) Update(ctx context.Context, id uuid.UUID, in RemoteInput) (*models.SyncRemote, error) {
	r, err := m.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	if err := applyRemote(r, in); err != nil {
		return nil, err
	}
	err = m.db.WithContext(ctx).Model(r).
		Select("name", "base_url", "poll_interval_sec", "enabled", "updated_at").
		Updates(r).Error
	if err != nil {
		return nil, fmt.Errorf("update sync remote %s: %w", id, err)
	}
	m.reconcile(ctx)
	return r, nil
}

// Delete removes the remote and stops its worker. The units mirrored from it
// stay, as ordinary local units.
func (m *Manager) Delete(ctx context.Context, id uuid.UUID) (*models.SyncRemote, error) {
	r, err := m.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	res := m.db.WithContext(ctx).Delete(&models.SyncRemote{}, "id = ?", id)
	if res.Error != nil {
		return nil, fmt.Errorf("delete sync remote %s: %w", id, res.Error)
	}
	if res.RowsAffected == 0 {
		return nil, ErrRemoteNotFound
	}
	m.reconcile(ctx)
	m.logs.forget(id)
	return r, nil
}

// applyRemote validates in and copies it onto r.
func applyRemote(r *models.SyncRemote, in RemoteInput) error {
	name := strings.TrimSpace(in.Name)
	if name == "" || utf8.RuneCountInString(name) > MaxNameLength {
		return ErrInvalidName
	}
	baseURL, err := normalizeBaseURL(in.BaseURL)
	if err != nil {
		return err
	}
	if in.PollIntervalSec < MinPollIntervalSec || in.PollIntervalSec > MaxPollIntervalSec {
		return ErrInvalidPollInterval
	}
	r.Name, r.BaseURL, r.PollIntervalSec, r.Enabled = name, baseURL, in.PollIntervalSec, in.Enabled
	return nil
}

// normalizeBaseURL checks that raw is an http(s) URL without query, fragment
// or credentials and returns it without a trailing slash.
func normalizeBaseURL(raw string) (string, error) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" ||
		u.RawQuery != "" || u.Fragment != "" || u.User != nil {
		return "", ErrInvalidBaseURL
	}
	u.Path = strings.TrimSuffix(u.Path, "/")
	u.RawPath = ""
	return u.String(), nil
}
