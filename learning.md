# Learning: Zabaan Backend

A short guide to how this codebase is structured and how the pieces fit together.

---

## What the app does

- **Auth:** Signup (create user + get token), Login (email/password → token), GetToken (new token + revoke all previous tokens for that user).
- **Users:** List users and get one user by ID (both require a valid JWT).
- **Health:** `/health` returns server and DB status; `/` returns API info.

All responses are JSON. Auth endpoints are rate-limited per IP; protected routes require `Authorization: Bearer <token>`.

---

## Architecture

The app uses a **layered style**: HTTP → handler → service → repository.

- **Handler:** Reads the request (body, headers), calls the service, writes the HTTP response (status, JSON).
- **Service:** Business logic (validation, hashing, token creation, calling the repository). No HTTP.
- **Repository:** Talks to the database (or another store). No business rules.

```
  HTTP request
       ↓
  Handler (auth, user, health)
       ↓
  Service (auth, user)
       ↓
  Repository (user)  ←→  Database
```

**Modules:**

| Module   | Handler | Service | Repository | Purpose                          |
|----------|---------|---------|------------|----------------------------------|
| auth     | ✅      | ✅      | uses user  | Signup, login, getToken, JWT     |
| user     | ✅      | ✅      | ✅         | User CRUD, used by auth          |
| health   | ✅      | —       | —          | /health, /                       |
| config   | —       | —       | —          | Load env (PORT, JWT_SECRET, …)  |
| database | —       | —       | —          | MySQL connection, table setup    |
| middleware | —     | —       | —          | RequireAuth, rate limit          |

---

## Project layout

```
zabaan_backend/
├── main.go                 # Entry: load config, wire dependencies, start server
├── go.mod / go.sum         # Go modules
├── .env                    # Your local env (do not commit)
├── .env.example            # Template env (commit this)
│
├── internal/
│   ├── config/             # config.Load(), config.Validate(), env parsing
│   ├── database/           # DB connection, createUsersTable, ensureAuthColumns
│   ├── models/             # Shared structs (e.g. User)
│   │
│   ├── auth/               # Authentication
│   │   ├── handler.go      # Signup, Login, GetToken HTTP handlers
│   │   ├── service.go      # SignUp, Login, CreateToken, ValidateTokenFull, …
│   │   └── auth.go        # JWT: CreateToken, ValidateToken, Claims
│   │
│   ├── user/               # User resource
│   │   ├── handler.go      # Users (GET list, GET :id, POST create)
│   │   ├── service.go      # List, GetByID, Create
│   │   └── repository.go   # DB: List, GetByID, GetByEmail, CreateWithPassword, token_valid_after
│   │
│   ├── health/             # Health and root
│   │   └── handler.go     # Check (health), Root (API info)
│   │
│   └── middleware/
│       ├── auth.go         # RequireAuth (JWT required), GetClaimsFromRequest
│       └── ratelimit.go    # AuthRateLimiter (per-IP limit on signup/login/getToken)
│
└── docs/                   # Swagger (swag-generated)
    ├── docs.go
    ├── swagger.json
    └── swagger.yaml
```

---

## How a request is handled

### 1. Signup `POST /signup`

1. **main** → `authRateLimiter.Wrap(authHandler.Signup)` → **middleware/ratelimit** checks IP; if over limit → 429.
2. **auth/handler.Signup** → parse JSON body (first_name, last_name, email, password), max body 1MB → call **auth/service.SignUp**.
3. **auth/service.SignUp** → validate email (format, length), password (length, letter+number, max 72 bytes), normalize email (lowercase) → bcrypt hash → **user/repository.CreateWithPassword**.
4. **auth/handler** → on success, **auth/service.CreateToken** → return 201 with `user` + `token` (and `Authorization: Bearer <token>`).

### 2. Login `POST /login`

1. Rate limiter (same as above).
2. **auth/handler.Login** → optional Bearer checked (if present must be valid and for same user) → parse email/password → **auth/service.Login** (email normalized, lookup by email, bcrypt compare).
3. On success → **CreateToken** → 200 with `user` + `token`.

### 3. GetToken `POST /getToken`

Same as login (email/password), but then:

- **auth/service.RevokePreviousTokensAt** (sets `token_valid_after` in DB so all older tokens are invalid).
- **CreateTokenWithIssuedAt** with that time so the new token is not revoked.
- Response: 200 with `token` only.

### 4. Protected route `GET /users` or `GET /users/:id`

1. **middleware.RequireAuth** → read `Authorization: Bearer <token>` → **auth/service.ValidateTokenFull** (parse JWT + check not revoked via `token_valid_after`) → put claims in context.
2. **user/handler.Users** → **middleware.GetClaimsFromRequest** (optional; here we only need “authenticated”) → **user/service.List** or **GetByID** → return JSON.

---

## Concepts worth knowing

### Config

- **config.Load()** reads `.env` (godotenv) and fills **Config** (Port, DatabaseURL, JWTSecret, Environment, TrustProxy, TokenExpiry, RevocationTolerance).
- **config.Validate()** in production requires non-default JWT_SECRET and DATABASE_URL; main calls it before starting the server.

### Auth and JWT

