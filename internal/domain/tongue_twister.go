// Package domain defines the core value types and sentinel errors used across
// all layers of the LinguaSpeed application.
package domain

import "time"

// Difficulty represents the skill level of a tongue-twister.
// It is stored as a string in the database to keep the schema readable.
type Difficulty string

const (
	// DifficultyEasy is the easiest difficulty level.
	DifficultyEasy Difficulty = "easy"
	// DifficultyMedium is the intermediate difficulty level.
	DifficultyMedium Difficulty = "medium"
	// DifficultyHard is the most challenging difficulty level.
	DifficultyHard Difficulty = "hard"
)

// IsValidDifficulty reports whether s is one of the three accepted difficulty values.
// It is used by validation helpers in the service and handler layers.
func IsValidDifficulty(s string) bool {
	switch Difficulty(s) {
	case DifficultyEasy, DifficultyMedium, DifficultyHard:
		return true
	}
	return false
}

// TongueTwister is a Spanish phrase shown to players during a game session.
type TongueTwister struct {
	// ID is the database-assigned primary key.
	ID int64
	// Text is the full Spanish phrase the player must type.
	Text string
	// Difficulty is the manually assigned skill level.
	Difficulty Difficulty
	// Active indicates whether this twister is available for play.
	// Inactive twisters are hidden from game sessions but not deleted.
	Active    bool
	CreatedAt time.Time
	UpdatedAt time.Time
}
