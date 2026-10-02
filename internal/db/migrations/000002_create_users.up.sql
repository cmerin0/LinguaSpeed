-- Migration 002 (up): create the user_role enum and users table.
-- The user_role enum enforces the role constraint at the database level,
-- preventing invalid roles from ever being stored regardless of application logic.

CREATE TYPE user_role AS ENUM ('admin', 'moderator');

CREATE TABLE users (
    id              BIGSERIAL       PRIMARY KEY,
    username        VARCHAR(64)     NOT NULL UNIQUE,
    hashed_password VARCHAR(72)     NOT NULL,   -- bcrypt max effective input is 72 bytes
    role            user_role       NOT NULL,
    created_at      TIMESTAMPTZ     NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ     NOT NULL DEFAULT NOW()
);
