package models

import (
	"time"

	"github.com/google/uuid"
)

type User struct {
	ID       uint   `gorm:"primaryKey"`
	Username string `gorm:"uniqueIndex;not null"`
	// PasswordHash is empty for accounts that can only sign in via SSO.
	PasswordHash string `gorm:"not null"`
	IsAdmin      bool   `gorm:"not null;default:false"`
	// OIDCIssuer and OIDCSubject link the account to an SSO identity; both
	// are nil for local accounts. Together they identify the user at the
	// provider for good, unlike the username or email claims. The column
	// names are explicit because GORM would otherwise derive o_id_c_issuer.
	OIDCIssuer  *string `gorm:"column:oidc_issuer;uniqueIndex:idx_users_oidc_identity"`
	OIDCSubject *string `gorm:"column:oidc_subject;uniqueIndex:idx_users_oidc_identity"`
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// SSO reports whether the account is linked to an SSO identity.
func (u *User) SSO() bool { return u.OIDCSubject != nil }

// HasPassword reports whether the account can sign in with a password.
func (u *User) HasPassword() bool { return u.PasswordHash != "" }

// Session is a server-side login session. Only the SHA-256 hash of the
// session token is stored, so a database leak does not expose live sessions.
type Session struct {
	ID        uint      `gorm:"primaryKey"`
	TokenHash string    `gorm:"uniqueIndex;not null"`
	UserID    uint      `gorm:"index;not null"`
	User      User      `gorm:"constraint:OnDelete:CASCADE"`
	ExpiresAt time.Time `gorm:"index;not null"`
	CreatedAt time.Time
}

// Unit is a tracked unit. Its last known position is optional: the Position
// columns are either all set (Height may still be nil) or all nil.
type Unit struct {
	ID   uuid.UUID `gorm:"type:uuid;primaryKey"`
	Name string    `gorm:"uniqueIndex;not null"`
	// Latitude and Longitude are WGS 84 degrees, Height is meters.
	Latitude          *float64 `gorm:"type:double precision"`
	Longitude         *float64 `gorm:"type:double precision"`
	Height            *float64 `gorm:"type:double precision"`
	PositionTimestamp *time.Time
	// Symbol is nil when the unit has no tactical symbol.
	Symbol *UnitSymbol `gorm:"serializer:json;type:jsonb"`
	// TacticalName is nil when the unit has no tactical name.
	TacticalName *TacticalName `gorm:"serializer:json;type:jsonb"`
	CreatedAt    time.Time
	UpdatedAt    time.Time
	// CreatedByID and UpdatedByID become nil when that user is deleted.
	CreatedByID *uint
	CreatedBy   *User `gorm:"constraint:OnDelete:SET NULL"`
	UpdatedByID *uint
	UpdatedBy   *User `gorm:"constraint:OnDelete:SET NULL"`
}

// UnitSymbol describes a tactical symbol (DV 102) as the component IDs of the
// @taktische-zeichen/core library, which renders it in the web UI. Empty
// fields are left out of the symbol. It is stored and served as JSON.
type UnitSymbol struct {
	Grundzeichen     string `json:"grundzeichen,omitempty"`
	Organisation     string `json:"organisation,omitempty"`
	Fachaufgabe      string `json:"fachaufgabe,omitempty"`
	Einheit          string `json:"einheit,omitempty"`
	Verwaltungsstufe string `json:"verwaltungsstufe,omitempty"`
	Funktion         string `json:"funktion,omitempty"`
	Symbol           string `json:"symbol,omitempty"`
}

// Fields returns pointers to all components, for validation.
func (s *UnitSymbol) Fields() []*string {
	return []*string{
		&s.Grundzeichen, &s.Organisation, &s.Fachaufgabe, &s.Einheit,
		&s.Verwaltungsstufe, &s.Funktion, &s.Symbol,
	}
}

// TacticalName is a unit's radio call sign, e.g. "Rotkreuz Musterstadt
// 12/83-1", split into its parts. Empty parts are left out. It is stored and
// served as JSON.
type TacticalName struct {
	Organisation        string `json:"organisation,omitempty"`
	RegionalAssociation string `json:"regionalAssociation,omitempty"`
	LocalAssociation    string `json:"localAssociation,omitempty"`
	Function            string `json:"function,omitempty"`
	Number              string `json:"number,omitempty"`
}

// Fields returns pointers to all parts, for validation.
func (n *TacticalName) Fields() []*string {
	return []*string{
		&n.Organisation, &n.RegionalAssociation, &n.LocalAssociation, &n.Function, &n.Number,
	}
}

// HasPosition reports whether the unit's position is known.
func (u *Unit) HasPosition() bool { return u.Latitude != nil && u.Longitude != nil }
