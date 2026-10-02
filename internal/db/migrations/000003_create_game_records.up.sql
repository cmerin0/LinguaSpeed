-- Migration 003 (up): create the game_records table.
-- Game records are written once at game end (win or loss) and never mutated.
-- The difficulty index supports future analytics queries filtered by difficulty.

CREATE TABLE game_records (
    id          BIGSERIAL       PRIMARY KEY,
    nickname    VARCHAR(32)     NOT NULL,
    final_score INTEGER         NOT NULL CHECK (final_score >= 0),
    difficulty  VARCHAR(10)     NOT NULL
                                CHECK (difficulty IN ('easy', 'medium', 'hard')),
    started_at  TIMESTAMPTZ     NOT NULL,
    ended_at    TIMESTAMPTZ     NOT NULL
);

CREATE INDEX idx_gr_difficulty ON game_records (difficulty);
