package remotesync

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/google/uuid"
)

// maxLogEntries bounds how many recent log lines are kept per remote.
const maxLogEntries = 200

// LogEntry is one line of a remote's recent sync activity.
type LogEntry struct {
	Time    time.Time `json:"time"`
	Level   string    `json:"level"`
	Message string    `json:"message"`
}

// logStore keeps each remote's recent sync activity in memory for the admin
// UI, and writes every entry to the process log too. Entries are lost on
// restart.
type logStore struct {
	mu      sync.Mutex
	entries map[uuid.UUID][]LogEntry
}

func newLogStore() *logStore {
	return &logStore{entries: make(map[uuid.UUID][]LogEntry)}
}

func (s *logStore) infof(remote uuid.UUID, format string, args ...any) {
	s.record(remote, slog.LevelInfo, fmt.Sprintf(format, args...))
}

func (s *logStore) errorf(remote uuid.UUID, format string, args ...any) {
	s.record(remote, slog.LevelError, fmt.Sprintf(format, args...))
}

func (s *logStore) record(remote uuid.UUID, level slog.Level, msg string) {
	slog.Log(context.Background(), level, "sync: "+msg, "remote", remote)

	name := "info"
	if level >= slog.LevelError {
		name = "error"
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	entries := append(s.entries[remote], LogEntry{Time: time.Now(), Level: name, Message: msg})
	if len(entries) > maxLogEntries {
		entries = append([]LogEntry(nil), entries[len(entries)-maxLogEntries:]...)
	}
	s.entries[remote] = entries
}

// logs returns a copy of remote's entries, oldest first; never nil.
func (s *logStore) logs(remote uuid.UUID) []LogEntry {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]LogEntry{}, s.entries[remote]...)
}

// forget drops remote's entries once it is deleted.
func (s *logStore) forget(remote uuid.UUID) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.entries, remote)
}
