// Package domain defines the core value types and sentinel errors used across
// all layers of the LinguaSpeed application.
package domain

import "time"

// SessionStatus represents the current state of a game session.
type SessionStatus string

const (
	// StatusActive means the session is in progress.
	StatusActive SessionStatus = "active"
	// StatusEndedWin means the player completed all available tongue-twisters.
	StatusEndedWin SessionStatus = "ended_win"
	// StatusEndedLoss means the player lost all hearts.
	StatusEndedLoss SessionStatus = "ended_loss"
)

// GameSession holds the live state of an in-progress game, stored in Redis.
// All fields are always present in the serialised JSON (no omitempty) so that
// round-trip serialisation is deterministic (Requirement 20).
type GameSession struct {
	// SessionID is the UUID identifying this session, used as the Redis key suffix.
	SessionID string `json:"session_id"`
	// Nickname is the player-chosen display name for the leaderboard.
	Nickname string `json:"nickname"`
	// Difficulty is the chosen game difficulty; tongue-twister selection is filtered by this.
	Difficulty string `json:"difficulty"`
	// Hearts is the remaining life count (0-3). Reaching 0 ends the game as a loss.
	Hearts int `json:"hearts"`
	// Score is the cumulative score accumulated across all successful attempts so far.
	Score int `json:"score"`
	// ShownIDs is the list of tongue-twister IDs already shown in this session.
	// Used to ensure no twister is repeated within a session (Requirement 6.1).
	ShownIDs []int64 `json:"shown_ids"`
	// StartedAt is the UTC timestamp when the session was created.
	StartedAt time.Time `json:"started_at"`
	// Status is the current lifecycle state of the session.
	Status SessionStatus `json:"status"`
}

// AttemptDetail records the result of one tongue-twister within a completed game.
// Written to the database as part of a GameRecord at game end.
type AttemptDetail struct {
	// TongueTwisterID is the ID of the tongue-twister shown.
	TongueTwisterID int64
	// AttemptCount is the number of submissions made for this twister (including failures).
	AttemptCount int
	// TimeTakenMS is the total milliseconds from first keystroke to successful submission.
	// Zero if the player never succeeded on this twister.
	TimeTakenMS int64
	// Success indicates whether the player eventually typed the twister correctly.
	Success bool
}

// GameRecord is the permanent record written to PostgreSQL when a game ends.
type GameRecord struct {
	// Nickname is the player-chosen display name.
	Nickname string
	// FinalScore is the cumulative score at game end.
	FinalScore int
	// Difficulty is the difficulty level chosen for this game.
	Difficulty string
	// StartedAt is when the game session was created.
	StartedAt time.Time
	// EndedAt is when the game ended (win or loss).
	EndedAt time.Time
	// Attempts holds one entry per tongue-twister shown during the game.
	Attempts []AttemptDetail
}

// LeaderboardEntry is a single entry in the public leaderboard.
type LeaderboardEntry struct {
	// Rank is the 1-based position on the leaderboard (1 = highest score).
	Rank int
	// Nickname is the player display name.
	Nickname string
	// Score is the final game score.
	Score int
}
