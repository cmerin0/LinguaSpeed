// Package domain defines the core value types and sentinel errors used across
// all layers of the LinguaSpeed application.
package domain

import "errors"

// ErrNotFound is returned when a requested resource does not exist.
var ErrNotFound = errors.New("not found")

// ErrValidation is returned when input fails validation rules.
var ErrValidation = errors.New("validation error")

// ErrConflict is returned when an operation conflicts with existing state
// (e.g. deactivating an already-inactive tongue-twister).
var ErrConflict = errors.New("conflict")

// ErrUnauthorized is returned when credentials are missing or invalid.
var ErrUnauthorized = errors.New("unauthorized")

// ErrForbidden is returned when credentials are valid but the role is insufficient.
var ErrForbidden = errors.New("forbidden")

// ErrCacheUnavailable is returned when the Redis cache cannot be reached.
var ErrCacheUnavailable = errors.New("cache unavailable")
