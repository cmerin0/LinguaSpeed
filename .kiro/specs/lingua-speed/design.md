# Design Document

## LinguaSpeed

---

## Overview

LinguaSpeed is a web-based typing game where players race against a timer to type Spanish tongue-twisters as accurately and quickly as possible. The architecture is intentionally simple: a single Go HTTP server sits in front of a PostgreSQL database (permanent storage) and a Redis store (live session state and leaderboard). A plain HTML/CSS/JS frontend is served as static files from the Go server itself. All services are orchestrated by Docker Compose.

### Design Principles

- **Thin frontend, fat server**: All game logic (scoring, life deduction, session advancement) lives in the Go backend. The frontend renders state returned by the API.
- **Redis is ephemeral**: No Redis data is the source of truth. Game records are persisted to PostgreSQL at game end. Redis loss is survivable (in-flight sessions expire; leaderboard can be rebuilt from `game_records`).
- **Single binary**: The Go server embeds all static assets and SQL migrations at compile time via `go:embed`, producing a self-contained image.
- **Explicit over implicit**: Every configuration value comes from environment variables. Missing values at startup cause an immediate, descriptive exit.

---

## Architecture

```mermaid
graph TD
    Browser["Browser\n(HTML / CSS / JS)"]
    Server["Go HTTP Server\n(chi router)"]
    PG["PostgreSQL\n(permanent data)"]
    Redis["Redis\n(sessions + leaderboard)"]

    Browser -- "HTTP (JSON)" --> Server
    Server -- "pgx/v5" --> PG
    Server -- "go-redis/v9" --> Redis
    Server -- "Static files\n(go:embed)" --> Browser
```

### Runtime Flow

1. Docker Compose starts PostgreSQL and Redis first, with health checks.
2. The Go server container waits for both health checks before starting.
3. On startup the server applies pending SQL migrations via `golang-migrate`, then binds the HTTP listener.
4. The frontend is served as static files from the same Go process.
5. All API calls are JSON over HTTP; there is no WebSocket or SSE.

---

## Components and Interfaces

### Package Structure

```
cmd/
  server/
    main.go           -- entry point: config, migrations, dependency wiring, server start
internal/
  config/
    config.go         -- reads and validates environment variables; exits on missing
  db/
    db.go             -- pgx/v5 connection pool factory
    migrations/       -- embedded SQL migration files (go:embed)
  cache/
    cache.go          -- go-redis/v9 client factory
  domain/
    game.go           -- GameSession, AttemptDetail, GameRecord value types
    tongue_twister.go -- TongueTwister, Difficulty value types
    user.go           -- User, Role value types
  repository/
    tongue_twister.go -- DB queries for tongue_twisters table
    game_record.go    -- DB queries for game_records + attempt_details
    user.go           -- DB queries for users table
    session.go        -- Redis read/write for GameSession
    leaderboard.go    -- Redis sorted set operations
  service/
    game.go           -- start game, fetch next, submit attempt, end game
    admin.go          -- login, CRUD for tongue twisters
    leaderboard.go    -- leaderboard retrieval
  handler/
    game.go           -- HTTP handlers for player-facing endpoints
    admin.go          -- HTTP handlers for admin endpoints
    leaderboard.go    -- HTTP handler for leaderboard endpoint
    middleware.go     -- JWT auth middleware, request ID, logging
  scoring/
    scoring.go        -- pure scoring formula; no external dependencies
static/               -- embedded HTML, CSS, JS (go:embed)
```

### Layer Responsibilities

| Layer | Responsibility |
|---|---|
| `handler` | HTTP binding: decode request, call service, encode response. No business logic. |
| `service` | Orchestrates repositories and domain logic. Transaction boundaries live here. |
| `repository` | Single-table or single-data-store queries. Returns domain types. |
| `domain` | Plain Go structs with no methods beyond trivial helpers. |
| `scoring` | Pure function with no I/O: computes score from inputs. |

### Key External Libraries

| Library | Purpose |
|---|---|
| `github.com/go-chi/chi/v5` | HTTP router |
| `github.com/jackc/pgx/v5` | PostgreSQL driver and connection pool |
| `github.com/redis/go-redis/v9` | Redis client |
| `github.com/golang-migrate/migrate/v4` | SQL migration runner (embedded files) |
| `github.com/golang-jwt/jwt/v5` | JWT signing and verification |
| `golang.org/x/crypto/bcrypt` | Password hashing (cost ≥ 12) |
| `github.com/google/uuid` | Session ID generation |

---

## Code Commenting Standard

All Go code in this project follows a consistent commenting convention. Implementors must adhere to the rules below so that `go doc`, IDE hover cards, and code reviews stay useful without redundancy.

### Rules

1. **Every exported symbol must have a godoc comment.** Functions, types, constants, and variables that start with a capital letter require a comment that begins with the symbol name itself (Go convention). This applies everywhere: `domain`, `service`, `repository`, `handler`, `scoring`.

