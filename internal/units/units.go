package units

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"regexp"
	"slices"
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
	// MinStatus and MaxStatus bound the FMS status codes.
	MinStatus = 0
	MaxStatus = 9
)

var (
	ErrUnitNotFound        = errors.New("unit not found")
	ErrNameTaken           = errors.New("unit name is already taken")
	ErrInvalidName         = fmt.Errorf("unit name must be 1 to %d characters", MaxNameLength)
	ErrInvalidPosition     = errors.New("latitude must be between -90 and 90, longitude between -180 and 180, height finite, accuracy and speed finite and not negative, and course at least 0 and below 360")
	ErrInvalidSymbol       = errors.New("symbol components must be IDs of lowercase letters, digits and dashes")
	ErrInvalidTacticalName = fmt.Errorf("tactical name parts must be at most %d characters without control characters", MaxTacticalNamePartLength)
	ErrGroupNotFound       = errors.New("group not found")
	ErrInvalidStatus       = fmt.Errorf("status must be an FMS status from %d to %d", MinStatus, MaxStatus)
	// ErrUnitSynced means the unit is mirrored from another instance, where
	// alone it can be changed.
	ErrUnitSynced = errors.New("unit is synced from another instance and can only be changed there")
	// ErrNotMirrored means a synced unit's ID belongs to a local unit or one
	// mirrored from another remote, which sync leaves alone.
	ErrNotMirrored = errors.New("a unit with this id exists but is not mirrored from this remote")
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

// Status is a unit's FMS status as reported at Timestamp.
type Status struct {
	Code      int
	Timestamp time.Time
}

// Input holds the editable fields of a unit. A nil Position, Status, Symbol
// or TacticalName clears it, as does a Symbol or TacticalName without any parts.
type Input struct {
	Name         string
	Position     *Position
	Status       *Status
	Symbol       *models.UnitSymbol
	TacticalName *models.TacticalName
	// GroupIDs are the groups the unit is assigned to; duplicates are
	// ignored. Update keeps the unit's groups when it is nil.
	GroupIDs []uint
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
		groups, err := findGroups(tx, in.GroupIDs)
		if err != nil {
			return err
		}
		if err := tx.Create(unit).Error; err != nil {
			return mapWriteError(err, "create unit")
		}
		if err := replaceGroups(tx, unit.ID, groups); err != nil {
			return err
		}
		unit.Groups = groups
		return recordPosition(tx, unit, by)
	})
	if err != nil {
		return nil, err
	}
	unit.CreatedBy, unit.UpdatedBy = by, by
	s.events.Publish(Event{Type: EventCreated, ID: unit.ID, Unit: unit})
	return unit, nil
}

// Update replaces the unit's editable fields with in, except the position and
// the status, which only SetPosition and SetStatus change; in.Position and
// in.Status are ignored. A nil in.GroupIDs
// keeps the unit's groups. It also returns which fields changed (see
// Changed).
func (s *Service) Update(ctx context.Context, id uuid.UUID, in Input, by *models.User) (*models.Unit, []Field, error) {
	return s.Patch(ctx, id, func(cur *Input) {
		pos, status, groupIDs := cur.Position, cur.Status, cur.GroupIDs
		*cur = in
		cur.Position, cur.Status = pos, status
		if in.GroupIDs == nil {
			cur.GroupIDs = groupIDs
		}
	}, by)
}

// SetPosition sets the unit's position, or clears it when p is nil, and
// records a new one in the position history. Setting the current position
// again changes nothing (see Patch).
func (s *Service) SetPosition(ctx context.Context, id uuid.UUID, p *Position, by *models.User) (*models.Unit, error) {
	unit, _, err := s.Patch(ctx, id, func(in *Input) { in.Position = p }, by)
	return unit, err
}

// SetStatus sets the unit's FMS status, or clears it when st is nil. Setting
// the current status again changes nothing, even with a newer timestamp, so
// repeated reports send no event.
func (s *Service) SetStatus(ctx context.Context, id uuid.UUID, st *Status, by *models.User) (*models.Unit, error) {
	unit, _, err := s.Patch(ctx, id, func(in *Input) {
		if st != nil && in.Status != nil && in.Status.Code == st.Code {
			return
		}
		in.Status = st
	}, by)
	return unit, err
}

