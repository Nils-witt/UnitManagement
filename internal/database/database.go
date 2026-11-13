package database

import (
	"errors"
	"fmt"
	"log/slog"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"go-unit-mangement/internal/models"
)

func Connect(dsn string) (*gorm.DB, error) {
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: logger.NewSlogLogger(slog.Default(), logger.Config{
			LogLevel:      logger.Warn,
			SlowThreshold: 200 * time.Millisecond,
			// Failed and slow queries are logged with placeholders instead of
			// their values, which can be password hashes or session tokens.
			ParameterizedQueries: true,
			// Callers handle a missing row; it is not worth an error line.
			IgnoreRecordNotFoundError: true,
		}),
		// Every write here is a single statement, so GORM's implicit
		// BEGIN/COMMIT around Create/Delete only adds two round trips.
		SkipDefaultTransaction: true,
		// Maps unique-constraint violations to gorm.ErrDuplicatedKey so a
		// taken username can be reported as a conflict.
		TranslateError: true,
	})
	if err != nil {
		return nil, fmt.Errorf("connect to database: %w", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("get database handle: %w", err)
	}
	// database/sql defaults to unlimited open connections and only 2 idle
	// ones, which causes connection churn under concurrent load.
	sqlDB.SetMaxOpenConns(25)
	sqlDB.SetMaxIdleConns(25)
	sqlDB.SetConnMaxLifetime(30 * time.Minute)
	sqlDB.SetConnMaxIdleTime(5 * time.Minute)

	// Memberships and unit assignments use explicit join models so their
	// foreign keys cascade.
	err = errors.Join(
		db.SetupJoinTable(&models.User{}, "Groups", &models.UserGroup{}),
		db.SetupJoinTable(&models.Group{}, "Users", &models.UserGroup{}),
		db.SetupJoinTable(&models.Unit{}, "Groups", &models.UnitGroup{}),
		db.SetupJoinTable(&models.Group{}, "Units", &models.UnitGroup{}),
	)
	if err != nil {
		return nil, fmt.Errorf("set up group join tables: %w", err)
	}
	if err := db.AutoMigrate(&models.User{}, &models.Session{}, &models.Group{}, &models.UserGroup{}, &models.SyncRemote{}, &models.Unit{}, &models.UnitGroup{}, &models.UnitPosition{}, &models.UnitStatusRequest{}, &models.AuditLog{}, &models.Setting{}, &models.APIKey{}); err != nil {
		return nil, fmt.Errorf("migrate database: %w", err)
	}
	// SSO accounts are looked up by subject alone. Not unique: accounts
	// provisioned while lookups were keyed by issuer and subject may share a
	// subject, and a unique index would then fail to build.
	if err := db.Exec(`CREATE INDEX IF NOT EXISTS idx_users_oidc_subject ON users (oidc_subject) WHERE oidc_subject IS NOT NULL`).Error; err != nil {
		return nil, fmt.Errorf("create oidc subject index: %w", err)
	}
	// Units positioned before the history existed start it with their current
	// position. Afterwards every unit's position is in its history, so this
	// finds nothing.
	err = db.Exec(`
		INSERT INTO unit_positions (unit_id, latitude, longitude, height, accuracy, speed, course, timestamp, created_at, recorded_by_id)
		SELECT u.id, u.latitude, u.longitude, u.height, u.accuracy, u.speed, u.course, u.position_timestamp, now(), u.updated_by_id
		FROM units u
		WHERE u.latitude IS NOT NULL AND u.longitude IS NOT NULL AND u.position_timestamp IS NOT NULL
			AND NOT EXISTS (SELECT 1 FROM unit_positions p WHERE p.unit_id = u.id)`).Error
	if err != nil {
		return nil, fmt.Errorf("backfill position history: %w", err)
	}
	// FMS 0, 5 and 9 used to be stored as the unit's status. They are
	// requests now and leave the status alone, so a unit stored with one
	// gets it as a pending request and an unknown status: what it had before
	// is lost. Afterwards no unit has such a status, so this finds nothing.
	err = db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec(`
			INSERT INTO unit_status_requests (unit_id, code, timestamp)
			SELECT id, status, status_timestamp FROM units
			WHERE status IN (0, 5, 9) AND status_timestamp IS NOT NULL
			ON CONFLICT (unit_id, code) DO UPDATE
				SET timestamp = EXCLUDED.timestamp, acknowledged_at = NULL, acknowledged_by = NULL`).Error; err != nil {
			return err
		}
		return tx.Exec(`UPDATE units SET status = NULL, status_timestamp = NULL WHERE status IN (0, 5, 9)`).Error
	})
	if err != nil {
		return nil, fmt.Errorf("move status requests: %w", err)
	}

	return db, nil
}
