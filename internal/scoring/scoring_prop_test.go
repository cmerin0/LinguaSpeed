// Package scoring_test contains property-based tests for the Compute function.
// Uses pgregory.net/rapid to generate arbitrary inputs.
package scoring_test

import (
	"testing"

	"pgregory.net/rapid"

	"linguaspeed/internal/scoring"
)

// TestPropertyScoring_NonNegativity verifies Property 2:
// For any valid non-negative timeMS and failedAttempts, Compute never returns < 0.
//
// Feature: lingua-speed, Property 2: Scoring formula non-negativity
// Validates: Requirements 8.2
func TestPropertyScoring_NonNegativity(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		timeMS := rapid.Int64Range(0, 1_000_000).Draw(t, "timeMS")
		failed := rapid.IntRange(0, 100).Draw(t, "failedAttempts")

		score := scoring.Compute(timeMS, failed, 0, 0)

		if score < 0 {
			t.Fatalf("Compute(%d, %d, 0, 0) = %d, want >= 0", timeMS, failed, score)
		}
	})
}

// TestPropertyScoring_DegradeWithFailures verifies part of Property 3:
// Holding timeMS fixed, increasing failedAttempts by 1 must not increase the score.
//
// Feature: lingua-speed, Property 3: Scoring formula degrades with failures and time
// Validates: Requirements 8.1, 8.2
func TestPropertyScoring_DegradeWithFailures(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		timeMS := rapid.Int64Range(0, 1_000_000).Draw(t, "timeMS")
		failed := rapid.IntRange(0, 99).Draw(t, "failedAttempts")

		score1 := scoring.Compute(timeMS, failed, 0, 0)
		score2 := scoring.Compute(timeMS, failed+1, 0, 0)

		// why: adding one more failed attempt must reduce or equal the score -
		// it can never increase it because the penalty term grows by exactly DefaultPenalty.
		if score2 > score1 {
			t.Fatalf("score increased with more failures: Compute(%d,%d)=%d > Compute(%d,%d)=%d",
				timeMS, failed+1, score2, timeMS, failed, score1)
		}
	})
}

// TestPropertyScoring_DegradeWithTime verifies the other half of Property 3:
// Holding failedAttempts fixed, increasing timeMS must not increase the score.
//
// Feature: lingua-speed, Property 3: Scoring formula degrades with failures and time
// Validates: Requirements 8.1, 8.2
func TestPropertyScoring_DegradeWithTime(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		timeMS1 := rapid.Int64Range(0, 999_900).Draw(t, "timeMS1")
		timeMS2 := rapid.Int64Range(timeMS1, 1_000_000).Draw(t, "timeMS2")
		failed := rapid.IntRange(0, 100).Draw(t, "failedAttempts")

		score1 := scoring.Compute(timeMS1, failed, 0, 0)
		score2 := scoring.Compute(timeMS2, failed, 0, 0)

		// why: more time means a larger time penalty, so score2 must be <= score1.
		if score2 > score1 {
			t.Fatalf("score increased with more time: Compute(%d,%d)=%d > Compute(%d,%d)=%d",
				timeMS2, failed, score2, timeMS1, failed, score1)
		}
	})
}
