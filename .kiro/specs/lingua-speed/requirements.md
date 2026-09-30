# Requirements Document

## Introduction

LinguaSpeed is a web-based typing game where players race against a timer to type Spanish tongue-twisters as fast and accurately as possible. Players choose a difficulty level, receive tongue-twisters one at a time from the database, and earn a score based on speed and accuracy. A 3-life system adds stakes to each attempt. Final results are persisted to PostgreSQL, and a leaderboard backed by Redis shows top scores. An admin panel protected by JWT authentication allows an administrator to manage the tongue-twister catalogue.

The system is composed of a Go backend API, a PostgreSQL database for permanent data, a Redis store for live game state and the leaderboard, and a plain HTML/CSS/JS frontend. All services run via Docker Compose.

---

## Glossary

- **Game**: A single play-through from nickname entry to game-over (win or loss).
- **Game_Session**: The live, in-progress state of a Game stored in Redis, identified by a Session_ID.
- **Session_ID**: A unique identifier (UUID) generated when a player starts a new Game, stored client-side in a cookie or localStorage and used to look up the Game_Session in Redis.
- **Player**: An anonymous user identified only by a self-chosen Nickname for the duration of a Game.
- **Request_Log**: A structured log line emitted by the Server for every incoming HTTP request, recording at minimum the HTTP method, path, response status code, and response latency.
- **Nickname**: A non-empty string chosen by the Player before starting a Game, used to identify the Player on the Leaderboard.
- **Tongue_Twister**: A Spanish phrase stored in the database with an associated Difficulty and Active flag.
- **Difficulty**: A manually assigned label for a Tongue_Twister: one of easy, medium, or hard.
- **Active_Flag**: A boolean field on a Tongue_Twister indicating whether it is available for play (active) or hidden from games (inactive).
- **Heart**: A life unit. Each Player starts a Game with 3 Hearts. Losing all Hearts ends the Game.
- **Attempt**: One instance of a Player typing a Tongue_Twister from start to finish or until a mistake triggers a reset.
- **Score**: A numeric value computed per Tongue_Twister Attempt based on accuracy and speed, summed across all Attempts for the final Game Score.
- **Leaderboard**: A ranked list of top Game results stored in Redis, keyed by final Score.
- **Log_Level**: A severity label attached to each log line — one of `DEBUG`, `INFO`, `WARN`, or `ERROR` — indicating the importance of the event.
- **Admin**: The single privileged user who can manage Tongue_Twisters via the Admin_Panel.
- **Admin_Panel**: The password-protected section of the application used to create, edit, list, and deactivate Tongue_Twisters.
- **JWT**: JSON Web Token used to authenticate and authorize Admin access to protected endpoints.
- **Game_Record**: The permanent PostgreSQL record written at Game end, containing the Nickname, final Score, and per-Tongue_Twister details.
- **Attempt_Detail**: A sub-record within a Game_Record capturing which Tongue_Twister was shown, how many Attempts were made, time taken, and success or failure.
- **Server**: The Go HTTP backend application.
- **Database**: The PostgreSQL instance storing permanent data (Tongue_Twisters, Admin users, Game_Records).
- **Cache**: The Redis instance storing Game_Sessions and the Leaderboard.
- **Seed_Script**: A one-time initialization script that creates the first Admin user in the Database on first startup.

---

## Requirements

### Requirement 1: Infrastructure and Deployment

**User Story:** As a developer, I want all services to run with a single command, so that the development environment is fully reproducible and easy to start.

#### Acceptance Criteria