2. **Non-obvious logic blocks must have a `// why` comment — not a `// what` comment.** The code already says *what* it does. The comment must explain *why* that approach was chosen. Mandatory sites:
   - `ZADD … GT` — explain the "only update if greater" semantics for leaderboard tie-breaking.
   - `SET … NX EX` — explain the atomic "set if not exists" concurrency guard for session creation.
   - `GETEX … EX 7200` — explain the sliding TTL refresh strategy.
   - bcrypt cost constant — explain why cost 12 is the minimum acceptable value.
   - `WithValidMethods([]string{"HS256"})` — explain algorithm pinning and why omitting it is a security hole.

3. **Each file must open with a package-level comment** (one line) stating the file's single responsibility. Example: `// Package scoring implements the pure scoring formula for a single tongue-twister attempt.`

4. **Middleware functions require a comment block** at their declaration explaining:
   - What they check, and in what order.
   - What HTTP status code they return on each failure mode.
   - What they inject into the request context on success.

5. **No commented-out code in committed files.** If code must be deferred, replace it with a `// TODO(<ticket>): <reason>` line referencing a ticket or issue.

### Examples

**Well-commented handler function:**

```go
// HandleAttempt processes a player's typed attempt for the current tongue-twister.
// It delegates scoring and state mutation to GameService and returns the updated
// session state. On session expiry it returns 404; on a finished session, 409.
func (h *GameHandler) HandleAttempt(w http.ResponseWriter, r *http.Request) {
    var req AttemptRequest
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        writeError(w, domain.ErrValidation)
        return
    }
    result, err := h.svc.SubmitAttempt(r.Context(), req.SessionID, req.TongueTwisterID, req.TypedText, req.TimeTakenMS)
    if err != nil {
        writeError(w, err)
        return
    }
    writeJSON(w, http.StatusOK, result)
}
```

**Well-commented repository function:**

```go
// NextTwister returns a random active tongue-twister for the given difficulty
// that is not present in excludeIDs. Returns domain.ErrNotFound when no
// eligible twisters remain, which the caller maps to the win condition.
func (r *TongueTwisterRepo) NextTwister(ctx context.Context, difficulty string, excludeIDs []int64) (*domain.TongueTwister, error) {
    // ANY(excludeIDs) is a PostgreSQL array operator; it avoids a dynamic IN clause
    // and works correctly even when excludeIDs is empty (ANY of empty → always false).
    const q = `SELECT id, text, difficulty FROM tongue_twisters
               WHERE difficulty = $1 AND active = TRUE AND id <> ALL($2)
               ORDER BY random() LIMIT 1`
    row := r.pool.QueryRow(ctx, q, difficulty, excludeIDs)
    // ... scan and return
}
```

---

## Data Models

### PostgreSQL Schema

All migrations live under `internal/db/migrations/` and are embedded into the binary. Filenames follow the `golang-migrate` convention: `000001_create_tongue_twisters.up.sql` / `000001_create_tongue_twisters.down.sql`.

#### `tongue_twisters`

```sql
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

CREATE INDEX idx_tt_difficulty_active ON tongue_twisters (difficulty, active);
```

The composite index supports the most common query: `WHERE difficulty = $1 AND active = TRUE`.

#### `users`

```sql
CREATE TYPE user_role AS ENUM ('admin', 'moderator');

CREATE TABLE users (
    id              BIGSERIAL       PRIMARY KEY,
    username        VARCHAR(64)     NOT NULL UNIQUE,
    hashed_password VARCHAR(72)     NOT NULL,   -- bcrypt max effective input length
    role            user_role       NOT NULL,
    created_at      TIMESTAMPTZ     NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ     NOT NULL DEFAULT NOW()
);
```

The `user_role` enum enforces the constraint at the database level (Requirement 2.7).

#### `game_records`

```sql
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
```

#### `attempt_details`

```sql
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
```

### Redis Data Structures

#### Game Session

Key: `session:{session_id}` (e.g., `session:550e8400-e29b-41d4-a716-446655440000`)

Stored as a JSON string via `SET` with `EX` for TTL. The TTL is refreshed to 7200 seconds on every read or write (`GETEX key EX 7200` on reads; `SET key value EX 7200` on writes). This implements the "2 hours of inactivity" requirement.

```json
{
  "session_id":    "550e8400-e29b-41d4-a716-446655440000",
  "nickname":      "ElFenómeno",
  "difficulty":    "medium",
  "hearts":        3,
  "score":         0,
  "shown_ids":     [],
  "started_at":    "2024-01-15T14:30:00Z",
  "status":        "active"
}
```

| Field | Type | Notes |
|---|---|---|
| `session_id` | string (UUID) | Matches the Redis key |
| `nickname` | string | 1–32 characters |
| `difficulty` | string | `easy` / `medium` / `hard` |
| `hearts` | integer | 0–3 |
| `score` | integer | ≥ 0 |
| `shown_ids` | []int64 | IDs of tongue twisters already shown |
| `started_at` | string (RFC3339) | Set at session creation |
| `status` | string | `active` / `ended_win` / `ended_loss` |

**Serialization**: The Go struct is marshalled to JSON using `encoding/json`. Field names are snake_case (struct tags). Integer slices marshal to JSON arrays. All fields are always present (no `omitempty`) so that deserialization is deterministic and complete (Requirement 20).

#### Leaderboard

Key: `leaderboard:all`

Stored as a Redis sorted set. Member: player's Nickname. Score: final game Score (integer stored as float64 by Redis).

