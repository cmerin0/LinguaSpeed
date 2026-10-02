-- Migration 001 (down): drop the tongue_twisters table and its index.
DROP INDEX IF EXISTS idx_tt_difficulty_active;
DROP TABLE IF EXISTS tongue_twisters;