1. THE Docker_Compose_Configuration SHALL define exactly three services: the Server, the Database, and the Cache.
2. WHEN the compose stack is started, THE Server SHALL start and accept HTTP connections only after the Database and Cache report a healthy status, where health is confirmed by a successful TCP connection to each service within 30 seconds and up to 5 retry attempts.
3. THE Server SHALL read all runtime configuration (database connection string, Cache address, JWT secret, server port) from environment variables.
4. IF a required environment variable is absent at startup, THEN THE Server SHALL exit with a non-zero status code and log an error message identifying the name of each missing variable before exiting.
5. THE Docker_Compose_Configuration SHALL expose the Server on a host port defined by an environment variable, defaulting to port 3000, so that the Server is reachable from the host machine's browser.
6. WHEN the compose stack is started, THE Docker_Compose_Configuration SHALL mount a named volume for the Database service so that database data persists across container restarts.

---

### Requirement 2: Database Schema and Seed

**User Story:** As a developer, I want the Database schema and initial Admin user to be created automatically on startup, so that no manual migration steps are required to run the application.

#### Acceptance Criteria

1. WHEN the Server starts, THE Server SHALL apply all pending database migrations in order before accepting any incoming requests.
2. IF any database migration fails during startup, THEN THE Server SHALL halt startup and emit an error message indicating which migration failed, without applying subsequent migrations.
3. THE Database SHALL contain a `tongue_twisters` table with columns: id, text, difficulty, active, created_at, updated_at.
4. THE Database SHALL contain a `users` table with columns: id, username, hashed_password, role, created_at, updated_at.
5. THE Database SHALL contain a `game_records` table with columns: id, nickname, final_score, difficulty, started_at, ended_at.
6. THE Database SHALL contain an `attempt_details` table with columns: id, game_record_id, tongue_twister_id, attempt_count, time_taken_ms, success.
7. THE `users` table `role` column SHALL accept only the values `admin` and `moderator` and reject any other value.
8. WHEN the Seed_Script runs and no Admin user exists in the Database, THE Seed_Script SHALL create exactly one Admin user with the role `admin` using credentials supplied via environment variables.
9. IF the Seed_Script runs and the required credential environment variables are absent or empty, THEN THE Seed_Script SHALL abort and emit an error message indicating which variables are missing, without creating any user record.
10. IF an Admin user already exists in the Database when the Seed_Script runs, THEN THE Seed_Script SHALL skip creation and log an informational message.

---

### Requirement 3: Nickname Entry

**User Story:** As a Player, I want to enter a nickname before starting a game, so that my result appears on the Leaderboard under a recognizable name.

#### Acceptance Criteria

1. THE Server SHALL expose a start-game endpoint that accepts a Nickname of 1-32 characters and a Difficulty value of easy, medium, or hard.
2. WHEN a Player submits a Nickname that is an empty string, THE Server SHALL return HTTP 422 and an error message indicating that the Nickname is required.
3. WHEN a Player submits a Nickname exceeding 32 characters, THE Server SHALL return HTTP 422 and an error message indicating the 32-character maximum length.
4. IF a Player submits a Difficulty value other than easy, medium, or hard, THEN THE Server SHALL return HTTP 422 and an error message listing the three accepted values.
5. WHEN a Player submits a Nickname of 1-32 characters and a valid Difficulty, THE Server SHALL create a new Game_Session in the Cache containing the Nickname, Difficulty, and a server-generated Session_ID, and return that Session_ID to the client.
6. IF the Cache is unavailable when creating a Game_Session, THEN THE Server SHALL return HTTP 503 and an error message indicating that the service is temporarily unavailable.

---

### Requirement 4: Difficulty Selection

**User Story:** As a Player, I want to choose a difficulty level before the game starts, so that I can play tongue-twisters that match my skill.

#### Acceptance Criteria

1. THE Frontend SHALL present exactly three difficulty options to the Player: easy, medium, and hard.
2. WHEN a Player selects a Difficulty and submits the start form, THE Frontend SHALL send the selected Difficulty to the Server along with the Nickname.
3. IF a Player submits the start form without selecting a Difficulty, THEN THE Frontend SHALL display an error indicating that a Difficulty selection is required and SHALL NOT submit the form.
4. WHILE a Game_Session is active, THE Server SHALL only select Tongue_Twisters whose Difficulty matches the Difficulty stored in the Game_Session.
5. WHILE a Game_Session is active, THE Server SHALL only select Tongue_Twisters whose Active_Flag is active.
6. IF no Tongue_Twisters with a matching Difficulty and an Active_Flag of active are available when the Server needs to select a Tongue_Twister, THEN THE Server SHALL respond with an error indicating that no content is available for the selected Difficulty.

