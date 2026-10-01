# Implementation Plan: LinguaSpeed

## Overview

Build the LinguaSpeed typing game from the ground up: Go backend (chi, pgx/v5, go-redis/v9), PostgreSQL schema, Redis session/leaderboard, plain HTML/CSS/JS frontend embedded via `go:embed`, Docker Compose orchestration, JWT-protected admin panel, structured `slog` logging, and property-based tests with `pgregory.net/rapid` for all 8 correctness properties defined in the design.

Tasks are ordered strictly by dependency: infrastructure first, then domain types, then repositories, then services, then handlers, then frontend. Each task is completable in a single focused coding session.

---

## Tasks

- [ ] 1. Project bootstrap
  - [x] 1.1 Initialise Go module, directory structure, and ignore files
    - Run `go mod init` with module path `linguaspeed`
    - Create the full package skeleton: `cmd/server/`, `internal/config/`, `internal/db/migrations/`, `internal/cache/`, `internal/domain/`, `internal/repository/`, `internal/service/`, `internal/handler/`, `internal/scoring/`, `static/`
    - Add a minimal `cmd/server/main.go` that compiles (no logic yet, just `func main() {}`)
    - Write `.gitignore` covering:
      - Secrets: `.env`, `.env.*` (except `.env.example`)
      - Go artifacts: compiled binary (`/linguaspeed`, `*.exe`), test binary (`*.test`), test cache (`/tmp/`)
      - Docker: no additional entries needed (compose files are committed)
      - OS noise: `.DS_Store`, `Thumbs.db`
      - Editor: `.vscode/`, `.idea/`, `*.swp`, `*.swo`
      - Testcontainers: `/tmp/testcontainers-*`
    - Write `.dockerignore` covering:
      - `.git/`, `.gitignore`
      - `.env`, `.env.*`
      - `*.md` documentation files
      - `.kiro/` spec directory
      - `*.test` test binaries
      - `tmp/`, `.vscode/`, `.idea/`
      - `Makefile` (build-time only, not needed at runtime)
      - Comment at top: `# Exclude everything not needed to compile the Go binary`
    - _Requirements: 1.3_

  - [x] 1.2 Add all external Go dependencies
    - `go get` the exact versions: `github.com/go-chi/chi/v5`, `github.com/jackc/pgx/v5`, `github.com/redis/go-redis/v9`, `github.com/golang-migrate/migrate/v4`, `github.com/golang-jwt/jwt/v5`, `golang.org/x/crypto`, `github.com/google/uuid`, `pgregory.net/rapid`
    - Commit `go.mod` and `go.sum`
    - _Requirements: 1.1_

  - [-] 1.5 Write `Makefile` with standard targets
    - `make build` — runs `go build -o linguaspeed ./cmd/server`
    - `make test-unit` — runs `go test -v -count=1 ./internal/scoring/... ./internal/domain/... ./internal/handler/...` (fast, no external dependencies)
    - `make test-property` — runs `go test -v -count=1 -run Property ./...` (rapid property tests only)
    - `make test-integration` — runs `go test -v -count=1 -tags integration ./...` (requires Docker via testcontainers)
    - `make test` — runs `make test-unit && make test-property && make test-integration`
    - `make lint` — runs `go vet ./...` (no extra linter dependency required)
    - `make clean` — removes the compiled binary
    - Each target has a one-line comment above it explaining what it runs and when to use it
    - _Requirements: none (developer tooling)_

  - [-] 1.3 Author Docker Compose configuration and `.env.example`
    - Write `docker-compose.yml` with three services: `db` (postgres:16-alpine), `cache` (redis:7-alpine), `server` (builds from `Dockerfile`)
    - `db` and `cache` have healthchecks (`pg_isready` and `redis-cli ping`); `server` has `depends_on: {db: {condition: service_healthy}, cache: {condition: service_healthy}}`
    - Mount named volume `db_data` for the `db` service; no host-facing ports for `db` or `cache`
    - `server` exposes `${SERVER_PORT:-3000}:3000`; all services on isolated `linguaspeed_net` bridge network
    - Write `.env.example` documenting `DB_PASSWORD`, `JWT_SECRET`, `ADMIN_USERNAME`, `ADMIN_PASSWORD`, `SERVER_PORT`, `BASE_POINTS`, `PENALTY`
    - Write a minimal multi-stage `Dockerfile` that builds the Go binary and copies it into a distroless image
    - _Requirements: 1.1, 1.2, 1.5, 1.6_

