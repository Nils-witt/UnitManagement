package units

import (
	"context"
	"errors"
	"fmt"
	"math"
	"regexp"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"go-unit-mangement/internal/models"
)

const (
	MaxNameLength             = 64
	MaxTacticalNamePartLength = 32
	// MaxHistoryLimit caps how many position history entries History returns.
	MaxHistoryLimit = 1000
)

var (
	ErrUnitNotFound        = errors.New("unit not found")
	ErrNameTaken           = errors.New("unit name is already taken")
	ErrInvalidName         = fmt.Errorf("unit name must be 1 to %d characters", MaxNameLength)
	ErrInvalidPosition     = errors.New("latitude must be between -90 and 90, longitude between -180 and 180, height finite, accuracy and speed finite and not negative, and course at least 0 and below 360")
	ErrInvalidSymbol       = errors.New("symbol components must be IDs of lowercase letters, digits and dashes")
	ErrInvalidTacticalName = fmt.Errorf("tactical name parts must be at most %d characters without control characters", MaxTacticalNamePartLength)
)

// symbolIDPattern matches the component IDs of @taktische-zeichen/core. The
// server doesn't know the catalog itself; the web UI only offers valid IDs.
var symbolIDPattern = regexp.MustCompile(`^[a-z0-9-]{1,64}$`)

// Position is a unit's last known location.
type Position struct {
	Latitude  float64
	Longitude float64
	// Height is nil when unknown.
	Height *float64
	// Accuracy is the horizontal accuracy radius in meters, nil when unknown.
	Accuracy *float64
	// Speed is meters per second, nil when unknown.
	Speed *float64
	// Course is degrees clockwise from true north, nil when unknown.
	Course    *float64
	Timestamp time.Time
}

// Input holds the editable fields of a unit. A nil Position, Symbol or
// TacticalName clears it, as does a Symbol or TacticalName without any parts.
type Input struct {
	Name         string
	Position     *Position
	Symbol       *models.UnitSymbol
	TacticalName *models.TacticalName
}

type Service struct {
	db     *gorm.DB
	events *Broker
}

func NewService(db *gorm.DB) *Service {
	return &Service{db: db, events: NewBroker()}
}

// Subscribe streams every successful create, update and delete; see
// Broker.Subscribe.
func (s *Service) Subscribe() (<-chan Event, func()) {
	return s.events.Subscribe()
}

func (s *Service) List(ctx context.Context) ([]models.Unit, error) {
	var units []models.Unit
	if err := s.preload(ctx).Order("name").Find(&units).Error; err != nil {
		return nil, fmt.Errorf("list units: %w", err)
	}
	return units, nil
}

func (s *Service) Get(ctx context.Context, id uuid.UUID) (*models.Unit, error) {
	var unit models.Unit
	err := s.preload(ctx).First(&unit, "id = ?", id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrUnitNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get unit %s: %w", id, err)
	}
	return &unit, nil
}

func (s *Service) Create(ctx context.Context, in Input, by *models.User) (*models.Unit, error) {
	id, err := uuid.NewV7()
	if err != nil {
		return nil, fmt.Errorf("generate unit id: %w", err)
	}
	unit := &models.Unit{ID: id, CreatedByID: &by.ID, UpdatedByID: &by.ID}
	if err := apply(unit, in); err != nil {
		return nil, err
	}
	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(unit).Error; err != nil {
			return mapWriteError(err, "create unit")
		}
		return recordPosition(tx, unit, by)
	})
	if err != nil {
		return nil, err
	}
	unit.CreatedBy, unit.UpdatedBy = by, by
	s.events.Publish(Event{Type: EventCreated, ID: unit.ID, Unit: unit})
	return unit, nil
}

func (s *Service) Update(ctx context.Context, id uuid.UUID, in Input, by *models.User) (*models.Unit, error) {
	return s.Patch(ctx, id, func(cur *Input) { *cur = in }, by)
}