Upsert strategy (Requirement 12.6 — retain highest score per nickname):
```
ZADD leaderboard:all GT <score> <nickname>
```
`GT` flag: only update the score if the new score is greater than the existing score. If the nickname is not present, it is always added (because there is nothing to compare against).

Tie-breaking for equal scores (Requirement 12.8 — earlier submission ranks higher): Redis sorted sets are stable for equal scores — members added first appear first in `ZREVRANGE` among equal-score members. Since scores are integers and `ZADD GT` only replaces when strictly greater, an earlier equal submission is never overwritten. The rank order among tied entries is therefore insertion order, which is the desired behaviour.

Retrieval:
```
ZREVRANGE leaderboard:all 0 9 WITHSCORES
```
Returns top 10 members with scores in descending order. Rank is computed as the 0-based index + 1.

---

## API Endpoints

All request and response bodies are JSON. All responses include a `Content-Type: application/json` header. Error responses have the shape `{"error": "<message>"}`. Success responses are described per endpoint.

The server also serves static frontend files from the embedded `static/` directory at the root path.

### Authentication

Protected endpoints require the header:
```
Authorization: Bearer <jwt>
```
The JWT must be signed with HS256 using the `JWT_SECRET` environment variable, must not be expired, and must carry a `role` claim of `admin`.

### Player-Facing Endpoints

#### `POST /api/game/start`

Start a new game session.

**Request:**
```json
{ "nickname": "ElFenómeno", "difficulty": "medium" }
```

**Response 201:**
```json
{ "session_id": "550e8400-e29b-41d4-a716-446655440000" }
```

**Errors:** 422 (validation), 503 (cache unavailable)

---

#### `GET /api/game/next?session_id={id}`

Fetch the next tongue-twister for the session.

**Response 200:**
```json
{
  "tongue_twister_id": 42,
  "text":             "Tres tristes tigres tragaban trigo",
  "hearts":           3,
  "score":            0
}
```

**Errors:** 404 (session not found/expired), 410 (no eligible tongue-twisters remain → triggers win condition), 503 (cache unavailable)

---

#### `POST /api/game/attempt`

Submit an attempt for the current tongue-twister.

**Request:**
```json
{
  "session_id":       "550e8400-e29b-41d4-a716-446655440000",
  "tongue_twister_id": 42,
  "typed_text":       "Tres tristes tigres tragaban trigo",
  "time_taken_ms":    4200
}
```

**Response 200 (failed attempt):**
```json
{
  "result":        "failed",
  "hearts":        2,
  "score":         0,
  "game_over":     false
}
```

**Response 200 (success, game continues):**
```json
{
  "result":        "success",
  "hearts":        2,
  "score":         650,
  "game_over":     false
}
```

**Response 200 (game over — loss):**
```json
{
  "result":        "failed",
  "hearts":        0,
  "score":         650,
  "game_over":     true,
  "outcome":       "loss",
  "final_score":   650,
  "nickname":      "ElFenómeno"
}
```

**Response 200 (game over — win, all twisters exhausted):**
```json
{
  "result":        "success",
  "hearts":        2,
  "score":         2800,
  "game_over":     true,
  "outcome":       "win",
  "final_score":   2800,
  "nickname":      "ElFenómeno"
}
```

**Errors:** 404 (session not found), 409 (session already ended), 422 (validation), 503 (cache unavailable)

---

#### `GET /api/leaderboard`

Retrieve the top 10 leaderboard entries. Public endpoint, no auth required.

**Response 200:**
```json
{
  "entries": [
    { "rank": 1, "nickname": "ElFenómeno", "score": 4200 },
    { "rank": 2, "nickname": "LaVelocista",  "score": 3950 }
  ]
}
```

**Errors:** 503 (cache unavailable)

---

### Admin Endpoints

All admin endpoints require a valid JWT with `role: admin`.

#### `POST /api/admin/login`

**Request:**
```json
{ "username": "admin", "password": "s3cr3t" }
```

**Response 200:**
```json
{ "token": "<jwt>" }
```

**Errors:** 400 (missing/oversized fields), 401 (wrong credentials), 403 (non-admin role)

---

#### `POST /api/admin/tongue-twisters`

Create a tongue-twister.

**Request:**
```json
{ "text": "Tres tristes tigres...", "difficulty": "hard" }
```

**Response 201:**
```json
{
  "id": 42,
  "text": "Tres tristes tigres...",
  "difficulty": "hard",
  "active": true,
  "created_at": "2024-01-15T14:30:00Z",
  "updated_at": "2024-01-15T14:30:00Z"
}
```

**Errors:** 401 (no/invalid JWT), 422 (validation)

---

#### `PATCH /api/admin/tongue-twisters/{id}`

Update a tongue-twister. At least one field required.

**Request (partial update):**
```json
{ "difficulty": "easy" }
```

**Response 200:** Full updated record (same shape as create response).

**Errors:** 401, 404, 422

---

#### `GET /api/admin/tongue-twisters`

List tongue-twisters with optional filters.

**Query params:** `?difficulty=easy&active=true`

**Response 200:**
```json
{
  "tongue_twisters": [ { "id": 1, "text": "...", "difficulty": "easy", "active": true, "created_at": "...", "updated_at": "..." } ],
  "count": 1
}
```

