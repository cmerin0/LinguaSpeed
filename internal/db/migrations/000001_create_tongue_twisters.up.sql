-- Migration 001 (up): create the tongue_twisters table.
-- This is the core content table; the composite index on (difficulty, active)
-- supports the most frequent query pattern: selecting random active twisters
-- filtered by difficulty during a game session.

CREATE TABLE tongue_twisters (
    id          BIGSERIAL       PRIMARY KEY,
    text        VARCHAR(500)    NOT NULL
                                CHECK (char_length(trim(text)) > 0),
    difficulty  VARCHAR(10)     NOT NULL
                                CHECK (difficulty IN ('easy', 'medium', 'hard')),
    active      BOOLEAN         NOT NULL DEFAULT TRUE,
    created_at  TIMESTAMPTZ     NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ     NOT NULL DEFAULT NOW()
);

-- Composite index: covers WHERE difficulty = $1 AND active = TRUE queries.
CREATE INDEX idx_tt_difficulty_active ON tongue_twisters (difficulty, active);
