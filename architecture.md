# Zabaan Backend — Architecture

Concise reference for design choices, structure, and rules.

---

## Layered architecture

Request flow: **HTTP → Handler → Service → Repository → Database**.

| Layer | Responsibility | No |
|-------|----------------|-----|
| **Handler** | Parse request, call service, write HTTP/JSON. No business logic. | No DB, no business rules |
| **Service** | Business logic, validation, orchestration. | No HTTP, no `http.ResponseWriter` |
| **Repository** | Data access only. | No business rules |

Dependencies are wired in **main.go** (repository → service → handler). No layer skips another (e.g. handler never talks to repository directly).

---

## Module layout

| Module | Handler | Service | Repository | Purpose |
|--------|---------|---------|------------|---------|
| **auth** | ✅ | ✅ | uses user | Signup, login, getToken, JWT, revocation |
| **user** | ✅ | ✅ | ✅ | User CRUD; auth uses user repo |
| **health** | ✅ | — | — | `/health`, `/` |
| **config** | — | — | — | Env load/validate |
| **database** | — | — | — | MySQL connection, migrations |
| **middleware** | — | — | — | RequireAuth, rate limit (global) |

Shared types live in **internal/models**. API docs in **docs/** (Swagger).

---

## Per-module file conventions

For **feature modules** (auth, user) we use:

```
internal/<module>/
├── dto/                 # Request/response structs (one file per resource or verb)
│   ├── signup.go
│   └── login.go
├── errors.go            # Error → HTTP (status, message) mappers; sentinel errors stay in service/repo
├── responses.go         # WriteJSON, WriteError, MethodNotAllowed (no business logic)
├── middleware.go       # Optional: module-specific HTTP helpers (e.g. optional Bearer + credentials)
├── handler.go          # Endpoint orchestration only: parse → service → respond
├── service.go
└── repository.go       # Only in modules that own data
```

- **dto/** — One package per module; one file per DTO group (e.g. `signup.go`, `login.go`, `create.go`). Chosen for scalability.
- **errors.go** — `MapXError(err) (status int, msg string)`. Returns `(0, "")` for unknown errors; handler falls back to 500.
- **responses.go** — Shared JSON helpers so handlers don’t repeat `json.NewEncoder(w).Encode(...)`.
- **handler.go** — Thin: validate input (or use DTO), call service, map errors via `MapXError`, write via `WriteJSON`/`WriteError`.

**Global** cross-cutting concerns (JWT auth, rate limit) stay in **internal/middleware/**.

---

## Rules we keep

1. **Interfaces for external deps** — Handlers depend on service interfaces (e.g. `AuthService`), not concrete types. Middleware depends on `TokenValidator`, not full auth service. Enables mocks in tests.
2. **One place for errors** — Domain errors (e.g. `ErrEmailExists`) are defined in service or repository; HTTP mapping is in `errors.go`.
3. **JSON only** — All responses are JSON. Set `Content-Type: application/json` in handler or in `WriteJSON`/`WriteError`.
4. **Config from env** — `config.Load()` + `config.Validate()`. Production requires non-default `JWT_SECRET` and `DATABASE_URL`.
5. **Structured logging** — `log/slog` with fields (`handler`, `component`, `err`). JSON in production, text in development.
6. **No ORM** — Raw `database/sql` + MySQL driver. Models in `internal/models`; GORM-style tags not used for DB.
7. **Auth** — JWT (HS256), `token_valid_after` for revocation. Protected routes use `middleware.RequireAuth`. Auth endpoints rate-limited per IP (sliding window).

---

## Request flow (summary)

- **Signup/Login/GetToken** — Rate limit → handler → parse DTO → service (validate, hash, repo) → token → `WriteJSON`.
- **GetToken** — Same as login then revoke previous tokens and issue new one with matching `iat`.
- **Protected (e.g. /users)** — `RequireAuth` (JWT + revocation check, claims in context) → handler → service → repository → `WriteJSON`.

---

## Where to add things

| Need | Place |
|------|--------|
| New endpoint | Handler in the right module; add route in main |
| New request/response shape | New or existing file under `internal/<module>/dto/` |
| New domain error | Service or repository; map in `errors.go` |
| New HTTP helper | `responses.go` or, if auth-specific, `auth/middleware.go` |
| New global middleware | `internal/middleware/` |
| New shared model | `internal/models/` |
| New env var | `internal/config/config.go` |

---

*See **learning.md** for request-by-request flows, DB setup, and file-by-file reading order.*