**Errors:** 401, 422 (invalid filter values)

---

#### `POST /api/admin/tongue-twisters/{id}/deactivate`

Deactivate a tongue-twister.

**Response 200:** Full updated record with `"active": false`.

**Errors:** 401, 404, 409 (already inactive)

---

## Session Management

### Session ID Generation

Session IDs are version 4 UUIDs generated by the server using `github.com/google/uuid`. They are generated at the start of `POST /api/game/start`, before any Redis write, ensuring they are globally unique with negligible collision probability.

### Redis Key Naming

| Purpose | Key pattern | TTL strategy |
|---|---|---|
| Game session | `session:{uuid}` | Sliding 2-hour TTL, reset on every read (`GETEX … EX 7200`) and write (`SET … EX 7200`) |
| Leaderboard | `leaderboard:all` | No TTL (persistent) |

### Concurrent Session Creation (Requirement 5.7)

To prevent two concurrent requests from writing two sessions with the same key, the server uses `SET key value EX 7200 NX`. `NX` (set if not exists) makes the operation atomic: only one writer wins. In practice, UUID v4 collisions are not a realistic concern, but the `NX` flag ensures correctness under all conditions.

---

## JWT Authentication Flow

A JSON Web Token (JWT) is a signed, self-contained credential: the server signs a small JSON payload with a secret key, and any subsequent request can be verified without a database lookup. We use JWTs here because admin sessions are stateless and short-lived (8 hours). The one rule that matters most: **always pin the allowed signing algorithm**. Accepting `"alg": "none"` or RS256 when you expect HS256 is a known attack vector — `WithValidMethods` closes it.

```mermaid
sequenceDiagram
    participant Admin
    participant Server
    participant DB

    Admin->>Server: POST /api/admin/login {username, password}
    Server->>DB: SELECT id, hashed_password, role WHERE username = $1
    DB-->>Server: user row
    Server->>Server: bcrypt.CompareHashAndPassword(hash, password)
    alt credentials valid AND role == admin
        Server->>Server: jwt.NewWithClaims(HS256, {sub, role, exp: now+8h})
        Server-->>Admin: 200 {token}
    else invalid credentials or non-admin role
        Server-->>Admin: 401 / 403
    end

    Admin->>Server: GET /api/admin/... Authorization: Bearer <jwt>
    Server->>Server: jwtMiddleware — verify signature, expiry, role
    alt valid
        Server->>Server: proceed to handler
    else invalid / expired / wrong role
        Server-->>Admin: 401 / 403
    end
```

### Middleware

```go
// jwtMiddleware authenticates admin API requests.
//
// Check order:
//  1. Authorization header present and has "Bearer <token>" shape → 401 if missing.
//  2. Token parses and signature is valid (HS256 only) → 401 if invalid or expired.
//  3. "role" claim equals "admin" → 403 if a valid non-admin token is presented.
//
// On success, the parsed claims are stored in the request context for handlers.
func jwtMiddleware(secret []byte) func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            authHeader := r.Header.Get("Authorization")
            if !strings.HasPrefix(authHeader, "Bearer ") {
                writeError(w, domain.ErrUnauthorized) // missing or malformed header
                return
            }
            rawToken := strings.TrimPrefix(authHeader, "Bearer ")

            var claims JWTClaims
            // WithValidMethods pins the algorithm to HS256.
            // Without this, an attacker can craft a token with "alg":"none" and bypass verification.
            token, err := jwt.ParseWithClaims(rawToken, &claims,
                func(t *jwt.Token) (interface{}, error) { return secret, nil },
                jwt.WithValidMethods([]string{"HS256"}),
            )
            if err != nil || !token.Valid {
                writeError(w, domain.ErrUnauthorized) // invalid signature or expired (checked by library)
                return
            }

            if claims.Role != "admin" {
                writeError(w, domain.ErrForbidden) // valid token, but insufficient role
                return
            }

            next.ServeHTTP(w, r.WithContext(withClaims(r.Context(), &claims)))
        })
    }
}
```

### Token Issuance (Login Handler)

```go
// issueAdminToken mints a signed JWT for a verified admin user.
func issueAdminToken(secret []byte, userID int64) (string, error) {
    claims := JWTClaims{
        RegisteredClaims: jwt.RegisteredClaims{
            Subject: strconv.FormatInt(userID, 10),
            // 8 hours balances security (short enough to limit damage if leaked) with
            // usability (an admin working a full shift won't need to re-login mid-session).
            ExpiresAt: jwt.NewNumericDate(time.Now().Add(8 * time.Hour)),
            IssuedAt:  jwt.NewNumericDate(time.Now()),
        },
        // Role is embedded in the token so the middleware needs no DB round-trip to authorise.
        Role: "admin",
    }
    // HS256 with a shared secret is appropriate here: there is only one issuer (this server)
    // and only one verifier (this server). Asymmetric keys (RS256) add complexity with no benefit.
    return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(secret)
}
```

### JWT Claims Structure

```json
{
  "sub": "42",
  "role": "admin",
  "iat": 1705329000,
  "exp": 1705357800
}
```

---

## Scoring Formula Implementation

The formula is defined in `internal/scoring/scoring.go` as a pure function with no I/O:

