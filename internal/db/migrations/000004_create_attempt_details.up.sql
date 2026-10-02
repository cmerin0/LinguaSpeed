-- Migration 004 (up): create the attempt_details table.
-- Each row records one tongue-twister shown during a game session.
-- ON DELETE CASCADE ensures attempt details are cleaned up when their
-- parent game record is deleted (e.g. in test teardown).
-- The game_record_id index makes fetching all attempts for a game fast.

CREATE TABLE attempt_details (
    id                BIGSERIAL   PRIMARY KEY,
    game_record_id    BIGINT      NOT NULL
                                  REFERENCES game_records(id) ON DELETE CASCADE,
    tongue_twister_id BIGINT      NOT NULL
                                  REFERENCES tongue_twisters(id),
    attempt_count     INTEGER     NOT NULL CHECK (attempt_count >= 1),
    time_taken_ms     BIGINT      NOT NULL CHECK (time_taken_ms >= 0),
    success           BOOLEAN     NOT NULL
);

CREATE INDEX idx_ad_game_record ON attempt_details (game_record_id);
