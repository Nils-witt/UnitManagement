package auth

import (
	"context"
	"fmt"

	"gorm.io/gorm"

	"go-unit-mangement/internal/models"
)

// ListGroups returns all groups by name, each with its members by username.
func (s *Service) ListGroups(ctx context.Context) ([]models.Group, error) {
	var groups []models.Group
	err := s.db.WithContext(ctx).
		Preload("Users", func(db *gorm.DB) *gorm.DB { return db.Order("username") }).
		Order("name").
		Find(&groups).Error
	if err != nil {
		return nil, fmt.Errorf("list groups: %w", err)
	}
	return groups, nil
}