```go
// DefaultBasePoints and DefaultPenalty are the configurable constants.
const (
    DefaultBasePoints = 1000
    DefaultPenalty    = 200
)

// Compute returns the score for a single successful tongue-twister attempt.
// timeMS is the elapsed time in milliseconds from first keystroke to successful submit.
// failedAttempts is the count of failed submissions for this tongue-twister.
// basePoints and penalty are configurable constants (use defaults if 0).
func Compute(timeMS int64, failedAttempts int, basePoints, penalty int) int {
    if basePoints == 0 {
        basePoints = DefaultBasePoints
    }
    if penalty == 0 {
        penalty = DefaultPenalty
    }
    score := basePoints - int(timeMS/100) - (failedAttempts * penalty)
    if score < 0 {
        return 0
    }
    return score
}
```

This is called by `service/game.go` after a successful attempt is recorded. The result is added to the session's cumulative score in Redis.

---

## Migrations and Startup Sequence

```mermaid
sequenceDiagram
    participant Compose
    participant PG
    participant Redis
    participant Server

    Compose->>PG: start (healthcheck: pg_isready)
    Compose->>Redis: start (healthcheck: redis-cli ping)
    Compose->>Server: start (depends_on: pg healthy, redis healthy)
    Server->>Server: config.Load() — exit non-zero if any env var missing
    Server->>PG: open connection pool (pgx/v5)
    Server->>Server: golang-migrate: apply pending migrations
    alt migration fails
        Server-->>Server: log error + exit(1)
    end
    Server->>Redis: open client (go-redis/v9, ping)
    Server->>Server: run seed script logic (create admin if absent)
    Server->>Server: bind HTTP listener on $SERVER_PORT (default 3000)
    Server-->>Compose: accepting connections
```

### Environment Variables

| Variable | Required | Default | Description |
|---|---|---|---|
| `DATABASE_URL` | Yes | — | PostgreSQL DSN (`postgres://user:pass@host:5432/db`) |
| `REDIS_ADDR` | Yes | — | Redis address (`redis:6379`) |
| `JWT_SECRET` | Yes | — | HS256 signing secret (≥ 32 bytes recommended) |
| `SERVER_PORT` | No | `3000` | Host-exposed HTTP port |
| `ADMIN_USERNAME` | Yes (seed) | — | Initial admin username |
| `ADMIN_PASSWORD` | Yes (seed) | — | Initial admin password |
| `BASE_POINTS` | No | `1000` | Scoring formula base points |
| `PENALTY` | No | `200` | Scoring formula penalty per failed attempt |

---

## Docker Compose Service Definitions

```yaml
# Load all secrets from a local .env file — never commit .env to source control.
# Copy .env.example to .env and fill in the values before running.
env_file: .env

services:
  db:
    image: postgres:16-alpine
    restart: unless-stopped   # recover from crashes automatically; operator must fix bad config manually
    environment:
      POSTGRES_USER: linguaspeed
      POSTGRES_PASSWORD: ${DB_PASSWORD}
      POSTGRES_DB: linguaspeed
      # Explicit UTF-8 encoding matters: tongue-twisters contain Spanish characters (tildes, ñ, ü).
      POSTGRES_INITDB_ARGS: "--encoding=UTF8"
    volumes:
      - db_data:/var/lib/postgresql/data
    # No `ports:` — the database is only reachable from inside linguaspeed_net.
    # Exposing 5432 to the host would allow any local process to connect directly.
    networks:
      - linguaspeed_net
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U linguaspeed"]
      interval: 5s
      timeout: 5s
      retries: 5
    # Production note: add deploy.resources.limits to cap CPU and memory, e.g.:
    #   deploy:
    #     resources:
    #       limits:
    #         cpus: "1.0"
    #         memory: 512M

  cache:
    image: redis:7-alpine
    restart: unless-stopped
    # Disable all Redis persistence: no RDB snapshots, no AOF journal.
    # Redis is ephemeral by design — in-flight sessions expire and the leaderboard
    # can be rebuilt from game_records. Persistence would waste I/O for no benefit.
    command: redis-server --save "" --appendonly no
    # No `ports:` — Redis is only reachable from inside linguaspeed_net.
    networks:
      - linguaspeed_net
    healthcheck:
      test: ["CMD", "redis-cli", "ping"]
      interval: 5s
      timeout: 5s
      retries: 5
    # Production note: add deploy.resources.limits as above.

  server:
    build: .
    # No restart policy — let it fail loudly on misconfiguration so the operator
    # sees the error immediately rather than entering a restart loop.
    ports:
      - "${SERVER_PORT:-3000}:3000"   # only the server is host-facing
    environment:
      DATABASE_URL: postgres://linguaspeed:${DB_PASSWORD}@db:5432/linguaspeed
      REDIS_ADDR: cache:6379
      JWT_SECRET: ${JWT_SECRET}
      SERVER_PORT: 3000
      ADMIN_USERNAME: ${ADMIN_USERNAME}
      ADMIN_PASSWORD: ${ADMIN_PASSWORD}
    networks:
      - linguaspeed_net
    depends_on:
      db:
        condition: service_healthy
      cache:
        condition: service_healthy

networks:
  linguaspeed_net:
    driver: bridge   # isolated bridge; services on this network cannot reach other Compose stacks

volumes:
  db_data:
```