// Patch updates the unit like Update, with the input patch makes of the
// unit's current fields. It runs under the row lock, so concurrent patches of
// different fields don't overwrite each other. A patch that changes nothing
// writes nothing: the unit keeps its updated_at and updated_by, and no event
// is published.
func (s *Service) Patch(ctx context.Context, id uuid.UUID, patch func(*Input), by *models.User) (*models.Unit, []Field, error) {
	var changed []Field
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
		if unit.SyncRemoteID != nil {
			return ErrUnitSynced
		}
		if unit.Groups, err = loadGroups(tx, id); err != nil {
			return err
		}
		before := unit
		in := inputOf(&unit)
		patch(&in)
		if err := apply(&unit, in); err != nil {
			return err
		}
		// Only look the groups up when they change: positions are patched
		// far more often than groups.
		if !slices.Equal(groupIDs(unit.Groups), normalizeGroupIDs(in.GroupIDs)) {
			if unit.Groups, err = findGroups(tx, in.GroupIDs); err != nil {
				return err
			}
		}
		changed = Changed(&before, &unit)
		// Nothing to write: leave updated_at and updated_by alone and don't
		// wake up every event stream.
		if len(changed) == 0 {
			return nil
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
		if slices.Contains(changed, FieldGroups) {
			if err := replaceGroups(tx, id, unit.Groups); err != nil {
				return err
			}
		}
		if samePosition(&before, &unit) {
			return nil
		}
		return recordPosition(tx, &unit, by)
	})
	if err != nil {
		return nil, nil, err
	}
	updated, err := s.Get(ctx, id)
	if err != nil {
		return nil, nil, err
	}
	if len(changed) > 0 {
		s.events.Publish(Event{Type: EventUpdated, ID: id, Unit: updated})
	}
	return updated, changed, nil
}

// Delete removes the unit and returns it as it was. Units synced from another
// instance can't be deleted (ErrUnitSynced).
func (s *Service) Delete(ctx context.Context, id uuid.UUID) (*models.Unit, error) {
	var unit models.Unit
	res := s.db.WithContext(ctx).Clauses(clause.Returning{}).Where("id = ? AND sync_remote_id IS NULL", id).Delete(&unit)
	if res.Error != nil {
		return nil, fmt.Errorf("delete unit %s: %w", id, res.Error)
	}
	if res.RowsAffected == 0 {
		if _, err := s.Get(ctx, id); err != nil {
			return nil, err
		}
		return nil, ErrUnitSynced
	}
	s.events.Publish(Event{Type: EventDeleted, ID: id})
	return &unit, nil
}

// Field names an editable field of a unit.
type Field string

const (
	FieldName         Field = "name"
	FieldPosition     Field = "position"
	FieldStatus       Field = "status"
	FieldSymbol       Field = "symbol"
	FieldTacticalName Field = "tacticalName"
	FieldGroups       Field = "groups"
)

// Changed returns the editable fields that differ between a and b.
func Changed(a, b *models.Unit) []Field {
	var changed []Field
	if a.Name != b.Name {
		changed = append(changed, FieldName)
	}
	if !samePosition(a, b) {
		changed = append(changed, FieldPosition)
	}
	if !sameStatus(a, b) {
		changed = append(changed, FieldStatus)
	}
	if !equalPtr(a.Symbol, b.Symbol) {
		changed = append(changed, FieldSymbol)
	}
	if !equalPtr(a.TacticalName, b.TacticalName) {
		changed = append(changed, FieldTacticalName)
	}
	if !slices.Equal(groupIDs(a.Groups), groupIDs(b.Groups)) {
		changed = append(changed, FieldGroups)
	}
	return changed
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

// CleanupHistory periodically deletes position history entries measured
// more than retention ago until ctx is done. Each unit's newest entry is
// kept, so its current position stays in its history.
func (s *Service) CleanupHistory(ctx context.Context, retention, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			deleted, err := s.deleteHistoryBefore(ctx, time.Now().Add(-retention))
			if err != nil {
				slog.Error("cleanup position history", "err", err)
			} else if deleted > 0 {
				slog.Info("cleaned up position history", "deleted", deleted)
			}
		}
	}
}

// deleteHistoryBefore deletes the position history entries measured before
// cutoff, except each unit's newest one, and returns how many it deleted.
func (s *Service) deleteHistoryBefore(ctx context.Context, cutoff time.Time) (int64, error) {
	res := s.db.WithContext(ctx).Exec(`
		DELETE FROM unit_positions
		WHERE timestamp < ?
			AND id NOT IN (
				SELECT DISTINCT ON (unit_id) id FROM unit_positions
				ORDER BY unit_id, timestamp DESC, id DESC
			)`, cutoff)
	if res.Error != nil {
		return 0, fmt.Errorf("delete position history: %w", res.Error)
	}
	return res.RowsAffected, nil
}