---

### Requirement 5: Game Session Initialization

**User Story:** As a Player, I want the game to track my progress across tongue-twisters, so that my hearts, score, and history are preserved throughout the game.

#### Acceptance Criteria

1. WHEN a Game_Session is created, THE Server SHALL store the following fields in the Cache: Session_ID, Nickname, Difficulty, current Heart count (initialized to 3), current Score (initialized to 0), list of Tongue_Twister IDs already shown (initialized to empty), and a timestamp for session creation.
2. THE Server SHALL set an expiry on the Cache entry for a Game_Session such that the entry is removed after exactly 2 hours of inactivity, where inactivity is defined as no read or write operation on that Cache entry.
3. WHEN a client presents a Session_ID that does not exist in the Cache, THE Server SHALL return an error response indicating the session was not found or has expired.
4. THE Server SHALL associate each Game_Session with exactly one Session_ID, and each Session_ID with exactly one Game_Session.
5. WHEN a new Game_Session is created, THE Server SHALL return the Session_ID to the client in the response body.
6. IF a Game_Session creation request is missing one or more required fields (Nickname or Difficulty), THEN THE Server SHALL reject the request with an error response indicating which fields are missing, and SHALL NOT create a partial Game_Session in the Cache.
7. IF two concurrent requests attempt to create a Game_Session with the same Session_ID, THEN THE Server SHALL ensure only one Game_Session is stored in the Cache for that Session_ID.

---

### Requirement 6: Tongue-Twister Selection

**User Story:** As a Player, I want each round to show me a new tongue-twister I have not seen yet this game, so that every game session is fresh and non-repetitive.

#### Acceptance Criteria

1. WHEN the Server selects the next Tongue_Twister for a Game_Session, THE Server SHALL exclude all Tongue_Twister IDs present in the Game_Session's already-shown list.
2. WHEN the Server selects the next Tongue_Twister for a Game_Session, THE Server SHALL select randomly from the remaining eligible Tongue_Twisters that match the Game_Session's Difficulty and have an Active_Flag of active.
3. IF no eligible Tongue_Twisters remain for a Game_Session, THEN THE Server SHALL treat the condition as a win, notify the Player with a message indicating all tongue-twisters for the current Difficulty have been completed, and end the Game_Session within 3 seconds of the condition being detected.
4. WHEN a Tongue_Twister is selected for a Game_Session, THE Server SHALL add its ID to the already-shown list in the Cache before returning it to the client.
5. IF the Cache is unavailable when the Server attempts to read the already-shown list for a Game_Session, THEN THE Server SHALL return an error response indicating the session state cannot be retrieved and preserve the existing Game_Session state without modification.
6. IF the Cache is unavailable when the Server attempts to write a selected Tongue_Twister ID to the already-shown list, THEN THE Server SHALL return an error response indicating the session state could not be updated and not return the Tongue_Twister to the client.

---

### Requirement 7: Typing and Attempt Mechanics

**User Story:** As a Player, I want the game to detect mistakes as I type and penalize me for errors, so that accuracy matters alongside speed.

#### Acceptance Criteria

