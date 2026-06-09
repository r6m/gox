# gox

`gox` is a small collection of reusable helpers for Go backend projects. It is
not a framework: it does not bootstrap applications, register routes, manage
dependencies, or impose repository, service, or entity layers.

The packages are intended to fit conventional applications built with Go,
chi, sqlc, Postgres, pgx, slog, and go-playground/validator.

## Install

```sh
go get github.com/r6m/gox
```

Import only the packages an application needs.

## Packages

| Package | Purpose |
| --- | --- |
| `httpx` | Error-returning handlers, JSON binding, response envelopes, and client-safe HTTP errors |
| `validx` | Struct validation with readable errors keyed by JSON field names |
| `authx` | Bcrypt passwords, bearer tokens, HMAC JWTs, identity context values, and simple auth middleware |
| `envx` | Environment lookup and parsing |
| `pgxutil` | pgx pool setup, health checks, and transaction handling |
| `logx` | Minimal slog setup for local text and production JSON logs |
| `testx` | JSON request, response decoding, and status assertion helpers |

## HTTP handlers

`httpx.Handler` adapts an error-returning function to `http.HandlerFunc`.
Successful JSON responses use a `data` envelope. Errors use an `error`
envelope, and unknown errors are returned to clients as a generic 500.

```go
type CreateUserRequest struct {
	Email string `json:"email" validate:"required,email"`
	Name  string `json:"name" validate:"required"`
}

func (in *CreateUserRequest) Validate() error {
	return validx.Struct(in)
}

func (s *Server) handleCreateUser(w http.ResponseWriter, r *http.Request) error {
	input, err := httpx.Bind[CreateUserRequest](r)
	if err != nil {
		return err
	}

	user, err := s.q.CreateUser(r.Context(), sqlc.CreateUserParams{
		Email: input.Email,
		Name:  input.Name,
	})
	if err != nil {
		return err
	}

	return httpx.Created(w, r, user)
}
```

Register it with chi without any special router integration:

```go
r.Post("/users", httpx.Handler(s.handleCreateUser))
```

Return explicit client errors when appropriate:

```go
return httpx.NotFound("user not found").WithCode("user_not_found")
```

Available response helpers are `JSON`, `OK`, `Created`, and `NoContent`.

## Validation

`validx.Struct` uses a shared validator instance. Validation errors can be
converted to readable field messages with `validx.Fields`; JSON tag names are
used instead of Go field names.

```go
type Input struct {
	Email string `json:"email" validate:"required,email"`
}

if err := validx.Struct(Input{}); err != nil {
	fields := validx.Fields(err)
	// fields["email"] == "is required"
}
```

Custom validator tags can be added with `validx.RegisterValidation`.

## Authentication

Passwords use bcrypt's safe default cost:

```go
hash, err := authx.HashPassword(password)
if err != nil {
	return err
}

if err := authx.CheckPassword(hash, password); err != nil {
	return httpx.Unauthorized("invalid credentials")
}
```

JWTs use HMAC SHA-256:

```go
claims := authx.Claims{
	UserID: user.ID,
	Roles:  []string{"admin"},
	RegisteredClaims: jwt.RegisteredClaims{
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)),
	},
}

token, err := authx.SignJWT(claims, []byte(envx.Required("JWT_SECRET")))
parsed, err := authx.ParseJWT(token, []byte(envx.Required("JWT_SECRET")))
```

`BearerToken` parses the `Authorization` header. `RequireAuth` accepts an
application-provided parser, and `RequireRole` checks roles stored in the
request context. Resource ownership and business authorization remain in the
application.

## Environment

```go
databaseURL := envx.Required("DATABASE_URL")
port := envx.Int("PORT", 8080)
debug := envx.Bool("DEBUG", false)
timeout := envx.Duration("HTTP_TIMEOUT", 10*time.Second)
environment := envx.String("APP_ENV", "development")
```

Use `LookupInt`, `LookupBool`, and `LookupDuration` when invalid values should
be handled explicitly rather than replaced with a fallback.

## Postgres

```go
pool, err := pgxutil.OpenPool(ctx, envx.Required("DATABASE_URL"))
if err != nil {
	return err
}
defer pool.Close()

err = pgxutil.Tx(ctx, pool, func(tx pgx.Tx) error {
	qtx := queries.WithTx(tx)
	if err := qtx.CreateUser(ctx, params); err != nil {
		return err
	}
	return qtx.CreateAuditLog(ctx, auditParams)
})
```

`pgxutil` is independent of sqlc; the application decides how a transaction is
passed to generated queries.

## Logging

```go
logger := logx.New(logx.Config{
	Env:   envx.String("APP_ENV", "development"),
	Level: envx.String("LOG_LEVEL", "info"),
})

logger.Info("server started", "port", 8080)
```

Local and development environments use slog's text handler. Other environments
use its JSON handler. Supported levels are `debug`, `info`, `warn`, and
`error`.

## HTTP tests

```go
func TestCreateUser(t *testing.T) {
	req := testx.JSONRequest(t, http.MethodPost, "/users", map[string]any{
		"email": "user@example.com",
		"name":  "Example",
	})
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	testx.AssertStatus(t, rec.Code, http.StatusCreated)
	body := testx.DecodeJSON[struct {
		Data User `json:"data"`
	}](t, rec.Body)

	if body.Data.Email != "user@example.com" {
		t.Fatalf("unexpected email: %s", body.Data.Email)
	}
}
```

## Design boundaries

These packages remove repeated plumbing only. Application startup, route
layout, dependency wiring, domain rules, database query ownership, and
authorization policy stay in each application.