// Patch updates the unit like Update, with the input patch makes of the
// unit's current fields. It runs under the row lock, so concurrent patches of
// different fields don't overwrite each other.
func (s *Service) Patch(ctx context.Context, id uuid.UUID, patch func(*Input), by *models.User) (*models.Unit, error) {
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Lock the row so concurrent updates can't both miss a position change.
		var unit models.Unit
		err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&unit, "id = ?", id).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrUnitNotFound
		}
		if err != nil {
			return fmt.Errorf("get unit %s: %w", id, err)
		}
		before := unit
		in := inputOf(&unit)
		patch(&in)
		if err := apply(&unit, in); err != nil {
			return err
		}
		unit.UpdatedByID = &by.ID
		// Select("*") so a cleared position is written as NULLs too.
		res := tx.Model(&unit).
			Select("*").Omit("id", "created_at", "created_by_id", clause.Associations).
			Updates(&unit)
		if res.Error != nil {
			return mapWriteError(res.Error, fmt.Sprintf("update unit %s", id))
		}
		if res.RowsAffected == 0 {
			return ErrUnitNotFound
		}
		if samePosition(&before, &unit) {
			return nil
		}
		return recordPosition(tx, &unit, by)
	})
	if err != nil {
		return nil, err
	}
	updated, err := s.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	s.events.Publish(Event{Type: EventUpdated, ID: id, Unit: updated})
	return updated, nil
}

func (s *Service) Delete(ctx context.Context, id uuid.UUID) error {
	res := s.db.WithContext(ctx).Delete(&models.Unit{}, "id = ?", id)
	if res.Error != nil {
		return fmt.Errorf("delete unit %s: %w", id, res.Error)
	}
	if res.RowsAffected == 0 {
		return ErrUnitNotFound
	}
	s.events.Publish(Event{Type: EventDeleted, ID: id})
	return nil
}

// History returns up to limit entries of the unit's position history, newest
// measurement first. A non-zero since leaves out positions measured before it,
// a non-zero to those measured after it.
func (s *Service) History(ctx context.Context, id uuid.UUID, limit int, since, to time.Time) ([]models.UnitPosition, error) {
	if limit <= 0 || limit > MaxHistoryLimit {
		limit = MaxHistoryLimit
	}
	// Check the unit exists so an unknown ID isn't reported as an empty history.
	var count int64
	if err := s.db.WithContext(ctx).Model(&models.Unit{}).Where("id = ?", id).Count(&count).Error; err != nil {
		return nil, fmt.Errorf("get unit %s: %w", id, err)
	}
	if count == 0 {
		return nil, ErrUnitNotFound
	}
	query := s.db.WithContext(ctx).Preload("RecordedBy").Where("unit_id = ?", id)
	if !since.IsZero() {
		query = query.Where("timestamp >= ?", since)
	}
	if !to.IsZero() {
		query = query.Where("timestamp <= ?", to)
	}
	var history []models.UnitPosition
	err := query.
		Order("timestamp DESC").Order("id DESC").
		Limit(limit).
		Find(&history).Error
	if err != nil {
		return nil, fmt.Errorf("get position history of unit %s: %w", id, err)
	}
	return history, nil
}

// recordPosition appends the unit's current position, if it has one, to its
// history.
func recordPosition(tx *gorm.DB, unit *models.Unit, by *models.User) error {
	if !unit.HasPosition() {
		return nil
	}
	entry := models.UnitPosition{
		UnitID:       unit.ID,
		Latitude:     *unit.Latitude,
		Longitude:    *unit.Longitude,
		Height:       unit.Height,
		Accuracy:     unit.Accuracy,
		Speed:        unit.Speed,
		Course:       unit.Course,
		Timestamp:    *unit.PositionTimestamp,
		RecordedByID: &by.ID,
	}
	if err := tx.Create(&entry).Error; err != nil {
		return fmt.Errorf("record position of unit %s: %w", unit.ID, err)
	}
	return nil
}

// samePosition reports whether a and b have the same position, including its
// timestamp, or both have none.
func samePosition(a, b *models.Unit) bool {
	if a.HasPosition() != b.HasPosition() {
		return false
	}
	if !a.HasPosition() {
		return true
	}
	return *a.Latitude == *b.Latitude && *a.Longitude == *b.Longitude &&
		equalPtr(a.Height, b.Height) && equalPtr(a.Accuracy, b.Accuracy) &&
		equalPtr(a.Speed, b.Speed) && equalPtr(a.Course, b.Course) &&
		a.PositionTimestamp.Equal(*b.PositionTimestamp)
}