- [ ] 2. Configuration and startup wiring
  - [~] 2.1 Implement `internal/config/config.go`
    - Define `Config` struct with fields: `DatabaseURL`, `RedisAddr`, `JWTSecret`, `ServerPort` (default `"3000"`), `AdminUsername`, `AdminPassword`, `BasePoints` (default 1000), `Penalty` (default 200)
    - `Load()` reads all values from environment variables; collects every missing required variable name and exits with code 1 after logging all missing names in a single `slog.Error` call
    - Include package-level godoc comment
    - _Requirements: 1.3, 1.4_

  - [~] 2.2 Implement `internal/db/db.go` — pgx connection pool factory
    - `New(ctx, databaseURL)` opens a `pgxpool.Pool` and pings; returns error on failure
    - Include package-level godoc comment and exported symbol comments
    - _Requirements: 1.2_

  - [~] 2.3 Implement `internal/cache/cache.go` — go-redis client factory
    - `New(addr)` creates a `redis.Client`, calls `Ping`; returns error on failure
    - Include package-level godoc comment
    - _Requirements: 1.2_

- [ ] 3. Database schema migrations
  - [~] 3.1 Write SQL migration 001 — `tongue_twisters` table
    - Up: `CREATE TABLE tongue_twisters` with columns `id`, `text` (VARCHAR 500, trimmed non-empty check), `difficulty` (CHECK IN 'easy','medium','hard'), `active` (BOOLEAN DEFAULT TRUE), `created_at`, `updated_at`; create composite index `idx_tt_difficulty_active (difficulty, active)`
    - Down: `DROP TABLE tongue_twisters`
    - _Requirements: 2.3_

  - [~] 3.2 Write SQL migration 002 — `users` table
    - Up: `CREATE TYPE user_role AS ENUM ('admin','moderator')`, `CREATE TABLE users` with columns `id`, `username` (VARCHAR 64 UNIQUE NOT NULL), `hashed_password` (VARCHAR 72), `role user_role NOT NULL`, `created_at`, `updated_at`
    - Down: `DROP TABLE users`, `DROP TYPE user_role`
    - _Requirements: 2.4, 2.7_

  - [~] 3.3 Write SQL migrations 003 and 004 — `game_records` and `attempt_details` tables
    - Migration 003 up: `CREATE TABLE game_records` with `id`, `nickname` (VARCHAR 32), `final_score` (INTEGER CHECK ≥ 0), `difficulty`, `started_at`, `ended_at`; index `idx_gr_difficulty`
    - Migration 004 up: `CREATE TABLE attempt_details` with `id`, `game_record_id` (FK → game_records ON DELETE CASCADE), `tongue_twister_id` (FK → tongue_twisters), `attempt_count` (CHECK ≥ 1), `time_taken_ms` (BIGINT CHECK ≥ 0), `success`; index `idx_ad_game_record`
    - Corresponding down migrations
    - _Requirements: 2.5, 2.6_

  - [~] 3.4 Wire migration runner and seed logic into `cmd/server/main.go`
    - Embed migration files with `//go:embed internal/db/migrations/*.sql`
    - On startup: call `golang-migrate` to apply pending migrations; halt with `slog.Error` + `os.Exit(1)` on failure
    - After migrations: run seed logic — if no admin user exists, insert one with bcrypt-hashed password (cost ≥ 12) using `ADMIN_USERNAME`/`ADMIN_PASSWORD` env vars; if vars absent, abort with error; if admin already exists, log INFO and skip
    - Log INFO for each applied migration and for seed outcome
    - _Requirements: 2.1, 2.2, 2.8, 2.9, 2.10_

