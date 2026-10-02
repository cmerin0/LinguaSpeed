// Package scoring implements the pure scoring formula for a single
// tongue-twister attempt. It has no I/O and no external dependencies.
package scoring

// DefaultBasePoints is the starting score for a successful attempt before
// time and failure penalties are applied.
const DefaultBasePoints = 1000

// DefaultPenalty is the score deducted per failed attempt on a single twister.
const DefaultPenalty = 200

// Compute returns the score for one successfully completed tongue-twister.
//
// timeMS is the elapsed time in milliseconds from the player first keystroke
// to their successful submission. failedAttempts is the count of incorrect
// submissions before the successful one. basePoints and penalty are the
// configurable constants; pass 0 to use the package defaults.
//
// The result is clamped to 0 - a very slow or failure-heavy attempt cannot
// produce a negative score.
func Compute(timeMS int64, failedAttempts int, basePoints, penalty int) int {
	if basePoints == 0 {
		basePoints = DefaultBasePoints
	}
	if penalty == 0 {
		penalty = DefaultPenalty
	}
	// why: integer division of timeMS by 100 converts milliseconds to
	// "deciseconds" for the penalty term, keeping the formula readable while
	// still being sensitive to speed differences at the 100ms granularity.
	score := basePoints - int(timeMS/100) - (failedAttempts * penalty)
	if score < 0 {
		return 0
	}
	return score
}