1. THE Frontend SHALL display the full Tongue_Twister text and an input field where the Player types, with a maximum input length of 500 characters.
2. WHEN the Player's typed input diverges from the expected Tongue_Twister text at any character position, THE Frontend SHALL visually distinguish the mismatched characters from correctly typed characters.
3. WHEN a Player submits a Tongue_Twister attempt that contains any character mismatch with the expected text, THE Server SHALL record the attempt as failed, deduct one Heart from the Game_Session, clear the typed input field to empty, and return the updated Heart count and remaining Attempts count to the Frontend.
4. WHEN a Player submits a Tongue_Twister attempt that matches the expected text exactly, THE Server SHALL record the attempt as successful and advance the Game_Session to the next Tongue_Twister.
5. IF the Server is unavailable when a Player submits an Attempt, THEN THE Frontend SHALL preserve the typed input and display a failure message without deducting Hearts.
6. THE Server SHALL track, per Tongue_Twister within a Game_Session, the number of Attempts made and the total time elapsed in milliseconds from first keystroke to successful submission.
7. WHEN a Player successfully completes a Tongue_Twister, THE Server SHALL NOT restore any Hearts that were lost during that Tongue_Twister's Attempts.

---

### Requirement 8: Scoring

**User Story:** As a Player, I want to earn a score based on how fast and accurately I type each tongue-twister, so that skilled players are rewarded on the Leaderboard.

#### Acceptance Criteria

1. THE Server SHALL compute a per-Tongue_Twister Score using both the time taken (in milliseconds) for the successful Attempt and the total number of failed Attempts made for that Tongue_Twister.
2. THE Server SHALL define the per-Tongue_Twister Score formula as: Score = max(0, base_points - (time_taken_ms / 100) - (failed_attempts * penalty)), where time_taken_ms is a non-negative integer, failed_attempts is a non-negative integer, and base_points and penalty are configurable constants with default values of 1000 and 200 respectively.
3. THE Server SHALL accumulate the per-Tongue_Twister Scores into the Game_Session's current Score in the Cache after each successful Tongue_Twister completion.
4. THE Server SHALL return the updated cumulative Score to the Frontend after each successful Tongue_Twister completion.
5. THE Server SHALL use the accumulated Score stored in the Game_Session as the final Score when writing the Game_Record at game end.
6. IF the Cache Score data is unavailable at game end, THEN THE Server SHALL return an error indicating the final Score could not be retrieved and SHALL NOT write a Game_Record with an incorrect Score.

---

### Requirement 9: Heart (Life) System

**User Story:** As a Player, I want to see my remaining hearts during the game, so that I understand how many mistakes I can still afford.

#### Acceptance Criteria

1. THE Server SHALL initialize every Game_Session with a Heart count of 3.
2. WHEN a failed Attempt is recorded, THE Server SHALL decrement the Heart count in the Cache by exactly 1.
3. WHEN the Heart count in the Cache reaches 0, THE Server SHALL end the Game_Session with a loss status immediately after recording the failed Attempt, without processing any further Attempts for that Game_Session.
4. IF the Heart count cannot be decremented due to a Cache write failure, THEN THE Server SHALL reject the Attempt with an error indicating the Game_Session state could not be updated and preserve the previous Heart count.
5. THE Server SHALL return the current Heart count as a non-negative integer in every response that modifies Game_Session state.
6. THE Frontend SHALL display the current Heart count using visually distinct states for each of the 3 heart positions, where each position renders as filled when the Heart count exceeds that position's index and empty otherwise.
7. IF the Frontend receives a Heart count outside the range 0 to 3 inclusive, THEN THE Frontend SHALL display an error state indicating the Game_Session data is invalid.

---

### Requirement 10: Game End - Win Condition

**User Story:** As a Player, I want the game to end gracefully when I finish all available tongue-twisters, so that my result is saved and I can see my final score.

#### Acceptance Criteria

1. WHEN no eligible Tongue_Twisters remain for a Game_Session (per Requirement 6, criterion 3), THE Server SHALL mark the Game_Session as ended with outcome win.
2. WHEN a Game_Session is ended with outcome win, THE Server SHALL write a Game_Record to the Database containing: Nickname, final Score, Difficulty, started_at, ended_at, and one Attempt_Detail row per Tongue_Twister shown.
3. IF the Database write of the Game_Record fails, THEN THE Server SHALL retain the Game_Session in the Cache and return an error response to the Frontend indicating the game result could not be saved.
4. WHEN the Game_Record is written successfully, THE Server SHALL remove the Game_Session from the Cache.
5. WHEN the Game_Record is written successfully, THE Server SHALL update the Leaderboard in the Cache with the Player's Nickname and final Score.
6. THE Server SHALL return the outcome (win), the final Score, and the Nickname to the Frontend within 3 seconds of the win condition being detected.