- [ ] 4. Domain types
  - [~] 4.1 Implement `internal/domain/` value types
    - `game.go`: `GameSession` struct (all fields from design JSON schema: `SessionID`, `Nickname`, `Difficulty`, `Hearts`, `Score`, `ShownIDs []int64`, `StartedAt time.Time`, `Status`); `AttemptDetail`; `GameRecord`; status constants `StatusActive`, `StatusEndedWin`, `StatusEndedLoss`
    - `tongue_twister.go`: `TongueTwister` struct; `Difficulty` type with constants `DifficultyEasy`, `DifficultyMedium`, `DifficultyHard`; `IsValidDifficulty(s string) bool`
    - `user.go`: `User` struct; `Role` type
    - `errors.go`: sentinel errors `ErrNotFound`, `ErrValidation`, `ErrConflict`, `ErrUnauthorized`, `ErrForbidden`, `ErrCacheUnavailable`
    - All exported symbols must have godoc comments; each file opens with a package-level comment
    - _Requirements: 3.1, 4.4, 5.1, 7.6, 8.2_

- [ ] 5. Scoring package
  - [~] 5.1 Implement `internal/scoring/scoring.go`
    - Export constants `DefaultBasePoints = 1000`, `DefaultPenalty = 200`
    - `Compute(timeMS int64, failedAttempts int, basePoints, penalty int) int` — apply formula `max(0, basePoints - int(timeMS/100) - failedAttempts*penalty)`; use defaults when `basePoints` or `penalty` are 0
    - Package-level godoc comment; exported symbol comments
    - _Requirements: 8.2_

  - [ ]* 5.2 Write property tests for scoring — Properties 2 and 3
    - File: `internal/scoring/scoring_prop_test.go`
    - **Property 2: Scoring formula non-negativity** — for any `timeMS ∈ [0, 1_000_000]` and `failedAttempts ∈ [0, 100]`, `Compute` must return ≥ 0
    - **Property 3: Scoring formula degrades with failures and time** — holding `timeMS` fixed, increasing `failedAttempts` by 1 must not increase score; holding `failedAttempts` fixed, increasing `timeMS` must not increase score
    - Tag each test with `// Feature: lingua-speed, Property N: <text>`
    - **Validates: Requirements 8.1, 8.2**

- [ ] 6. Repository layer
  - [~] 6.1 Implement `internal/repository/tongue_twister.go`
    - `TongueTwisterRepo` with methods: `Create`, `GetByID`, `Update`, `Deactivate`, `List` (with optional difficulty + active filters, max 1000), `NextTwister(ctx, difficulty string, excludeIDs []int64) (*domain.TongueTwister, error)` — uses `id <> ALL($2)` idiom; returns `domain.ErrNotFound` when none remain
    - All exported symbols have godoc comments; `NextTwister` includes a `// why` comment on the ANY/ALL array operator
    - _Requirements: 4.4, 4.5, 4.6, 6.1, 6.2, 6.3, 14.5, 15.5, 16.3, 17.5_

  - [~] 6.2 Implement `internal/repository/user.go`
    - `UserRepo` with methods: `GetByUsername(ctx, username) (*domain.User, error)`, `Create(ctx, username, hashedPassword, role) (*domain.User, error)`, `Exists(ctx) (bool, error)` (for seed check)
    - _Requirements: 2.8, 13.4_

  - [~] 6.3 Implement `internal/repository/session.go` — Redis Game Session CRUD
    - `SessionRepo` with methods: `Create`, `Get`, `Update`, `Delete`
    - `Create` uses `SET key value EX 7200 NX`; include `// why` comment on NX atomicity
    - `Get` uses `GETEX key EX 7200` to refresh sliding TTL; include `// why` comment
    - `Update` uses `SET key value EX 7200`; include `// why` comment
    - Serialize/deserialize `GameSession` via `encoding/json`; on marshal error do NOT write to Redis; on unmarshal error do NOT return partial struct — both return errors immediately (Requirements 20.4, 20.5)
    - Return `domain.ErrCacheUnavailable` on Redis connectivity errors; `domain.ErrNotFound` on missing key
    - _Requirements: 5.1, 5.2, 5.3, 5.4, 5.7, 20.1–20.5_

  - [~] 6.4 Implement `internal/repository/game_record.go`
    - `GameRecordRepo` with method `Save(ctx, pool, record GameRecord, details []AttemptDetail) error`
    - Wrap the `game_records` INSERT and all `attempt_details` INSERTs in a single PostgreSQL transaction; roll back on any error
    - _Requirements: 10.2, 11.2_

  - [~] 6.5 Implement `internal/repository/leaderboard.go`
    - `LeaderboardRepo` with methods: `Upsert(ctx, nickname string, score int) error` and `Top10(ctx) ([]domain.LeaderboardEntry, error)`
    - `Upsert` uses `ZADD leaderboard:all GT <score> <nickname>`; include `// why` comment on GT semantics and tie-breaking behaviour
    - `Top10` uses `ZREVRANGE leaderboard:all 0 9 WITHSCORES`; computes rank as 1-based index
    - Return `domain.ErrCacheUnavailable` on Redis errors
    - _Requirements: 12.1, 12.2, 12.3, 12.6, 12.7, 12.8_

