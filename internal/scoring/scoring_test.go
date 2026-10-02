// Package scoring_test contains unit tests for the Compute function.
package scoring_test

import (
	"testing"

	"linguaspeed/internal/scoring"
)

func TestCompute_perfectAttempt(t *testing.T) {
	// 1000ms, 0 failures, defaults: 1000 - 10 - 0 = 990
	got := scoring.Compute(1000, 0, 0, 0)
	if got != 990 {
		t.Errorf("Compute(1000, 0, 0, 0) = %d, want 990", got)
	}
}

func TestCompute_clampToZero(t *testing.T) {
	// Very slow + many failures should never go negative
	got := scoring.Compute(1_000_000, 100, 0, 0)
	if got != 0 {
		t.Errorf("Compute(1000000, 100, 0, 0) = %d, want 0", got)
	}
}

func TestCompute_customConstants(t *testing.T) {
	// base=500, penalty=100, timeMS=500, failed=1: 500 - 5 - 100 = 395
	got := scoring.Compute(500, 1, 500, 100)
	if got != 395 {
		t.Errorf("Compute(500, 1, 500, 100) = %d, want 395", got)
	}
}
