// Package audit records who changed what, and who signed in, in the audit
// log, and reads it back for administrators.
package audit

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"gorm.io/gorm"

	"go-unit-mangement/internal/models"
)

// Action names what an entry records.
type Action string

const (
	ActionLogin       Action = "auth.login"
	ActionLoginFailed Action = "auth.login_failed"
	ActionLogout      Action = "auth.logout"
	ActionUserCreate  Action = "user.create"
	ActionUserUpdate  Action = "user.update"
	ActionUserDelete  Action = "user.delete"
	ActionTokenCreate Action = "token.create"
	ActionTokenRevoke Action = "token.revoke"
	ActionUnitCreate  Action = "unit.create"
	ActionUnitUpdate  Action = "unit.update"
	ActionUnitDelete  Action = "unit.delete"
)

// Target types of entries.
const (
	TargetUser  = "user"
	TargetToken = "token"
	TargetUnit  = "unit"
)

const (
	// DefaultLimit and MaxLimit bound how many entries List returns.
	DefaultLimit = 100
	MaxLimit     = 500
)

// Entry is an action to record.
type Entry struct {
	Action Action
	// Actor is who acted; nil for anonymous requests, which may name the
	// claimed user in ActorName instead.
	Actor      *models.User
	ActorName  string
	TargetType string
	TargetID   string
	TargetName string
	Details    map[string]any
	RemoteAddr string
}

// Filter selects entries for List. Zero fields don't filter.
type Filter struct {
	Limit int
	// BeforeID continues a listing after its last (oldest) entry.
	BeforeID   uint
	Action     Action
	ActorID    uint
	TargetType string
	TargetID   string
	Since, To  time.Time
}

type Service struct {
	db *gorm.DB
}

func NewService(db *gorm.DB) *Service {
	return &Service{db: db}
}

// Record stores e. It runs after the action has succeeded, so a failure is
// only logged instead of failing a request whose change is already made.
func (s *Service) Record(ctx context.Context, e Entry) {
	log := models.AuditLog{
		ActorName:  e.ActorName,
		Action:     string(e.Action),
		TargetType: e.TargetType,
		TargetID:   e.TargetID,
		TargetName: e.TargetName,
		Details:    e.Details,
		RemoteAddr: e.RemoteAddr,
	}
	if e.Actor != nil {
		log.ActorID = &e.Actor.ID
		log.ActorName = e.Actor.Username
	}
	// The request may already be canceled once the client has its response;
	// the entry must be stored anyway.
	ctx = context.WithoutCancel(ctx)
	if err := s.db.WithContext(ctx).Omit("Actor").Create(&log).Error; err != nil {
		slog.Error("record audit log entry", "action", e.Action, "err", err)
	}
}

// List returns the entries matching f, newest first.
func (s *Service) List(ctx context.Context, f Filter) ([]models.AuditLog, error) {
	if f.Limit <= 0 || f.Limit > MaxLimit {
		f.Limit = DefaultLimit
	}
	query := s.db.WithContext(ctx)
	if f.BeforeID != 0 {
		query = query.Where("id < ?", f.BeforeID)
	}
	if f.Action != "" {
		query = query.Where("action = ?", string(f.Action))
	}
	if f.ActorID != 0 {
		query = query.Where("actor_id = ?", f.ActorID)
	}
	if f.TargetType != "" {
		query = query.Where("target_type = ?", f.TargetType)
	}
	if f.TargetID != "" {
		query = query.Where("target_id = ?", f.TargetID)
	}
	if !f.Since.IsZero() {
		query = query.Where("created_at >= ?", f.Since)
	}
	if !f.To.IsZero() {
		query = query.Where("created_at <= ?", f.To)
	}
	// IDs grow with time, so ordering by them alone is newest first and
	// makes BeforeID a stable cursor.
	var entries []models.AuditLog
	if err := query.Order("id DESC").Limit(f.Limit).Find(&entries).Error; err != nil {
		return nil, fmt.Errorf("list audit log: %w", err)
	}
	return entries, nil
}

// Cleanup periodically deletes entries older than retention until ctx is
// done.
func (s *Service) Cleanup(ctx context.Context, retention, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			res := s.db.WithContext(ctx).Where("created_at < ?", time.Now().Add(-retention)).Delete(&models.AuditLog{})
			if res.Error != nil {
				slog.Error("cleanup audit log", "err", res.Error)
			} else if res.RowsAffected > 0 {
				slog.Info("cleaned up audit log", "deleted", res.RowsAffected)
			}
		}
	}
}
