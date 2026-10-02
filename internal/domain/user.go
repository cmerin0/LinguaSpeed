// Package domain defines the core value types and sentinel errors used across
// all layers of the LinguaSpeed application.
package domain

import "time"

// Role represents the access level of a user account.
// Only "admin" has real permissions in v1; "moderator" is reserved for future use.
type Role string

const (
	// RoleAdmin is the administrator role with full tongue-twister management access.
	RoleAdmin Role = "admin"
	// RoleModerator is reserved for future use.
	RoleModerator Role = "moderator"
)

// User represents an admin account stored in the database.
// Players are anonymous and are NOT represented by this type.
type User struct {
	// ID is the database-assigned primary key.
	ID int64
	// Username is the unique login name for the admin account.
	Username string
	// HashedPassword is the bcrypt hash of the account password.
	// It is never returned in API responses or logged.
	HashedPassword string
	// Role is the access level of this account.
	Role      Role
	CreatedAt time.Time
	UpdatedAt time.Time
}
