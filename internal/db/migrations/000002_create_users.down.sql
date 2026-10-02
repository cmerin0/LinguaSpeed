-- Migration 002 (down): drop the users table and user_role enum.
DROP TABLE IF EXISTS users;
DROP TYPE IF EXISTS user_role;
