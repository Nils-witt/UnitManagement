package database

import (
	"errors"
	"fmt"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"go-unit-mangement/internal/models"
)

func Connect(dsn string) (*gorm.DB, error) {
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Warn),
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

	// Memberships use an explicit join model so their foreign keys cascade.
	err = errors.Join(
		db.SetupJoinTable(&models.User{}, "Groups", &models.UserGroup{}),
		db.SetupJoinTable(&models.Group{}, "Users", &models.UserGroup{}),
	)
	if err != nil {
		return nil, fmt.Errorf("set up user_groups join table: %w", err)
	}
	if err := db.AutoMigrate(&models.User{}, &models.Session{}, &models.Group{}, &models.UserGroup{}, &models.Unit{}, &models.UnitPosition{}); err != nil {
		return nil, fmt.Errorf("migrate database: %w", err)
	}
	// Units positioned before the history existed start it with their current
	// position. Afterwards every unit's position is in its history, so this
	// finds nothing.
	err = db.Exec(`
		INSERT INTO unit_positions (unit_id, latitude, longitude, height, timestamp, created_at, recorded_by_id)
		SELECT u.id, u.latitude, u.longitude, u.height, u.position_timestamp, now(), u.updated_by_id
		FROM units u
		WHERE u.latitude IS NOT NULL AND u.longitude IS NOT NULL AND u.position_timestamp IS NOT NULL
			AND NOT EXISTS (SELECT 1 FROM unit_positions p WHERE p.unit_id = u.id)`).Error
	if err != nil {
		return nil, fmt.Errorf("backfill position history: %w", err)
	}

	return db, nil
}
