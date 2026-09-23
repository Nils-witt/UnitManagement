package database

import (
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

	if err := db.AutoMigrate(&models.User{}, &models.Session{}, &models.Unit{}); err != nil {
		return nil, fmt.Errorf("migrate database: %w", err)
	}

	return db, nil
}