The `server` container only starts after both `db` and `cache` pass their health checks, satisfying Requirement 1.2. The database and cache have no host-facing ports; all inter-service traffic stays on the `linguaspeed_net` bridge network.

### `.env.example`

Developers must copy this file to `.env` and fill in every value before running `docker compose up`:

| Variable | Required | Example value | Notes |
|---|---|---|---|
| `DB_PASSWORD` | Yes | `changeme` | PostgreSQL password for the `linguaspeed` user |
| `JWT_SECRET` | Yes | `<32+ random bytes>` | HS256 signing secret — generate with `openssl rand -hex 32` |
| `ADMIN_USERNAME` | Yes | `admin` | Seed admin account username |
| `ADMIN_PASSWORD` | Yes | `changeme` | Seed admin account password — change before any exposure |
| `SERVER_PORT` | No | `3000` | Host port the server listens on; defaults to 3000 |
| `BASE_POINTS` | No | `1000` | Scoring formula base points |
| `PENALTY` | No | `200` | Scoring formula penalty per failed attempt |

---

## Frontend Page Structure

The frontend is served as static files embedded in the Go binary. No JavaScript framework is used.

### Pages

| File | Route | Description |
|---|---|---|
| `static/index.html` | `/` | Nickname + difficulty selection form |
| `static/game.html` | `/game` | Active game UI |
| `static/result.html` | `/result` | Win/loss screen with leaderboard link |
| `static/leaderboard.html` | `/leaderboard` | Public leaderboard display |
| `static/admin/login.html` | `/admin/login` | Admin login page |
| `static/admin/index.html` | `/admin` | Tongue-twister management panel |

### Session Persistence

The `session_id` returned from `POST /api/game/start` is stored in `localStorage` under the key `linguaspeed_session_id`. The game page reads this key on load and uses it for all subsequent API calls.

### Character Mismatch Highlighting

The game page compares the player's current input character-by-character against the tongue-twister text on every `input` event. Characters are wrapped in `<span>` elements with CSS classes:

- `.char-pending` — not yet typed
- `.char-correct` — typed correctly
- `.char-incorrect` — typed incorrectly

This is pure client-side DOM manipulation; no server call is made during typing.

### Heart Display

Three `<span>` elements are rendered for the heart positions. Each position receives class `.heart-filled` if `hearts > positionIndex` and `.heart-empty` otherwise. On each server response the display is updated from the returned `hearts` value.

---

## Correctness Properties

*A property is a characteristic or behavior that should hold true across all valid executions of a system — essentially, a formal statement about what the system should do. Properties serve as the bridge between human-readable specifications and machine-verifiable correctness guarantees.*

### Property 1: Game Session Serialization Round-Trip

*For any* valid `GameSession` struct — with any combination of nickname (1–32 Unicode characters), difficulty (easy/medium/hard), hearts (0–3), score (≥ 0), zero or more shown IDs, any started_at timestamp, and any status — marshalling it to JSON and then unmarshalling the result must produce a `GameSession` struct with field values identical to the original, including every element of the `shown_ids` slice, with no fields missing or defaulting to zero values.

**Validates: Requirements 20.1, 20.2, 20.3**

### Property 2: Scoring Formula Non-Negativity

*For any* non-negative `time_taken_ms` and any non-negative `failed_attempts`, the `Compute` function must return a value ≥ 0.

**Validates: Requirements 8.2**

### Property 3: Scoring Formula Degrades With Failures and Time

*For any* valid inputs, holding `time_taken_ms` fixed and increasing `failed_attempts` by 1 must not increase the score, and holding `failed_attempts` fixed and increasing `time_taken_ms` must not increase the score. Both dimensions of degradation must hold simultaneously: more failures and more time must never produce a higher score than fewer failures and less time.

**Validates: Requirements 8.1, 8.2**

### Property 4: Nickname Validation Accepts Exactly Valid-Length Inputs

*For any* string whose rune length is between 1 and 32 inclusive, the nickname validation function must return no error; and for any string whose rune length is 0 or exceeds 32, the nickname validation function must return an error. The classification must be consistent and exhaustive across all possible Unicode strings.

**Validates: Requirements 3.1, 3.2, 3.3**

### Property 5: Tongue-Twister Selection Never Repeats Shown IDs

*For any* `GameSession` with a `shown_ids` list of any length (including empty) and any pool of eligible tongue-twisters, the next-twister selection logic must return a tongue-twister whose ID is not present in the session's `shown_ids` list; and after the selection, the returned ID must be appended to the `shown_ids` list.

**Validates: Requirements 6.1, 6.2, 6.4**

### Property 6: Cumulative Score Equals Sum of Attempt Scores

*For any* sequence of successful tongue-twister attempts with independently computed per-attempt scores, the cumulative score stored in the Game_Session after all attempts must equal the arithmetic sum of the individual per-attempt scores, with no rounding, truncation, or loss.

**Validates: Requirements 8.3, 8.4, 8.5**

### Property 7: Leaderboard Retains Highest Score Per Nickname

*For any* nickname and any pair of scores s1 and s2 submitted in any order, the leaderboard must contain exactly one entry for that nickname with a score equal to max(s1, s2). Submitting a lower score after a higher score must not overwrite the higher score.