- [~] 7. Checkpoint — repository layer complete
  - Ensure all repository files compile; run `go build ./...`; ask the user if questions arise.

- [ ] 8. Service layer
  - [~] 8.1 Implement `internal/service/game.go` — `GameService`
    - `StartGame(ctx, nickname, difficulty string) (sessionID string, err error)` — validates inputs (nickname 1–32 chars, difficulty enum), generates UUID v4, builds `GameSession`, calls `SessionRepo.Create`; emits `slog.Info` with `session_id` and `difficulty` (not nickname)
    - `NextTwister(ctx, sessionID string) (*domain.TongueTwister, error)` — loads session, checks status active, calls `TongueTwisterRepo.NextTwister` excluding `ShownIDs`; on `ErrNotFound` triggers win condition; on success updates `ShownIDs` in session via `SessionRepo.Update`
    - `SubmitAttempt(ctx, sessionID string, twisterID int64, typedText string, timeTakenMS int64) (*AttemptResult, error)` — loads session, compares typed text to twister text; on mismatch: deduct heart, update session, check for loss condition; on match: compute score via `scoring.Compute`, accumulate, advance session; on game end (win or loss): call `GameService.endGame` (see below)
    - `endGame` private method: writes `GameRecord` + `AttemptDetails` in DB transaction; on DB error retain session + return error; on success delete session from Redis, upsert leaderboard; emit INFO log for game end; emit ERROR log if leaderboard upsert fails
    - Wire `BASE_POINTS` and `PENALTY` from config into `scoring.Compute`
    - _Requirements: 3.1–3.6, 4.4–4.6, 5.1–5.7, 6.1–6.6, 7.3–7.7, 8.1–8.6, 9.1–9.5, 10.1–10.6, 11.1–11.6, 12.2_

  - [~] 8.2 Implement `internal/service/admin.go` — `AdminService`
    - `Login(ctx, username, password string) (token string, err error)` — loads user by username; bcrypt compare; returns `ErrUnauthorized` for wrong credentials (same message regardless of whether username or password is wrong); returns `ErrForbidden` if role ≠ admin; issues JWT (HS256, 8h expiry, `sub` = user ID, `role` = "admin") via `issueAdminToken`; emits WARN log on failure with `username` and `reason`; emits INFO log on success
    - `CreateTongueTwister`, `UpdateTongueTwister`, `DeactivateTongueTwister`, `ListTongueTwisters` — delegate to `TongueTwisterRepo` with input validation; validate text (non-empty, ≤ 500 chars after trim), difficulty enum
    - _Requirements: 13.1–13.13, 14.1–14.5, 15.1–15.6, 16.1–16.8, 17.1–17.6_

  - [~] 8.3 Implement `internal/service/leaderboard.go` — `LeaderboardService`
    - `GetTop10(ctx) ([]domain.LeaderboardEntry, error)` — delegates to `LeaderboardRepo.Top10`
    - _Requirements: 12.3, 12.4, 12.7_