func equalPtr[T comparable](a, b *T) bool {
	return a == b || (a != nil && b != nil && *a == *b)
}

func (s *Service) preload(ctx context.Context) *gorm.DB {
	return s.db.WithContext(ctx).Preload("CreatedBy").Preload("UpdatedBy")
}

// inputOf returns the unit's editable fields.
func inputOf(unit *models.Unit) Input {
	in := Input{Name: unit.Name, Symbol: unit.Symbol, TacticalName: unit.TacticalName}
	if unit.HasPosition() {
		in.Position = &Position{
			Latitude:  *unit.Latitude,
			Longitude: *unit.Longitude,
			Height:    unit.Height,
			Accuracy:  unit.Accuracy,
			Speed:     unit.Speed,
			Course:    unit.Course,
			Timestamp: *unit.PositionTimestamp,
		}
	}
	return in
}

// apply validates in and copies it onto unit.
func apply(unit *models.Unit, in Input) error {
	name := strings.TrimSpace(in.Name)
	if name == "" || utf8.RuneCountInString(name) > MaxNameLength {
		return ErrInvalidName
	}
	unit.Name = name

	symbol, err := normalizeSymbol(in.Symbol)
	if err != nil {
		return err
	}
	unit.Symbol = symbol

	tacticalName, err := normalizeTacticalName(in.TacticalName)
	if err != nil {
		return err
	}
	unit.TacticalName = tacticalName

	if in.Position == nil {
		unit.Latitude, unit.Longitude, unit.Height, unit.Accuracy, unit.Speed, unit.Course, unit.PositionTimestamp = nil, nil, nil, nil, nil, nil, nil
		return nil
	}
	p := *in.Position
	if !validCoordinate(p.Latitude, 90) || !validCoordinate(p.Longitude, 180) ||
		(p.Height != nil && !validCoordinate(*p.Height, math.MaxFloat64)) ||
		!validNonNegative(p.Accuracy) || !validNonNegative(p.Speed) ||
		(p.Course != nil && !(*p.Course >= 0 && *p.Course < 360)) {
		return ErrInvalidPosition
	}
	ts := p.Timestamp.UTC()
	unit.Latitude, unit.Longitude, unit.Height, unit.Accuracy, unit.Speed, unit.Course, unit.PositionTimestamp =
		&p.Latitude, &p.Longitude, p.Height, p.Accuracy, p.Speed, p.Course, &ts
	return nil
}

// normalizeSymbol validates s and returns a trimmed copy, or nil when it has
// no components.
func normalizeSymbol(s *models.UnitSymbol) (*models.UnitSymbol, error) {
	if s == nil {
		return nil, nil
	}
	out := *s
	empty := true
	for _, f := range out.Fields() {
		*f = strings.TrimSpace(*f)
		if *f == "" {
			continue
		}
		if !symbolIDPattern.MatchString(*f) {
			return nil, ErrInvalidSymbol
		}
		empty = false
	}
	if empty {
		return nil, nil
	}
	return &out, nil
}

// normalizeTacticalName validates n and returns a trimmed copy, or nil when it
// has no parts.
func normalizeTacticalName(n *models.TacticalName) (*models.TacticalName, error) {
	if n == nil {
		return nil, nil
	}
	out := *n
	empty := true
	for _, f := range out.Fields() {
		*f = strings.TrimSpace(*f)
		if *f == "" {
			continue
		}
		if utf8.RuneCountInString(*f) > MaxTacticalNamePartLength || strings.ContainsFunc(*f, unicode.IsControl) {
			return nil, ErrInvalidTacticalName
		}
		empty = false
	}
	if empty {
		return nil, nil
	}
	return &out, nil
}

func validCoordinate(v, limit float64) bool {
	return !math.IsNaN(v) && v >= -limit && v <= limit
}

// validNonNegative reports whether v is unknown (nil) or finite and not
// negative.
func validNonNegative(v *float64) bool {
	return v == nil || (*v >= 0 && *v <= math.MaxFloat64)
}

func mapWriteError(err error, op string) error {
	if errors.Is(err, gorm.ErrDuplicatedKey) {
		return ErrNameTaken
	}
	return fmt.Errorf("%s: %w", op, err)
}