**Validates: Requirements 12.6**

### Property 8: Heart Decrement is Exactly One Per Failed Attempt

*For any* Game_Session with a heart count in [1, 2, 3], recording exactly one failed attempt must reduce the heart count by exactly 1, leaving it at `previous_hearts - 1`. The decrement must never be 0, 2, or any value other than 1.

**Validates: Requirements 9.2**

---

## Observability and Logging

### Library Choice

The project uses Go's standard library `log/slog` (available since Go 1.21). No third-party logging library is introduced. `slog` writes key-value pairs natively, supports log levels, and outputs JSON in production or human-readable text in development — chosen via the environment variable `LOG_FORMAT` (`json` or `text`, default `text`).

Why `slog` over zerolog or zap: LinguaSpeed is a small, self-contained project. `slog` is zero-dependency, ships with the standard library, covers structured logging and log levels with no additional module, and keeps the dependency graph clean. The performance advantage of zerolog/zap is irrelevant at this scale.

### Log Levels in Use

| Level | When to use |
|---|---|
| `INFO` | Normal operations worth recording: server started, session created, game ended, admin login |
| `WARN` | Expected-but-notable failures: 401/403/404/409/503 responses, session not found, leaderboard upsert skipped |
| `ERROR` | Unexpected failures requiring investigation: DB write failed, cache unavailable, 5xx responses, serialization errors |
| `DEBUG` | Not used in production; reserved for future verbose tracing if needed |

### Standard Fields

Every log line MUST include these fields:

| Field | Type | Value |
|---|---|---|
| `time` | string (RFC3339) | Timestamp of the log event |
| `level` | string | `INFO`, `WARN`, or `ERROR` |
| `msg` | string | Short human-readable description of the event |
| `request_id` | string | UUID assigned per HTTP request by the request-ID middleware (absent for non-request logs) |

Additional fields are added per event (see Event Catalogue below).

### Request Logging Middleware

Every HTTP request emits exactly one log line when the response is complete. The middleware wraps `http.ResponseWriter` to capture the status code written by the handler, then selects the log level based on the response:

```go
// requestLogMiddleware logs one line per completed HTTP request.
// It never logs request bodies — only metadata (method, path, status, latency, request_id).
func requestLogMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        start := time.Now()
        rw    := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
        next.ServeHTTP(rw, r)
        latencyMS := time.Since(start).Milliseconds()

        level := slog.LevelInfo
        if rw.status >= 500 {
            level = slog.LevelError
        } else if rw.status >= 400 {
            level = slog.LevelWarn
        }
        slog.Log(r.Context(), level, "request completed",
            "method",     r.Method,
            "path",       r.URL.Path,
            "status",     rw.status,
            "latency_ms", latencyMS,
            "request_id", requestIDFromContext(r.Context()),
        )
    })
}
```

The request body (typed text, passwords, nicknames) is **never** logged. Only request metadata is recorded.

### Request ID Middleware

A UUID v4 is generated for every incoming request and stored in the request context. It is attached to every log line emitted during that request's lifetime and returned to the client in the `X-Request-ID` response header. This allows a frontend error report and the corresponding backend log line to be correlated precisely.

### Event Catalogue

| Event | Level | Extra fields | Requirement |
|---|---|---|---|
| Server started | INFO | `port` | Req 1 |
| Migration applied | INFO | `migration_name`, `migration_version` | Req 2.1 |
| Migration failed | ERROR | `migration_name`, `error` (sanitized) | Req 2.2 |
| Seed skipped (admin exists) | INFO | — | Req 2.10 |
| Admin seed created | INFO | `username` | Req 2.8 |
| Session created | INFO | `session_id`, `difficulty` | Req 21.5 |
| Game ended | INFO | `session_id`, `outcome` (win/loss), `final_score`, `difficulty`, `db_saved` (bool) | Req 21.6 |
| Leaderboard upsert failed | ERROR | `session_id`, `error` (sanitized) | Req 21.7 |
| Admin login succeeded | INFO | `username` | Req 21.9 |
| Admin login failed | WARN | `username`, `reason` (bad_credentials / role_mismatch) | Req 21.8 |
| JWT rejected | WARN | `reason` (missing / invalid / expired / wrong_role), `path` | Req 13.8–13.10 |
| Cache unavailable | ERROR | `operation` (e.g., session_read, session_write, leaderboard_upsert), `error` (sanitized) | Req 6.5, 6.6, 9.4 |
| DB write failed | ERROR | `operation` (e.g., save_game_record), `error` (sanitized) | Req 10.3, 11.3 |

### What Is Never Logged

- Passwords — in any form, plain or hashed
- Player nicknames — user-chosen data; omitting them reduces PII exposure
- Raw database error messages with table or column names — sanitize before logging
- JWT token values
- Full request bodies
- SQL query strings

### Example Log Lines

A completed request (LOG_FORMAT=json):

```json
{
  "time":       "2024-01-15T14:30:04Z",
  "level":      "INFO",
  "msg":        "request completed",
  "request_id": "a3f1c820-3e2b-4d9f-b7e0-1c5d8f9a2b4c",
  "method":     "POST",
  "path":       "/api/game/attempt",
  "status":     200,
  "latency_ms": 12
}
```

