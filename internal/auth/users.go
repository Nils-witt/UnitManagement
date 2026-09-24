package auth

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"go-unit-mangement/internal/models"
)

const (
	MinPasswordLength = 8
	MaxUsernameLength = 64
)

var (
	ErrUserNotFound     = errors.New("user not found")
	ErrUsernameTaken    = errors.New("username is already taken")
	ErrInvalidUsername  = fmt.Errorf("username must be 1 to %d characters", MaxUsernameLength)
	ErrPasswordTooShort = fmt.Errorf("password must be at least %d characters", MinPasswordLength)
)

// byName orders preloaded groups.
func byName(db *gorm.DB) *gorm.DB { return db.Order("name") }

func (s *Service) ListUsers(ctx context.Context) ([]models.User, error) {
	var users []models.User
	if err := s.db.WithContext(ctx).Preload("Groups", byName).Order("username").Find(&users).Error; err != nil {
		return nil, fmt.Errorf("list users: %w", err)
	}
	return users, nil
}

func (s *Service) GetUser(ctx context.Context, id uint) (*models.User, error) {
	var user models.User
	err := s.db.WithContext(ctx).Preload("Groups", byName).First(&user, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrUserNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get user %d: %w", id, err)
	}
	return &user, nil
}

// CreateUser creates a local account that signs in with a password.
func (s *Service) CreateUser(ctx context.Context, username, password string, isAdmin bool) (*models.User, error) {
	if err := validatePassword(password); err != nil {
		return nil, err
	}
	return s.createUser(ctx, username, password, isAdmin)
}

// createUser skips the password policy, so the bootstrap admin can use
// whatever ADMIN_PASSWORD the operator set.
func (s *Service) createUser(ctx context.Context, username, password string, isAdmin bool) (*models.User, error) {
	username, err := normalizeUsername(username)
	if err != nil {
		return nil, err
	}
	hash, err := HashPassword(password)
	if err != nil {
		return nil, err
	}
	user := &models.User{Username: username, PasswordHash: hash, IsAdmin: isAdmin}
	if err := s.db.WithContext(ctx).Create(user).Error; err != nil {
		if errors.Is(err, gorm.ErrDuplicatedKey) {
			return nil, ErrUsernameTaken
		}
		return nil, fmt.Errorf("create user: %w", err)
	}
	return user, nil
}

// UpdateUser sets the user's administrator role and, if password is not
// empty, replaces their password. A new password ends the user's existing
// sessions, so a leaked password stops working everywhere at once.
func (s *Service) UpdateUser(ctx context.Context, id uint, isAdmin bool, password string) (*models.User, error) {
	var hash string
	if password != "" {
		if err := validatePassword(password); err != nil {
			return nil, err
		}
		var err error
		if hash, err = HashPassword(password); err != nil {
			return nil, err
		}
	}

	var user models.User
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Preload("Groups", byName).First(&user, id).Error; err != nil {
			return err
		}
		user.IsAdmin = isAdmin
		if hash != "" {
			user.PasswordHash = hash
			if err := tx.Where("user_id = ?", id).Delete(&models.Session{}).Error; err != nil {
				return err
			}
		}
		// Memberships only change at SSO sign-in.
		return tx.Omit(clause.Associations).Save(&user).Error
	})
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrUserNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("update user %d: %w", id, err)
	}
	return &user, nil
}

// DeleteUser removes the user; their sessions go with them (ON DELETE CASCADE).
func (s *Service) DeleteUser(ctx context.Context, id uint) error {
	res := s.db.WithContext(ctx).Delete(&models.User{}, id)
	if res.Error != nil {
		return fmt.Errorf("delete user %d: %w", id, res.Error)
	}
	if res.RowsAffected == 0 {
		return ErrUserNotFound
	}
	return nil
}

func normalizeUsername(username string) (string, error) {
	username = strings.TrimSpace(username)
	if username == "" || utf8.RuneCountInString(username) > MaxUsernameLength {
		return "", ErrInvalidUsername
	}
	return username, nil
}

func validatePassword(password string) error {
	if utf8.RuneCountInString(password) < MinPasswordLength {
		return ErrPasswordTooShort
	}
	return nil
}