- [ ] 9. Handler and middleware layer
  - [~] 9.1 Implement `internal/handler/middleware.go`
    - `RequestIDMiddleware`: generates UUID v4 per request, stores in context, adds `X-Request-ID` response header
    - `RequestLogMiddleware`: wraps `ResponseWriter` to capture status code; logs one `slog` line per completed request with `method`, `path`, `status`, `latency_ms`, `request_id`; level INFO for 2xx, WARN for 4xx, ERROR for 5xx; never logs request bodies
    - `JWTMiddleware(secret []byte)`: checks Authorization Bearer header (401 if missing); parses JWT with `WithValidMethods([]string{"HS256"})` — include `// why` comment on algorithm pinning; returns 401 on invalid/expired; returns 403 if `role ≠ admin`; injects claims into context on success
    - Full middleware comment block per design commenting standard
    - _Requirements: 13.8–13.10, 21.1–21.4, 21.8_

  - [~] 9.2 Implement `internal/handler/` helper utilities
    - `writeJSON(w, status, v)` helper
    - `writeError(w, err)` helper — maps sentinel errors to HTTP status codes per the error table in the design; sanitized error message (no raw DB errors or stack traces)
    - `JWTClaims` struct with `RegisteredClaims`, `Role string`; `issueAdminToken(secret []byte, userID int64) (string, error)`
    - _Requirements: 21.2, 21.4_

  - [~] 9.3 Implement `internal/handler/game.go` — player-facing HTTP handlers
    - `POST /api/game/start` → `GameService.StartGame`; returns 201 `{"session_id": "…"}` on success; 422 on validation error; 503 on cache unavailable
    - `GET /api/game/next?session_id={id}` → `GameService.NextTwister`; returns 200 with twister text, hearts, score; 404 on session not found; 410 on no twisters remaining (win condition detected mid-request); 503 on cache error
    - `POST /api/game/attempt` → `GameService.SubmitAttempt`; returns 200 with result, hearts, score, game_over flag, and optional outcome/final_score/nickname on game end; 404/409/422/503 on errors
    - `GET /api/leaderboard` → `LeaderboardService.GetTop10`; returns 200 `{"entries":[…]}`; 503 on cache error
    - Each handler has a godoc comment per design commenting standard
    - _Requirements: 3.1–3.6, 7.1–7.7, 9.5, 10.6, 11.6, 12.3, 12.4_

  - [~] 9.4 Implement `internal/handler/admin.go` — admin HTTP handlers
    - `POST /api/admin/login` → `AdminService.Login`; returns 200 `{"token":"…"}`; 400/401/403 per requirements
    - `POST /api/admin/tongue-twisters` → `AdminService.CreateTongueTwister`; returns 201 with full record
    - `PATCH /api/admin/tongue-twisters/{id}` → `AdminService.UpdateTongueTwister`; returns 200 with full updated record
    - `GET /api/admin/tongue-twisters` → `AdminService.ListTongueTwisters` with `difficulty` and `active` query params; returns 200 `{"tongue_twisters":[…],"count":N}`
    - `POST /api/admin/tongue-twisters/{id}/deactivate` → `AdminService.DeactivateTongueTwister`; returns 200 with updated record
    - All handlers behind `JWTMiddleware` except login
    - _Requirements: 13.1–13.13, 14.1–14.5, 15.1–15.6, 16.1–16.8, 17.1–17.6_

  - [~] 9.5 Wire chi router and start HTTP server in `cmd/server/main.go`
    - Mount `RequestIDMiddleware` and `RequestLogMiddleware` globally
    - Mount player routes, leaderboard route, and admin routes (with `JWTMiddleware` on admin sub-router)
    - Serve embedded static files at `/` using `http.FileServer` over `go:embed`
    - Bind on `config.ServerPort`; emit `slog.Info("server started", "port", port)`
    - _Requirements: 1.3, 1.5, 18.1_

  - [~] 9.6 Add `//go:build integration` build tag to all integration test files
    - Every file in the integration test group (task 13.*) must open with `//go:build integration` so that `go test ./...` (without `-tags integration`) skips them in fast/unit runs
    - Add a comment on the build tag line explaining why: `// integration build tag ensures these tests are skipped in unit/CI-fast runs — they require Docker via testcontainers`
    - Update `make test-unit` and `make test-property` Makefile targets to confirm they do not pass `-tags integration`
    - _Requirements: none (test hygiene)_

- [~] 10. Checkpoint — backend complete
  - Run `go build ./...` and `go vet ./...`; ensure zero errors. Ask the user if questions arise.