- **auth/auth.go:** Low-level JWT: build claims (sub=userID, email, exp, iat), sign with HS256, parse and validate.
- **auth/service.go:** Uses that + **UserRepository** (CreateWithPassword, GetByEmail, GetTokenValidAfter, UpdateTokenValidAfter). Handles signup, login, token creation, and **revocation** (tokens issued before `token_valid_after` are rejected).
- **auth/handler.go:** Depends on **AuthService** interface (not concrete *Service), so tests can pass a mock.

### Interfaces

- **AuthService** (in handler): SignUp, Login, CreateToken, CreateTokenWithIssuedAt, RevokePreviousTokensAt, ValidateTokenFull. Implemented by **auth.Service**.
- **TokenValidator:** ValidateTokenFull. Implemented by **auth.Service**; used by **middleware.RequireAuth** so middleware doesn’t depend on the full auth service.
- **UserRepository** (in auth): CreateWithPassword, GetByEmail, GetTokenValidAfter, UpdateTokenValidAfter. Implemented by **user.Repository**.

### Rate limiting

- **middleware.AuthRateLimiter:** Per-IP, sliding window (e.g. 10 requests per minute). Used on signup, login, getToken.
- If **TrustProxy** is true, client IP is taken from X-Real-IP or X-Forwarded-For (first IP).

### Database

- **database.Init(cfg)** opens MySQL if DATABASE_URL is set, creates `users` table if needed, adds auth columns (first_name, last_name, password_hash, token_valid_after).
- **database.DB** is used by **user.NewRepository(database.DB)**; auth uses the same repo for user + token_valid_after.

### Logging

- **log/slog** is used everywhere. Default logger is set in main: JSON in production, text in development.
- Logs use structured fields (e.g. `handler`, `component`, `err`, `reason`).

---

## Connecting to the database

The app uses **MySQL** and the Go driver `github.com/go-sql-driver/mysql`. Connection is controlled by the **DATABASE_URL** environment variable.

### 1. Install and run MySQL

- Install MySQL (or MariaDB) locally or use a cloud instance.
- Create a database for the app (e.g. `zabaan`):

```sql
CREATE DATABASE zabaan;
CREATE USER 'your_user'@'localhost' IDENTIFIED BY 'your_password';
GRANT ALL PRIVILEGES ON zabaan.* TO 'your_user'@'localhost';
FLUSH PRIVILEGES;
```

(Adjust user/host/password to match your setup.)

### 2. Set DATABASE_URL

In your **.env** file (copy from `.env.example` if needed), set:

```
DATABASE_URL=username:password@tcp(host:port)/database_name
```

**Examples:**

- Local MySQL, user `root`, password `mypass`, DB `zabaan`, default port:
  ```
  DATABASE_URL=root:mypass@tcp(127.0.0.1:3306)/zabaan
  ```
- With optional query params (e.g. charset):
  ```
  DATABASE_URL=root:mypass@tcp(127.0.0.1:3306)/zabaan?charset=utf8mb4
  ```

The code automatically appends `parseTime=true` (or `&parseTime=true` if the URL already has `?`), so `DATETIME` columns are parsed as `time.Time`.

### 3. What the app does on startup

When **database.Init(cfg)** runs and DATABASE_URL is non-empty:

1. Opens a connection to MySQL and **pings** it (exits with an error if the DB is unreachable).
2. Creates the **users** table if it doesn’t exist (id, email, username, created_at, updated_at).
3. Runs **ensureAuthColumns**: adds `first_name`, `last_name`, `password_hash`, `token_valid_after` if they are missing (safe to run multiple times).

So you only need to create the **database** (and user) yourself; the app creates/updates the table and columns.

### 4. If DATABASE_URL is not set

- **database.Init** does nothing: no connection is opened, **database.DB** stays `nil`.
- Auth and user endpoints that use the DB will fail (e.g. “database not available” or connection errors).
- **Production:** **config.Validate()** requires DATABASE_URL when `ENVIRONMENT=production`, so the server won’t start without it.

### 5. Connection pool settings

In **internal/database/database.go** the pool is configured as:

- **ConnMaxLifetime:** 5 minutes
- **MaxIdleConns:** 2

You can change these in code if you need different limits.

---

## Running the app

1. **Copy env:** `cp .env.example .env`
2. **Set DATABASE_URL** (and optionally JWT_SECRET, PORT, etc.) in `.env` — see [Connecting to the database](#connecting-to-the-database) above.
3. **Start:** `go run .`
4. Server listens on `:8080` (or PORT from env). Try `GET /health` to confirm DB status, then use signup/login with a JSON body.

---

## Files to read in order

1. **main.go** – See how config, DB, repos, services, handlers, and middleware are wired and which routes exist.
2. **internal/config/config.go** – What env vars exist and how they’re validated.
3. **internal/auth/handler.go** – How signup/login/getToken HTTP handlers call the service and map errors to status codes.
4. **internal/auth/service.go** – SignUp, Login, token creation, revocation; use of UserRepository and auth/auth.go.
5. **internal/auth/auth.go** – JWT creation and validation (claims, secret, expiry).
6. **internal/user/repository.go** – How users and token_valid_after are stored and read.
7. **internal/middleware/auth.go** – How RequireAuth validates the Bearer token and puts claims in context.
8. **internal/middleware/ratelimit.go** – How per-IP rate limiting and optional proxy headers work.

This should be enough to follow any request from HTTP to database and back.
