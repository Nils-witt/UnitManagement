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
	// are nil for local accounts. Accounts are looked up by subject alone,
	// which identifies the user at the provider for good, unlike the
	// username or email claims; the issuer is the one the account was first
	// seen from. The column
	// names are explicit because GORM would otherwise derive o_id_c_issuer.
	OIDCIssuer  *string `gorm:"column:oidc_issuer;uniqueIndex:idx_users_oidc_identity"`
	OIDCSubject *string `gorm:"column:oidc_subject;uniqueIndex:idx_users_oidc_identity"`
	// Groups are the SSO account's groups at the provider as of their last
	// sign-in; local accounts have none.
	Groups    []Group `gorm:"many2many:user_groups"`
	CreatedAt time.Time
	UpdatedAt time.Time
}

// Group is a group at the SSO provider. Groups are created the first time a
// member signs in and kept when their last member leaves.
type Group struct {
	ID        uint   `gorm:"primaryKey"`
	Name      string `gorm:"uniqueIndex;not null"`
	Users     []User `gorm:"many2many:user_groups"`
	CreatedAt time.Time
	UpdatedAt time.Time
}

// UserGroup is a membership. It is the explicit join table of User.Groups
// (see database.Connect), so memberships go when either side is deleted.
type UserGroup struct {
	UserID  uint   `gorm:"primaryKey"`
	User    *User  `gorm:"constraint:OnDelete:CASCADE"`
	GroupID uint   `gorm:"primaryKey;index"`
	Group   *Group `gorm:"constraint:OnDelete:CASCADE"`
}

// SSO reports whether the account is linked to an SSO identity.
func (u *User) SSO() bool { return u.OIDCSubject != nil }

// HasPassword reports whether the account can sign in with a password.
func (u *User) HasPassword() bool { return u.PasswordHash != "" }

// Session is a server-side login session backing an access token (a JWT
// whose "jti" is the session ID). Only the SHA-256 hash of the ID is stored,
// so a database leak does not expose live sessions.
type Session struct {
	ID        uint      `gorm:"primaryKey"`
	TokenHash string    `gorm:"uniqueIndex;not null"`
	UserID    uint      `gorm:"index;not null"`
	User      User      `gorm:"constraint:OnDelete:CASCADE"`
	ExpiresAt time.Time `gorm:"index;not null"`
	// APIToken marks sessions an administrator issued (see
	// auth.Service.CreateToken) rather than sign-ins; only those have a Name.
	APIToken  bool   `gorm:"not null;default:false"`
	Name      string `gorm:"not null;default:''"`
	CreatedAt time.Time
}

// Unit is a tracked unit. Its last known position is optional: the Position
// columns are either all set (Height and Accuracy may still be nil) or all
// nil.
type Unit struct {
	ID   uuid.UUID `gorm:"type:uuid;primaryKey"`
	Name string    `gorm:"uniqueIndex;not null"`
	// Latitude and Longitude are WGS 84 degrees, Height is meters.
	// Accuracy is the horizontal accuracy radius in meters.
	Latitude          *float64 `gorm:"type:double precision"`
	Longitude         *float64 `gorm:"type:double precision"`
	Height            *float64 `gorm:"type:double precision"`
	Accuracy          *float64 `gorm:"type:double precision"`
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

// UnitPosition is one entry of a unit's position history. An entry is added
// whenever a unit's position is set or changes; the history is deleted with
// the unit.
type UnitPosition struct {
	ID     uint      `gorm:"primaryKey"`
	UnitID uuid.UUID `gorm:"type:uuid;not null;index:idx_unit_positions_unit_timestamp,priority:1"`
	Unit   *Unit     `gorm:"constraint:OnDelete:CASCADE"`
	// Latitude and Longitude are WGS 84 degrees, Height is meters.
	// Accuracy is the horizontal accuracy radius in meters.
	Latitude  float64  `gorm:"type:double precision;not null"`
	Longitude float64  `gorm:"type:double precision;not null"`
	Height    *float64 `gorm:"type:double precision"`
	Accuracy  *float64 `gorm:"type:double precision"`
	// Timestamp is when the position was measured.
	Timestamp time.Time `gorm:"not null;index:idx_unit_positions_unit_timestamp,priority:2,sort:desc"`
	CreatedAt time.Time
	// RecordedByID becomes nil when that user is deleted.
	RecordedByID *uint
	RecordedBy   *User `gorm:"constraint:OnDelete:SET NULL"`
}