---

### Requirement 11: Game End - Loss Condition

**User Story:** As a Player, I want the game to end when I run out of hearts, so that failing all lives concludes the game and records my progress.

#### Acceptance Criteria

1. WHEN the Heart count in a Game_Session reaches 0, THE Server SHALL mark the Game_Session as ended with outcome loss.
2. WHEN a Game_Session is ended with outcome loss, THE Server SHALL write a Game_Record to the Database containing: Nickname, final Score, Difficulty, started_at, ended_at, and one Attempt_Detail row per Tongue_Twister shown (including the failing Tongue_Twister).
3. IF the Database write of the Game_Record fails, THEN THE Server SHALL retain the Game_Session in the Cache and return an error response to the Frontend indicating the game result could not be saved.
4. WHEN the Game_Record is written after a loss, THE Server SHALL remove the Game_Session from the Cache.
5. WHEN the Game_Record is written after a loss, THE Server SHALL update the Leaderboard in the Cache with the Player's Nickname and final Score.
6. WHEN a Game_Session ends with outcome loss, THE Server SHALL return the outcome (loss), the final Score, and the Nickname to the Frontend within 3 seconds of the Heart count reaching 0.

---

### Requirement 12: Leaderboard

**User Story:** As a Player, I want to see a leaderboard of top scores, so that I can compare my performance against other players.

#### Acceptance Criteria

1. THE Server SHALL store Leaderboard entries in the Cache as a sorted set keyed by Score, with the Player's Nickname as the member, supporting up to 1,000,000 entries.
2. WHEN a Game ends (win or loss), THE Server SHALL upsert the Player's entry into the Leaderboard sorted set using the final Score within 2 seconds of the Game ending.
3. WHEN the Leaderboard is requested, THE Server SHALL return the top 10 entries ordered by Score descending, where each entry contains rank, Nickname, and Score.
4. THE Server SHALL expose a public (unauthenticated) endpoint to retrieve the Leaderboard and respond within 2 seconds.
5. THE Frontend SHALL display the Leaderboard with each entry's rank, Nickname (truncated to 20 characters if longer), and Score.
6. IF the same Nickname submits multiple Game results, THE Cache SHALL retain only the entry with the highest Score for that Nickname, discarding only the lower Score without affecting other entries.
7. IF the Cache is unavailable when the Leaderboard is requested, THEN THE Server SHALL return an error response indicating the Leaderboard is temporarily unavailable.
8. IF two Leaderboard entries have the same Score, THE Cache SHALL rank the entry submitted earlier as higher.

---

### Requirement 13: Admin Authentication

**User Story:** As an Admin, I want to log in with a username and password, so that tongue-twister management is protected from unauthorized access.

#### Acceptance Criteria