// recordPosition appends the unit's current position, if it has one, to its
// history. by is nil for positions synced from another instance.
func recordPosition(tx *gorm.DB, unit *models.Unit, by *models.User) error {
	if !unit.HasPosition() {
		return nil
	}
	var recordedBy *uint
	if by != nil {
		recordedBy = &by.ID
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
		RecordedByID: recordedBy,
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

// sameStatus reports whether a and b have the same status, including its
// timestamp, or both have none.
func sameStatus(a, b *models.Unit) bool {
	if !equalPtr(a.Status, b.Status) {
		return false
	}
	return a.StatusTimestamp == b.StatusTimestamp ||
		(a.StatusTimestamp != nil && b.StatusTimestamp != nil && a.StatusTimestamp.Equal(*b.StatusTimestamp))
}

func equalPtr[T comparable](a, b *T) bool {
	return a == b || (a != nil && b != nil && *a == *b)
}

// groupIDs returns the IDs of groups, sorted; it is never nil.
func groupIDs(groups []models.Group) []uint {
	ids := make([]uint, len(groups))
	for i, g := range groups {
		ids[i] = g.ID
	}
	slices.Sort(ids)
	return ids
}

// normalizeGroupIDs returns ids sorted and without duplicates; it is never
// nil.
func normalizeGroupIDs(ids []uint) []uint {
	out := slices.Clone(ids)
	if out == nil {
		out = []uint{}
	}
	slices.Sort(out)
	return slices.Compact(out)
}

// findGroups returns the groups with the given IDs by name, or
// ErrGroupNotFound if one doesn't exist.
func findGroups(tx *gorm.DB, ids []uint) ([]models.Group, error) {
	ids = normalizeGroupIDs(ids)
	if len(ids) == 0 {
		return nil, nil
	}
	var groups []models.Group
	if err := tx.Where("id IN ?", ids).Order("name").Find(&groups).Error; err != nil {
		return nil, fmt.Errorf("find groups: %w", err)
	}
	if len(groups) != len(ids) {
		return nil, ErrGroupNotFound
	}
	return groups, nil
}

// loadGroups returns the groups the unit is assigned to, by name.
func loadGroups(tx *gorm.DB, id uuid.UUID) ([]models.Group, error) {
	var groups []models.Group
	err := tx.Joins("JOIN unit_groups ON unit_groups.group_id = groups.id").
		Where("unit_groups.unit_id = ?", id).
		Order("groups.name").
		Find(&groups).Error
	if err != nil {
		return nil, fmt.Errorf("get groups of unit %s: %w", id, err)
	}
	return groups, nil
}

// replaceGroups assigns the unit to exactly the given groups.
func replaceGroups(tx *gorm.DB, id uuid.UUID, groups []models.Group) error {
	if err := tx.Where("unit_id = ?", id).Delete(&models.UnitGroup{}).Error; err != nil {
		return fmt.Errorf("clear groups of unit %s: %w", id, err)
	}
	if len(groups) == 0 {
		return nil
	}
	rows := make([]models.UnitGroup, len(groups))
	for i, g := range groups {
		rows[i] = models.UnitGroup{UnitID: id, GroupID: g.ID}
	}
	if err := tx.Create(&rows).Error; err != nil {
		return fmt.Errorf("assign groups to unit %s: %w", id, err)
	}
	return nil
}

func (s *Service) preload(ctx context.Context) *gorm.DB {
	return s.db.WithContext(ctx).Preload("CreatedBy").Preload("UpdatedBy").Preload("SyncRemote").
		Preload("Groups", func(db *gorm.DB) *gorm.DB { return db.Order("name") })
}

// inputOf returns the unit's editable fields.
func inputOf(unit *models.Unit) Input {
	in := Input{Name: unit.Name, Symbol: unit.Symbol, TacticalName: unit.TacticalName, GroupIDs: groupIDs(unit.Groups)}
	if unit.Status != nil && unit.StatusTimestamp != nil {
		in.Status = &Status{Code: *unit.Status, Timestamp: *unit.StatusTimestamp}
	}
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

	if in.Status == nil {
		unit.Status, unit.StatusTimestamp = nil, nil
	} else {
		if in.Status.Code < MinStatus || in.Status.Code > MaxStatus {
			return ErrInvalidStatus
		}
		code, ts := in.Status.Code, in.Status.Timestamp.UTC()
		unit.Status, unit.StatusTimestamp = &code, &ts
	}

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