- [ ] 11. Property-based tests for game logic
  - [~] 11.1 Write property test for `GameSession` serialization round-trip — Property 1
    - File: `internal/domain/session_prop_test.go`
    - **Property 1: GameSession serialization round-trip** — generate arbitrary `GameSession` values (all field combinations); marshal to JSON; unmarshal; assert field-by-field equality including every element of `ShownIDs`
    - Use `rapid` generators; min 100 iterations
    - Tag: `// Feature: lingua-speed, Property 1: GameSession serialization round-trip`
    - **Validates: Requirements 20.1, 20.2, 20.3**

  - [~] 11.2 Write property tests for game service logic — Properties 4, 5, 6, 7, 8
    - File: `internal/service/game_prop_test.go`
    - **Property 4: Nickname validation accepts exactly valid-length inputs** — for any Unicode string of rune length 1–32 → no error; length 0 or > 32 → error
    - **Property 5: Tongue-twister selection never repeats shown IDs** — for any session with non-empty `ShownIDs` pool and eligible twisters, selected ID must not be in `ShownIDs`; after selection ID must appear in `ShownIDs`
    - **Property 6: Cumulative score equals sum of attempt scores** — for any sequence of successful attempts, cumulative score = arithmetic sum of individual `scoring.Compute` results; no truncation or loss
    - **Property 7: Leaderboard retains highest score per nickname** — for any nickname and any pair of scores submitted in any order, leaderboard contains exactly one entry for that nickname with score = max(s1, s2)
    - **Property 8: Heart decrement is exactly one per failed attempt** — for any session with hearts ∈ [1,2,3], one failed attempt reduces hearts by exactly 1
    - Tag each with `// Feature: lingua-speed, Property N: <property text>`
    - **Validates: Requirements 3.1–3.3, 6.1, 6.2, 6.4, 8.3–8.5, 9.2, 12.6**

- [ ] 12. Frontend
  - [~] 12.1 Implement `static/index.html` — nickname + difficulty selection
    - Form with nickname input (maxlength=32, required), three difficulty radio buttons (easy/medium/hard), submit button
    - Client-side validation: non-empty nickname, difficulty selected; display inline error if either missing (do not submit form)
    - On successful `POST /api/game/start` response: store `session_id` in `localStorage` under key `linguaspeed_session_id`; redirect to `/game`
    - Plain CSS, no framework; accessibility: labels associated with inputs, focus styles
    - _Requirements: 3.1–3.3, 4.1–4.3, 5.5_

  - [~] 12.2 Implement `static/game.html` — active game UI
    - On load: read `linguaspeed_session_id` from `localStorage`; `GET /api/game/next` to fetch first twister
    - Display tongue-twister text with per-character `<span>` elements (classes `.char-pending`, `.char-correct`, `.char-incorrect`); update on every `input` event (client-side only, no server call during typing)
    - Display heart count (3 `<span>` positions, `.heart-filled` / `.heart-empty` classes); display cumulative score — both in a persistent area visible without scrolling
    - Input field maxlength=500; on submit: disable input, show loading indicator; POST to `/api/game/attempt` with `session_id`, `tongue_twister_id`, `typed_text`, `time_taken_ms` (from first keystroke)
    - On timeout (10 s): re-enable input, clear loader, show timeout error; preserve typed text
    - On failed attempt response: re-enable input, clear input, update heart display
    - On success (game continues): re-enable input, clear input, fetch next twister
    - On game-over response: redirect to `/result` passing `outcome`, `final_score`, `nickname` via `localStorage`
    - Display error from server on error responses; handle heart count outside 0–3 with error state
    - _Requirements: 7.1–7.2, 9.3, 9.6, 9.7, 18.1–18.9_

  - [~] 12.3 Implement `static/result.html` — win/loss results screen
    - Read `outcome`, `final_score`, `nickname` from `localStorage`; display outcome, score, nickname
    - Navigation link to `/leaderboard`
    - _Requirements: 10.6, 11.6, 18.9_

  - [~] 12.4 Implement `static/leaderboard.html` — public leaderboard
    - On load: `GET /api/leaderboard`; render table of rank, nickname (truncated to 20 chars), score
    - Handle 503 error gracefully with user-facing message
    - _Requirements: 12.3, 12.5, 12.7_

  - [~] 12.5 Implement `static/admin/login.html` — admin login page
    - Form with username (maxlength=50) and password (maxlength=100) inputs
    - On success: store JWT in `localStorage` under `linguaspeed_admin_jwt`; redirect to `/admin` within 1 second
    - On failure: display server error message without clearing username field
    - On load: if `linguaspeed_admin_jwt` already present, redirect to `/admin` immediately
    - _Requirements: 19.1, 19.2, 19.3_

  - [~] 12.6 Implement `static/admin/index.html` — tongue-twister management panel
    - On load: check `linguaspeed_admin_jwt` in `localStorage`; if absent redirect to `/admin/login`
    - On any server 401 response: clear JWT from `localStorage`, redirect to `/admin/login`
    - Form to create new twister: text input (maxlength=500), difficulty select (easy/medium/hard), submit → `POST /api/admin/tongue-twisters` with `Authorization: Bearer <jwt>`
    - Table listing all twisters (up to 200 shown): columns id, text, difficulty, active status; paginate client-side if > 200
    - Edit form per row: text (maxlength=500) and/or difficulty → `PATCH /api/admin/tongue-twisters/{id}`
    - Deactivate button shown only for active twisters → `POST /api/admin/tongue-twisters/{id}/deactivate`
    - _Requirements: 19.4–19.9_

