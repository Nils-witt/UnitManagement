package units

import (
	"context"
	"errors"
	"fmt"
	"math"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"go-unit-mangement/internal/models"
)

const MaxNameLength = 64

var (
	ErrUnitNotFound    = errors.New("unit not found")
	ErrNameTaken       = errors.New("unit name is already taken")
	ErrInvalidName     = fmt.Errorf("unit name must be 1 to %d characters", MaxNameLength)
	ErrInvalidPosition = errors.New("latitude must be between -90 and 90, longitude between -180 and 180, and height finite")
	ErrInvalidSymbol   = errors.New("symbol components must be IDs of lowercase letters, digits and dashes")
)

// symbolIDPattern matches the component IDs of @taktische-zeichen/core. The
// server doesn't know the catalog itself; the web UI only offers valid IDs.
var symbolIDPattern = regexp.MustCompile(`^[a-z0-9-]{1,64}$`)

// Position is a unit's last known location.
type Position struct {
	Latitude  float64
	Longitude float64
	// Height is nil when unknown.
	Height    *float64
	Timestamp time.Time
}

// Input holds the editable fields of a unit. A nil Position or Symbol clears
// it, as does a Symbol without any components.
type Input struct {
	Name     string
	Position *Position
	Symbol   *models.UnitSymbol
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
	if err := s.db.WithContext(ctx).Create(unit).Error; err != nil {
		return nil, mapWriteError(err, "create unit")
	}
	unit.CreatedBy, unit.UpdatedBy = by, by
	s.events.Publish(Event{Type: EventCreated, ID: unit.ID, Unit: unit})
	return unit, nil
}

func (s *Service) Update(ctx context.Context, id uuid.UUID, in Input, by *models.User) (*models.Unit, error) {
	var unit models.Unit
	if err := s.db.WithContext(ctx).First(&unit, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrUnitNotFound
		}
		return nil, fmt.Errorf("get unit %s: %w", id, err)
	}
	if err := apply(&unit, in); err != nil {
		return nil, err
	}
	unit.UpdatedByID = &by.ID
	// Select("*") so a cleared position is written as NULLs too.
	res := s.db.WithContext(ctx).Model(&unit).
		Select("*").Omit("id", "created_at", "created_by_id", clause.Associations).
		Updates(&unit)
	if res.Error != nil {
		return nil, mapWriteError(res.Error, fmt.Sprintf("update unit %s", id))
	}
	if res.RowsAffected == 0 {
		return nil, ErrUnitNotFound
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

func (s *Service) preload(ctx context.Context) *gorm.DB {
	return s.db.WithContext(ctx).Preload("CreatedBy").Preload("UpdatedBy")
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

	if in.Position == nil {
		unit.Latitude, unit.Longitude, unit.Height, unit.PositionTimestamp = nil, nil, nil, nil
		return nil
	}
	p := *in.Position
	if !validCoordinate(p.Latitude, 90) || !validCoordinate(p.Longitude, 180) ||
		(p.Height != nil && !validCoordinate(*p.Height, math.MaxFloat64)) {
		return ErrInvalidPosition
	}
	ts := p.Timestamp.UTC()
	unit.Latitude, unit.Longitude, unit.Height, unit.PositionTimestamp = &p.Latitude, &p.Longitude, p.Height, &ts
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

func validCoordinate(v, limit float64) bool {
	return !math.IsNaN(v) && v >= -limit && v <= limit
}

func mapWriteError(err error, op string) error {
	if errors.Is(err, gorm.ErrDuplicatedKey) {
		return ErrNameTaken
	}
	return fmt.Errorf("%s: %w", op, err)
}