1. THE Server SHALL expose a login endpoint that accepts a username (1-64 characters) and a password (1-128 characters).
2. WHEN a login request is received with a username that does not exist in the Database, THE Server SHALL return HTTP 401 and a generic error message that does not distinguish whether the username or password was wrong.
3. WHEN a login request is received with a correct username but incorrect password, THE Server SHALL return HTTP 401 and the same generic error message used for an unknown username.
4. WHEN a login request is received with valid credentials for a user with role admin, THE Server SHALL return HTTP 200 and a signed JWT with the user ID and role embedded in the payload.
5. THE Server SHALL sign JWTs using a secret loaded from an environment variable.
6. THE JWT SHALL have an expiry of 8 hours from time of issuance.
7. THE Server SHALL store Admin passwords in the Database using bcrypt with a cost factor of at least 12.
8. WHEN a request to any Admin_Panel endpoint is received without a valid JWT in the Authorization header, THE Server SHALL return HTTP 401.
9. WHEN a request to any Admin_Panel endpoint is received with an expired JWT, THE Server SHALL return HTTP 401.
10. WHEN a request to any Admin_Panel endpoint is received with a JWT whose role claim is not admin, THE Server SHALL return HTTP 403.
11. IF a login request is received with a missing username or password field, THEN THE Server SHALL return HTTP 400 and an error message identifying the missing field.
12. WHEN a login request is received with valid credentials for a user whose role is not admin, THE Server SHALL return HTTP 403 and an error message indicating insufficient permissions.
13. IF a login request is received with a username exceeding 64 characters or a password exceeding 128 characters, THEN THE Server SHALL return HTTP 400 and an error message identifying the field that exceeds the limit.

---

### Requirement 14: Tongue-Twister Management - Create

**User Story:** As an Admin, I want to add new tongue-twisters to the database, so that the game catalogue grows over time.

#### Acceptance Criteria

1. THE Server SHALL expose an endpoint to create a Tongue_Twister that requires a valid Admin authentication token, accepting the fields: text (string) and difficulty (string).
2. IF a create request is received without a valid Admin authentication token, THEN THE Server SHALL return HTTP 401 and an error message indicating authentication is required.
3. WHEN a create request is received with a text field that is absent, null, empty, contains only whitespace characters, or exceeds 500 characters, THE Server SHALL return HTTP 422 and an error message identifying the text field as invalid.
4. WHEN a create request is received with a difficulty value other than easy, medium, or hard, THE Server SHALL return HTTP 422 and an error message listing the accepted values.
5. WHEN a valid create request is received, THE Server SHALL persist the Tongue_Twister to the Database with Active_Flag set to active and return HTTP 201 with the created record containing its generated ID, text, difficulty, and Active_Flag.

---

### Requirement 15: Tongue-Twister Management - Edit

**User Story:** As an Admin, I want to edit existing tongue-twisters, so that I can correct errors or update difficulty.

#### Acceptance Criteria

1. THE Server SHALL expose an authenticated endpoint to update a Tongue_Twister by ID, accepting text (1-500 characters) and/or difficulty; IF both fields are omitted from the request, THEN THE Server SHALL return HTTP 422 and an error message indicating that at least one field is required.
2. WHEN an edit request is received for a Tongue_Twister ID that does not exist in the Database, THE Server SHALL return HTTP 404 and an error message indicating the resource was not found.
3. WHEN an edit request contains a text field that is empty, contains only whitespace, or exceeds 500 characters, THE Server SHALL return HTTP 422 and an error message identifying the text field as invalid.
4. WHEN an edit request contains a difficulty value other than easy, medium, or hard, THE Server SHALL return HTTP 422 and an error message listing the accepted values.
5. WHEN a valid edit request is received, THE Server SHALL update only the provided fields in the Tongue_Twister record, set the updated_at timestamp to the current UTC time, and return HTTP 200 with the full updated record.
6. IF an edit request is received without a valid Admin authentication token, THEN THE Server SHALL return HTTP 401 and an error message indicating authentication is required.

---

### Requirement 16: Tongue-Twister Management - List

**User Story:** As an Admin, I want to see all tongue-twisters in the catalogue, so that I can review content and monitor what is active or inactive.

#### Acceptance Criteria