- [ ] 13. Integration tests
  - [~] 13.1 Write integration test — full game flow (win path)
    - Spin up test PostgreSQL and Redis via `testcontainers-go`; run migrations + seed
    - Insert at least 2 active tongue-twisters for difficulty `easy`
    - `POST /api/game/start` → assert `session_id` returned; verify session in Redis
    - Loop: `GET /api/game/next` → `POST /api/game/attempt` (correct text) until all twisters exhausted
    - Assert final attempt response has `game_over: true`, `outcome: "win"`, non-negative `final_score`
    - Assert `game_records` row exists in DB with correct nickname, score, difficulty
    - Assert `attempt_details` rows exist (one per twister)
    - Assert leaderboard contains the player's entry with correct score
    - Assert session no longer exists in Redis
    - _Requirements: 6.3, 8.3–8.5, 10.1–10.6, 12.2_

  - [~] 13.2 Write integration test — full game flow (loss path)
    - Same setup; submit 3 failed attempts for a single twister
    - Assert final attempt response has `game_over: true`, `outcome: "loss"`, `hearts: 0`
    - Assert `game_records` and `attempt_details` written to DB; leaderboard updated; session deleted from Redis
    - _Requirements: 9.3, 11.1–11.6, 12.2_

  - [~] 13.3 Write integration test — leaderboard highest-score retention
    - Two game sessions with the same nickname; first ends with score S1, second ends with score S2 (S2 < S1)
    - Assert leaderboard contains exactly one entry for that nickname with score S1
    - _Requirements: 12.6_

  - [~] 13.4 Write integration test — admin CRUD for tongue-twisters
    - Login with seed admin credentials; assert JWT returned
    - Create twister → assert 201 and record in DB
    - Update twister text → assert 200 and `updated_at` changed
    - List twisters with `?difficulty=easy` filter → assert only easy twisters returned
    - Deactivate twister → assert 200 and `active: false` in DB
    - Attempt to deactivate again → assert 409
    - _Requirements: 13.1–13.13, 14.1–14.5, 15.1–15.6, 16.1–16.8, 17.1–17.6_

  - [~] 13.5 Write integration test — migration correctness
    - Apply migrations to a clean database
    - Assert all four tables exist: `tongue_twisters`, `users`, `game_records`, `attempt_details`
    - Assert `user_role` enum rejects values other than `admin` and `moderator`
    - Assert `tongue_twisters.difficulty` rejects values other than `easy`, `medium`, `hard`
    - _Requirements: 2.1–2.7_

- [~] 14. Final checkpoint — full build and test suite
  - Run `go build ./...`; run `go test ./...`; ensure all tests pass. Ask the user if questions arise.

