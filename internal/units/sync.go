package units

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"go-unit-mangement/internal/models"
)

// UpsertSynced mirrors a unit of the sync remote remoteID (see internal/sync)
// under the same ID: it creates the unit or updates it to in, recording
// position changes in its history without a user. It reports whether anything
// changed; an unchanged unit is not written and publishes no event. A unit
// with this ID that isn't mirrored from remoteID is left alone
// (ErrNotMirrored), so two instances syncing from each other don't take over
// each other's units.
func (s *Service) UpsertSynced(ctx context.Context, remoteID, id uuid.UUID, in Input) (bool, error) {
	created, changed := false, false
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var unit models.Unit
		err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&unit, "id = ?", id).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			unit = models.Unit{ID: id, SyncRemoteID: &remoteID}
			if err := apply(&unit, in); err != nil {
				return err
			}
			if err := tx.Omit(clause.Associations).Create(&unit).Error; err != nil {
				return mapWriteError(err, "create synced unit")
			}
			created, changed = true, true
			return recordPosition(tx, &unit, nil)
		}
		if err != nil {
			return fmt.Errorf("get unit %s: %w", id, err)
		}
		if unit.SyncRemoteID == nil || *unit.SyncRemoteID != remoteID {
			return ErrNotMirrored
		}
		before := unit
		if err := apply(&unit, in); err != nil {
			return err
		}
		if len(Changed(&before, &unit)) == 0 {
			return nil
		}
		changed = true
		res := tx.Model(&unit).
			Select("*").Omit("id", "created_at", "created_by_id", "sync_remote_id", clause.Associations).
			Updates(&unit)
		if res.Error != nil {
			return mapWriteError(res.Error, fmt.Sprintf("update synced unit %s", id))
		}
		if samePosition(&before, &unit) {
			return nil
		}
		return recordPosition(tx, &unit, nil)
	})
	if err != nil || !changed {
		return false, err
	}
	unit, err := s.Get(ctx, id)
	if err != nil {
		return true, err
	}
	typ := EventUpdated
	if created {
		typ = EventCreated
	}
	s.events.Publish(Event{Type: typ, ID: id, Unit: unit})
	return true, nil
}

// DeleteSynced deletes the unit mirrored from remoteID with this ID. It
// reports whether there was one.
func (s *Service) DeleteSynced(ctx context.Context, remoteID, id uuid.UUID) (bool, error) {
	res := s.db.WithContext(ctx).Where("id = ? AND sync_remote_id = ?", id, remoteID).Delete(&models.Unit{})
	if res.Error != nil {
		return false, fmt.Errorf("delete synced unit %s: %w", id, res.Error)
	}
	if res.RowsAffected == 0 {
		return false, nil
	}
	s.events.Publish(Event{Type: EventDeleted, ID: id})
	return true, nil
}

// SyncedIDs returns the IDs of the units mirrored from remoteID.
func (s *Service) SyncedIDs(ctx context.Context, remoteID uuid.UUID) ([]uuid.UUID, error) {
	var ids []uuid.UUID
	err := s.db.WithContext(ctx).Model(&models.Unit{}).Where("sync_remote_id = ?", remoteID).Pluck("id", &ids).Error
	if err != nil {
		return nil, fmt.Errorf("list units synced from %s: %w", remoteID, err)
	}
	return ids, nil
}