1. THE Server SHALL expose an authenticated endpoint to list Tongue_Twisters, returning at most 1000 records per response.
2. IF a list request is received without a valid Admin authentication token, THEN THE Server SHALL return HTTP 401 and an error message indicating authentication is required.
3. WHEN the list endpoint is called, THE Server SHALL return all Tongue_Twisters regardless of their Active_Flag (up to the 1000-record limit).
4. IF the difficulty query parameter is provided with a value other than easy, medium, or hard, THEN THE Server SHALL return HTTP 422 and an error message listing the accepted values.
5. WHEN the difficulty query parameter is provided with a valid value, THE Server SHALL return only Tongue_Twisters matching the specified Difficulty.
6. IF the active query parameter is provided with a value other than true or false, THEN THE Server SHALL return HTTP 422 and an error message indicating the accepted values.
7. WHEN the active query parameter is provided with value true or false, THE Server SHALL filter the results by Active_Flag accordingly.
8. THE Server SHALL return each Tongue_Twister record with fields: id, text, difficulty, active, created_at, updated_at.

---

### Requirement 17: Tongue-Twister Management - Deactivate

**User Story:** As an Admin, I want to deactivate tongue-twisters, so that I can remove content from the game without permanently deleting it.

#### Acceptance Criteria

1. THE Server SHALL expose an authenticated endpoint to set a Tongue_Twister's Active_Flag to inactive by ID.
2. IF a deactivate request is received without a valid Admin authentication token, THEN THE Server SHALL return HTTP 401 and an error message indicating authentication is required.
3. WHEN a deactivate request is received for a Tongue_Twister ID that does not exist in the Database, THE Server SHALL return HTTP 404.
4. IF a deactivate request is received for a Tongue_Twister whose Active_Flag is already inactive, THEN THE Server SHALL return HTTP 409 and an error message indicating the Tongue_Twister is already inactive.
5. WHEN a valid deactivate request is received, THE Server SHALL set the Tongue_Twister's Active_Flag to inactive, set the updated_at timestamp to the current UTC time, and return HTTP 200 with the updated record.
6. WHEN a Tongue_Twister is deactivated, THE Server SHALL NOT include that Tongue_Twister in future Tongue_Twister selections for any new or in-progress Game_Session.

---

### Requirement 18: Frontend - Game UI

**User Story:** As a Player, I want a clear and functional game interface, so that I can focus on typing without confusion.

#### Acceptance Criteria

1. WHILE a game round is in progress, THE Frontend SHALL display the current Tongue_Twister text in a dedicated area that is visually distinct from the input field and occupies the primary content area of the screen.
2. WHILE a game round is in progress, THE Frontend SHALL display an enabled text input field for the Player to type the Tongue_Twister.
3. WHILE a game round is in progress, THE Frontend SHALL display the current Heart count as a numeric value in a persistent area visible without scrolling.
4. WHILE a game round is in progress, THE Frontend SHALL display the current cumulative Score as a numeric value in a persistent area visible without scrolling.
5. WHEN a Tongue_Twister attempt is submitted, THE Frontend SHALL disable the input field and display a loading indicator until the Server response is received or 10 seconds have elapsed, whichever comes first.
6. IF 10 seconds elapse after a Tongue_Twister attempt is submitted without a Server response, THEN THE Frontend SHALL re-enable the input field, clear the loading indicator, and display an error message indicating the request timed out.
7. WHEN the Server returns a success response for an attempt that does not end the game, THE Frontend SHALL re-enable the input field, clear its contents, and advance to the next Tongue_Twister.
8. WHEN an error response is received from the Server for an attempt, THE Frontend SHALL re-enable the input field, clear its contents, and display the updated Heart count.
9. WHEN the Server returns a game-end response, THE Frontend SHALL navigate to a results screen displaying the outcome (win or loss), the final Score, and a navigation link to the Leaderboard page.

---

### Requirement 19: Frontend - Admin Panel UI

**User Story:** As an Admin, I want a simple web interface for the admin panel, so that I can manage tongue-twisters without using API tools directly.

#### Acceptance Criteria

