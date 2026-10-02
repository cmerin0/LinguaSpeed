-- Migration 004 (down): drop the attempt_details table and its index.
DROP INDEX IF EXISTS idx_ad_game_record;
DROP TABLE IF EXISTS attempt_details;
