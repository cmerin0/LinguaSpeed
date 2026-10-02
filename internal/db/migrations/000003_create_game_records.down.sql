-- Migration 003 (down): drop the game_records table and its index.
DROP INDEX IF EXISTS idx_gr_difficulty;
DROP TABLE IF EXISTS game_records;