1. THE Frontend SHALL provide a login page for the Admin_Panel that accepts a username input (1-50 characters) and a password input (1-100 characters).
2. WHEN a successful login response is received, THE Frontend SHALL store the JWT in localStorage and redirect the Admin to the tongue-twister management page within 1 second.
3. WHEN a failed login response is received, THE Frontend SHALL display the error message returned by the Server on the login page without clearing the username field.
4. THE Frontend SHALL provide a form to create a new Tongue_Twister with a text field (1-500 characters) and a difficulty field accepting values easy, medium, or hard.
5. THE Frontend SHALL display the full list of Tongue_Twisters with their ID, text, difficulty, and active status, showing up to 200 entries per page.
6. THE Frontend SHALL provide an edit form for each Tongue_Twister that allows updating the text field (1-500 characters) and/or the difficulty field accepting values easy, medium, or hard.
7. THE Frontend SHALL provide a deactivate action exclusively for each Tongue_Twister whose active status is true.
8. IF the JWT stored in localStorage is absent, THEN THE Frontend SHALL redirect the user to the Admin login page before rendering any protected page.
9. IF the Server returns an HTTP 401 response, THEN THE Frontend SHALL clear the JWT from localStorage and redirect the user to the Admin login page.

---

### Requirement 20: Round-Trip Integrity for Game Session Serialization

**User Story:** As a developer, I want the Game_Session data to survive serialization and deserialization through Redis without data loss, so that game state is always consistent.

#### Acceptance Criteria

1. WHEN the Server writes a Game_Session to the Cache, THE Server SHALL serialize the Game_Session state to a deterministic, lossless format such that serializing the same Game_Session state twice produces identical byte sequences.
2. WHEN the Server reads a Game_Session from the Cache, THE Server SHALL deserialize the stored data into an in-memory Game_Session object with all fields explicitly populated to their stored values.
3. THE Server SHALL guarantee that for any valid Game_Session, serializing then deserializing the Game_Session produces an in-memory object with field values identical to the original, including all nested structures and collection elements.
4. IF the Server encounters an error during Game_Session serialization, THEN THE Server SHALL not write any partial data to the Cache and SHALL propagate the error to the caller.
5. IF the Server encounters an error during Game_Session deserialization, THEN THE Server SHALL not return a partial Game_Session object and SHALL propagate the error to the caller.

---

### Requirement 21: Observability and Logging

**User Story:** As a developer, I want the server to emit structured logs for all significant events, so that I can diagnose failures and understand system behaviour without attaching a debugger.

#### Acceptance Criteria

1. THE Server SHALL emit a Request_Log for every HTTP request it processes, including the HTTP method, URL path, response status code, and response latency in milliseconds.
2. WHEN the Server returns an HTTP 5xx response, THE Server SHALL emit an ERROR-level log containing the request method, path, a sanitized error message (no raw database errors or stack traces), and a unique request ID.
3. WHEN the Server returns an HTTP 4xx response for a reason other than a validation error (i.e., 401, 403, 404, 409, 503), THE Server SHALL emit a WARN-level log containing the request method, path, status code, and a brief reason.
4. THE Server SHALL NOT log raw database query errors, SQL strings, or internal stack traces in any log line accessible outside the server process.
5. WHEN a Game_Session is created, THE Server SHALL emit an INFO-level log containing the Session_ID and Difficulty (but NOT the Nickname, to avoid logging player-chosen data).
6. WHEN a Game ends (win or loss), THE Server SHALL emit an INFO-level log containing the Session_ID, outcome (win/loss), final Score, Difficulty, and whether the Game_Record was written successfully.
7. WHEN the Leaderboard upsert fails after a game ends, THE Server SHALL emit an ERROR-level log containing the Session_ID and the sanitized error, without retrying automatically.
8. WHEN an admin login attempt fails (wrong credentials or insufficient role), THE Server SHALL emit a WARN-level log containing the username attempted and the failure reason (bad credentials or role mismatch), without logging the submitted password.
9. WHEN an admin login attempt succeeds, THE Server SHALL emit an INFO-level log containing the username and the timestamp of the login.
10. THE Server SHALL emit all logs in a consistent structured format (key-value pairs or JSON) so that log lines can be parsed and filtered programmatically.
