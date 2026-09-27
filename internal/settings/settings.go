// Package settings stores the instance-wide settings administrators change in
// the web UI. They are kept in memory as well, since the Content-Security-Policy
// of every response depends on them.
package settings

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"sync"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"go-unit-mangement/internal/models"
)

// Keys of the settings table.
const keyMapStyleURL = "map_style_url"

// maxURLLength bounds the stored URL, which ends up in every response header.
const maxURLLength = 2048

// ErrInvalid is wrapped by the errors Update returns for invalid input.
var ErrInvalid = errors.New("invalid settings")

// Settings are the instance-wide settings. Zero values mean the default.
type Settings struct {
	// MapStyleURL is the MapLibre style JSON the map loads; empty uses the
	// built-in OpenStreetMap style.
	MapStyleURL string
}

// Validate reports the first invalid field, wrapping ErrInvalid.
func (s Settings) Validate() error {
	if s.MapStyleURL == "" {
		return nil
	}
	if len(s.MapStyleURL) > maxURLLength {
		return fmt.Errorf("%w: mapStyleUrl must be at most %d characters", ErrInvalid, maxURLLength)
	}
	u, err := url.Parse(s.MapStyleURL)
	if err != nil || (u.Scheme != "https" && u.Scheme != "http") || u.Host == "" || u.User != nil {
		return fmt.Errorf("%w: mapStyleUrl must be an absolute http(s) URL", ErrInvalid)
	}
	// The origin goes into the Content-Security-Policy, where characters
	// such as ';' would start another directive.
	if strings.Trim(u.Host, "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789.-:[]") != "" {
		return fmt.Errorf("%w: mapStyleUrl has an invalid host", ErrInvalid)
	}
	return nil
}

// MapOrigin is the origin (scheme and host) of MapStyleURL, or "" when it is
// unset. The browser must be allowed to load the style, and the tiles,
// sprites and fonts it references, from there.
func (s Settings) MapOrigin() string {
	u, err := url.Parse(s.MapStyleURL)
	if s.MapStyleURL == "" || err != nil {
		return ""
	}
	return u.Scheme + "://" + u.Host
}

type Service struct {
	db *gorm.DB

	mu      sync.RWMutex
	current Settings
}

// NewService loads the stored settings.
func NewService(ctx context.Context, db *gorm.DB) (*Service, error) {
	s := &Service{db: db}
	var rows []models.Setting
	if err := db.WithContext(ctx).Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("load settings: %w", err)
	}
	for _, row := range rows {
		if row.Key == keyMapStyleURL {
			s.current.MapStyleURL = row.Value
		}
	}
	return s, nil
}

// Get returns the current settings.
func (s *Service) Get() Settings {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.current
}

// Update validates and stores in, returning the names (as in the API) of the
// fields that changed.
func (s *Service) Update(ctx context.Context, in Settings) ([]string, error) {
	if err := in.Validate(); err != nil {
		return nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if in == s.current {
		return nil, nil
	}
	row := models.Setting{Key: keyMapStyleURL, Value: in.MapStyleURL}
	err := s.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "key"}},
		DoUpdates: clause.AssignmentColumns([]string{"value", "updated_at"}),
	}).Create(&row).Error
	if err != nil {
		return nil, fmt.Errorf("store settings: %w", err)
	}
	s.current = in
	return []string{"mapStyleUrl"}, nil
}