A game-ending event (LOG_FORMAT=json):

```json
{
  "time":        "2024-01-15T14:30:04Z",
  "level":       "INFO",
  "msg":         "game ended",
  "request_id":  "a3f1c820-3e2b-4d9f-b7e0-1c5d8f9a2b4c",
  "session_id":  "550e8400-e29b-41d4-a716-446655440000",
  "outcome":     "win",
  "final_score": 2800,
  "difficulty":  "medium",
  "db_saved":    true
}
```

---

## Error Handling

### Strategy

All errors are wrapped with context at each layer using `fmt.Errorf("…: %w", err)`. Handlers translate domain errors to HTTP status codes via a central `handler.writeError` helper that maps sentinel errors to statuses:

| Sentinel | HTTP Status |
|---|---|
| `domain.ErrNotFound` | 404 |
| `domain.ErrValidation` | 422 |
| `domain.ErrConflict` | 409 |
| `domain.ErrUnauthorized` | 401 |
| `domain.ErrForbidden` | 403 |
| `domain.ErrCacheUnavailable` | 503 |
| anything else | 500 |

### Partial Write Prevention

- **Session creation**: Uses Redis `SET NX EX`. If the write fails, the session does not exist in Redis and the error is returned immediately — no partial state.
- **Game record persistence**: Uses a PostgreSQL transaction spanning the `game_records` insert and all `attempt_details` inserts. The session is only deleted from Redis after the DB transaction commits.
- **Session JSON**: The struct is marshalled to a `[]byte` before calling Redis `SET`. If `json.Marshal` returns an error, no write is made (Requirement 20.4).

### Startup Failure

`config.Load()` collects all missing variable names and exits with code 1 after logging all of them in a single message, so operators see every missing variable at once rather than one at a time.

---

## Testing Strategy

### Unit Tests

Unit tests cover:

- `scoring.Compute`: input boundaries, zero score clamp, default constants
- `config.Load`: missing variables, present variables, default port
- Validation helpers: nickname length boundaries, difficulty enum, tongue-twister text rules
- JWT middleware: valid token, expired token, wrong role, missing header
- `handler` layer: using `httptest.NewRecorder`, mock services, all happy/error paths

### Property-Based Tests

The project uses [`pgregory.net/rapid`](https://pkg.go.dev/pgregory.net/rapid) as the property-based testing library. Each property test runs a minimum of 100 iterations.

```
// Tag format: Feature: lingua-speed, Property {N}: {property_text}
```

Property tests are located in `internal/scoring/scoring_prop_test.go` and `internal/domain/session_prop_test.go`.

| Test file | Properties covered |
|---|---|
| `internal/domain/session_prop_test.go` | Property 1 (round-trip) |
| `internal/scoring/scoring_prop_test.go` | Properties 2, 3 |
| `internal/service/game_prop_test.go` | Properties 4, 5, 6, 7, 8 |

**Property 1 — Round-trip test outline:**

```go
// Feature: lingua-speed, Property 1: GameSession serialization round-trip
rapid.Check(t, func(t *rapid.T) {
    session := generateArbitraryGameSession(t)  // draws using rapid generators
    data, err := json.Marshal(session)
    require.NoError(t, err)
    var restored GameSession
    err = json.Unmarshal(data, &restored)
    require.NoError(t, err)
    require.Equal(t, session, restored)
})
```

**Property 2–3 — Scoring test outline:**

```go
// Feature: lingua-speed, Property 2: scoring non-negativity
rapid.Check(t, func(t *rapid.T) {
    timeMS := rapid.Int64Range(0, 1_000_000).Draw(t, "timeMS")
    failed := rapid.IntRange(0, 100).Draw(t, "failed")
    score  := scoring.Compute(timeMS, failed, 0, 0)
    require.GreaterOrEqual(t, score, 0)
})

// Feature: lingua-speed, Property 3: scoring degrades with failures and time
rapid.Check(t, func(t *rapid.T) {
    timeMS  := rapid.Int64Range(0, 1_000_000).Draw(t, "timeMS")
    failed  := rapid.IntRange(0, 99).Draw(t, "failed")
    score1  := scoring.Compute(timeMS, failed,   0, 0)
    score2  := scoring.Compute(timeMS, failed+1, 0, 0)
    require.GreaterOrEqual(t, score1, score2)  // more failures → same or lower score

    timeMS2 := rapid.Int64Range(timeMS, 1_000_000).Draw(t, "timeMS2")
    score3  := scoring.Compute(timeMS2, failed, 0, 0)
    require.GreaterOrEqual(t, score1, score3)  // more time → same or lower score
})
```

### Integration Tests

Integration tests (using `testcontainers-go` or a shared test Docker Compose) cover:

- Full game flow: start → next → attempt (fail) → attempt (success) → next → … → win
- Loss condition: start → attempt (fail × 3) → game over
- Leaderboard upsert: same nickname submits lower then higher score; only highest retained
- Admin login and CRUD for tongue-twisters
- Migrations: clean DB → migrations apply → schema matches expected structure

### Frontend Tests

Manual smoke tests cover the happy path for each page. The character-mismatch highlight logic is tested in isolation using plain JS unit tests (no framework).