- [ ] 15. GitHub Actions CI/CD
  - [~] 15.1 Create `.github/workflows/ci.yml` — continuous integration workflow
    - Trigger: `push` and `pull_request` targeting branches `develop` and `main` (git-flow: CI runs on every branch that targets integration or production)
    - Jobs — run in this order:
      1. **lint**: `go vet ./...` on `ubuntu-latest`, Go version pinned to `1.22`
      2. **test-unit**: `make test-unit` — no external services needed; runs fast
      3. **test-property**: `make test-property` — no external services needed; runs after lint
      4. **test-integration**: `make test-integration` — requires Docker; uses `services:` block with `postgres:16-alpine` and `redis:7-alpine`; sets env vars `DATABASE_URL`, `REDIS_ADDR`, `JWT_SECRET`, `ADMIN_USERNAME`, `ADMIN_PASSWORD` from GitHub Actions secrets or hardcoded test values (not production secrets)
    - `test-unit` and `test-property` run in parallel after `lint`; `test-integration` runs after both pass
    - Cache Go module downloads between runs using `actions/cache` on `~/.cache/go`
    - All jobs use `actions/checkout@v4` and `actions/setup-go@v5` with `go-version-file: go.mod`
    - Add a workflow status badge comment at the top of the YAML file: `# Add this badge to README.md: ![CI](https://github.com/cmerin0/LinguaSpeed/actions/workflows/ci.yml/badge.svg)`
    - _Requirements: 1.1, 1.2 (validates build and test gates before merge)_

  - [~] 15.2 Create `.github/workflows/pr-review.yml` — pull request review gate
    - Trigger: `pull_request` targeting `develop` or `main`, on events `opened`, `synchronize`, `reopened`
    - Single job: **require-review**
      - Uses `actions/github-script@v7` to post a PR comment reminding the reviewer to check: (1) all tasks completed, (2) tests pass, (3) no `.env` files committed, (4) no raw secrets in code
      - The comment template must include a checklist in Markdown so the reviewer can tick items
    - This workflow does NOT block the merge — it only posts the reminder comment; branch protection rules on GitHub control actual merge requirements
    - Note in a comment at the top: `# This workflow posts a review checklist comment on every PR. Configure branch protection rules in GitHub Settings → Branches to enforce required reviews before merge.`
    - _Requirements: none (process enforcement)_

  - [~] 15.3 Create `.github/PULL_REQUEST_TEMPLATE.md` — PR description template
    - Sections: **Summary** (what changed), **Type of change** (checkboxes: bug fix, new feature, chore, docs), **Testing** (checkboxes: unit tests pass, property tests pass, integration tests pass, manually tested), **Git-flow checklist** (checkboxes: branched from `develop`, targets `develop` (or `main` for release), no direct commits to `main`), **Notes for reviewer**
    - Keep it concise — the template is a prompt, not a form to fill in exhaustively
    - _Requirements: none (process)_

  - [~] 15.4 Add `.github/workflows/README.md` — brief CI/CD documentation
    - Explain the two workflows and when they run
    - Explain the git-flow branch model used: `main` (production-ready), `develop` (integration), `feature/*` (individual features branched from develop, merged back to develop), `release/*` (branched from develop to main), `hotfix/*` (branched from main, merged to both main and develop)
    - Explain what secrets need to be set in GitHub repo settings for CI to work (none required for unit/property tests; testcontainers handles its own Docker — no registry secrets needed for integration tests)
    - _Requirements: none (documentation)_

---

## Notes

- Tasks marked with `*` are optional and can be skipped for a faster MVP; all other tasks are required
- Each task references specific requirements for traceability
- Checkpoints (tasks 7, 10, 14) ensure incremental validation before moving to the next layer
- Property tests use `pgregory.net/rapid` with at minimum 100 iterations per property
- All exported Go symbols must carry godoc comments; all files must open with a package-level comment
- The `// why` commenting obligation (NX/GETEX/ZADD GT/bcrypt cost/WithValidMethods) is part of the implementation requirement, not optional
- Integration tests require Docker to be available in the test environment (via `testcontainers-go`)

## Task Dependency Graph

```json
{
  "waves": [
    { "id": 0, "tasks": ["1.1", "1.2"] },
    { "id": 1, "tasks": ["1.3", "1.5", "2.1"] },
    { "id": 2, "tasks": ["2.2", "2.3", "3.1", "3.2"] },
    { "id": 3, "tasks": ["3.3", "4.1"] },
    { "id": 4, "tasks": ["3.4", "5.1"] },
    { "id": 5, "tasks": ["5.2", "6.1", "6.2"] },
    { "id": 6, "tasks": ["6.3", "6.4", "6.5"] },
    { "id": 7, "tasks": ["8.1", "8.2", "8.3"] },
    { "id": 8, "tasks": ["9.1", "9.2"] },
    { "id": 9, "tasks": ["9.3", "9.4"] },
    { "id": 10, "tasks": ["9.5", "9.6"] },
    { "id": 11, "tasks": ["11.1", "11.2", "12.1"] },
    { "id": 12, "tasks": ["12.2", "12.3", "12.4", "12.5"] },
    { "id": 13, "tasks": ["12.6", "13.1", "13.2", "13.3", "13.4", "13.5"] },
    { "id": 14, "tasks": ["15.1", "15.2", "15.3", "15.4"] }
  ]
}
```
