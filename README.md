# gox

`gox` is a small infrastructure toolkit for Go backend projects. It is a
library, not an application framework: it does not bootstrap applications,
register routes, own database queries, define domain policy, or provide
dependency injection.

```sh
go get github.com/r6m/gox
```

## Packages

| Package | Purpose |
| --- | --- |
| `blobx` | Streaming filesystem and in-memory blob storage |
| `blobx/s3` | Optional AWS SDK v2 S3-compatible storage |
| `cachex` | Byte cache, memory implementation, JSON helpers, and test fake |
| `cachex/redis` | Optional go-redis cache implementation |
| `emailx` | SMTP delivery, MIME composition, templates, and test recorder |
| `configx` | Typed environment configuration |
| `fieldx` | Three-state fields for PATCH-style DTOs |
| `httpx` | JSON transport and error-returning handlers |
| `httpx/page` | Optional offset pagination |
| `httpx/middleware` | CORS, request IDs, recovery, and slog request logs |
| `validx` | Validator instances and readable JSON field errors |
| `authx` | Bearer, JWT, context, middleware, and password primitives |
| `pgxutil` | Pools, transactions, and nullable pgtype conversion |
| `logx` | Configurable slog construction |
| `testx` | HTTP and PostgreSQL test helpers |
| `retryx` | Pure exponential backoff calculations |
| `envx` | Deprecated environment lookup helpers |

## Blob Storage

`blobx.Store` streams object bodies and does not model directories:

```go
store, err := blobx.NewFileStore(blobx.FileOptions{
	Root: "./data/objects",
})

object, err := store.Put(ctx, "avatars/user-1.jpg", file, blobx.PutOptions{
	ContentType: "image/jpeg",
})

body, metadata, err := store.Get(ctx, object.Key)
if err != nil {
	if errors.Is(err, blobx.ErrNotFound) {
		// Handle a missing object.
	}
	return err
}
defer body.Close()
```

Putting an existing key overwrites it. Keys are canonical slash-separated
relative names; absolute paths, backslashes, empty segments, `.` and `..` are
rejected. Filesystem roots and S3 buckets belong to their concrete
implementations. No cross-provider atomicity guarantee is made.

S3 support is isolated from the core package:

```go
store, err := s3.New(awsS3Client, s3.Options{
	Bucket:    "uploads",
	KeyPrefix: "production",
})

url, err := store.PresignGet(ctx, "report.pdf", blobx.PresignOptions{
	Expires: 15 * time.Minute,
})
```

Presigning is the optional `blobx.Presigner` capability, not part of
`blobx.Store`. `blobx.NewMemoryStore` is a race-safe test implementation.

## Cache

`cachex.Cache` deliberately stores bytes:

```go
cache := cachex.NewMemory()
err := cache.Set(ctx, "session:"+id, encoded, 30*time.Minute)
data, err := cache.Get(ctx, "session:"+id)
if errors.Is(err, cachex.ErrMiss) {
	// Recompute the value.
}
```

Input and returned byte slices are copied. A zero TTL means no expiration; a
negative TTL deletes the key. Expired values are misses. Context cancellation
is returned as a context error, and provider failures are never converted into
`ErrMiss`.

JSON helpers stay outside the interface:

```go
err := cachex.SetJSON(ctx, cache, "user:"+id, user, time.Minute)
user, err := cachex.GetJSON[User](ctx, cache, "user:"+id)
```

Redis support is isolated in `cachex/redis`:

```go
cache, err := redis.New(redisClient, redis.Options{
	KeyPrefix: "myapp:",
})
```

`cachex.NewFake` provides deterministic application tests.

## Email

`emailx.Sender` accepts only fully rendered messages:

```go
sender, err := emailx.NewSMTPSender(emailx.SMTPOptions{
	Host:     "smtp.example.com",
	Port:     587,
	Username: username,
	Password: password,
})

result, err := sender.Send(ctx, emailx.Message{
	From:    emailx.Address{Name: "Example", Email: "hello@example.com"},
	To:      []emailx.Address{{Email: "user@example.com"}},
	Subject: "Welcome",
	Text:    "Welcome to Example.",
	HTML:    "<p>Welcome to Example.</p>",
})
```

Messages support To, CC, BCC, Reply-To, custom non-reserved headers, and
streaming attachments. An attachment provides a repeatable `Open` function;
the sender opens it once per delivery and always closes the returned reader.
BCC recipients are sent through the SMTP envelope and omitted from MIME
headers. SMTP requires implicit TLS or STARTTLS by default;
`AllowInsecure` is an explicit development-only escape hatch.

Templates are separate from transport and parsed during construction:

```go
renderer, err := emailx.NewTemplateRenderer(templateFS, []emailx.TemplateSpec{{
	Name:        "welcome",
	Message:     baseMessage,
	SubjectPath: "email/welcome.subject",
	TextPath:    "email/welcome.txt",
	HTMLPath:    "email/welcome.html",
}})

message, err := renderer.Render(ctx, "welcome", data)
result, err := sender.Send(ctx, message)
```

The renderer uses `text/template` for subject and text bodies and
`html/template` for HTML. It is safe for concurrent rendering.
`emailx.NewRecorder` records defensive message copies for tests.

## Configuration

`configx` wraps `github.com/caarlos0/env` instead of defining another
reflection-based configuration system.

```go
type Config struct {
	DatabaseURL string        `env:"DATABASE_URL,required"`
	Port        int           `env:"PORT" envDefault:"8080"`
	Timeout     time.Duration `env:"TIMEOUT" envDefault:"10s"`
}

func (c Config) Validate() error {
	if c.Port < 1 {
		return errors.New("port must be positive")
	}
	return nil
}

cfg, err := configx.Load[Config](configx.Options{Prefix: "MYAPP_"})
```

Types can implement `encoding.TextUnmarshaler`; `Options.FuncMap` handles types
that cannot. Keep derived defaults and application-specific validation in the
application.

## Partial Updates

`fieldx.Field[T]` distinguishes omitted, explicit `null`, and explicit values,
including zero values.

```go
type UpdateUser struct {
	Name fieldx.Field[string]  `json:"name"`
	Bio  fieldx.Field[*string] `json:"bio"`
}

var input UpdateUser
err := json.NewDecoder(r.Body).Decode(&input)

switch {
case !input.Name.IsSet():
	// Leave the database column unchanged.
case input.Name.IsNull():
	// Set the column to NULL, if the application permits it.
default:
	name, _ := input.Name.Value()
	// Set the column to name.
}
```

This package does not depend on sqlc or pgx. Translate each state into the
update query shape used by the application.

An unset value field marshals as `null`. `MarshalJSON` cannot make a containing
struct field disappear under `omitempty`; use `omitzero` on supported Go
versions. A pointer field can be omitted, but JSON `null` also decodes to a nil
pointer and therefore loses the omitted-versus-null distinction. Zero a reused
request DTO before decoding because `encoding/json` does not visit omitted
keys.

## HTTP

Raw transport helpers do not impose an envelope:

```go
func (s *Server) createUser(w http.ResponseWriter, r *http.Request) error {
	input, err := httpx.Bind[CreateUserRequest](r)
	if err != nil {
		return err
	}
	user, err := s.queries.CreateUser(r.Context(), params(input))
	if err != nil {
		return err
	}
	return httpx.WriteJSON(w, http.StatusCreated, user)
}
```

Applications choose error serialization:

```go
writer := httpx.ErrorWriterFunc(func(w http.ResponseWriter, r *http.Request, err error) {
	status, body := mapApplicationError(err)
	_ = httpx.WriteJSON(w, status, body)
})

r.Post("/users", httpx.HandlerWithErrorWriter(s.createUser, writer))
```

`httpx.Handler` and the envelope-producing `JSON`, `OK`, and `Created` helpers
remain for compatibility.

### Pagination

```go
params, err := page.Parse(r, page.Config{
	DefaultLimit: 20,
	MaxLimit:     100,
})
if err != nil {
	return httpx.BadRequest(err.Error())
}

result := page.Page[User]{
	Items: users, Offset: params.Offset, Limit: params.Limit, Total: total,
}
```

### Middleware

```go
handler := middleware.Recovery(middleware.RecoveryConfig{
	Logger: logger,
	WriteError: func(w http.ResponseWriter, r *http.Request, err error) {
		httpx.DefaultErrorWriter(w, r, err)
	},
})(
	middleware.RequestID(middleware.RequestIDConfig{})(
		middleware.RequestLogger(middleware.RequestLogConfig{
			Logger: logger,
			Enrich: func(r *http.Request) []slog.Attr {
				return []slog.Attr{slog.String("service", "api")}
			},
		})(router),
	),
)
```

`middleware.CORS` is separately configurable and optional.

## Validation

Prefer an independent validator when registering application tags:

```go
validate := validx.New()
err := validate.RegisterValidation("slug", validateSlug)
err = validate.Struct(input)
fields := validx.Fields(err)
```

The package-level `validx.Struct` remains for callers that need only built-in
tags. Field errors use JSON names.

## Authentication

JWT and bearer helpers remain generic:

```go
token, err := authx.BearerToken(r)
claims, err := authx.ParseJWT(token, secret)
signed, err := authx.SignJWT(authx.Claims{UserID: userID}, secret)
```

Use bcrypt directly when compatibility requires it:

```go
bcryptPasswords, err := authx.NewBcryptHasher(authx.BcryptOptions{
	Cost: 12,
})
hash, err := bcryptPasswords.Hash(password)
valid, err := bcryptPasswords.Verify(password, hash)
```

Use Argon2id directly for new systems:

```go
argonPasswords, err := authx.NewArgon2idHasher(authx.Argon2idOptions{})
hash, err := argonPasswords.Hash(password)
valid, err := argonPasswords.Verify(password, hash)
```

Zero Argon2id options use documented secure defaults: 64 MiB memory, three
iterations, two lanes, a 16-byte random salt, and a 32-byte derived key. Hashes
use the standard PHC string format.

For gradual bcrypt-to-Argon2id migration, select the hasher in application
code based on the stored hash prefix:

```go
var valid bool
switch {
case strings.HasPrefix(storedHash, "$2"):
	valid, err = bcryptPasswords.Verify(password, storedHash)
case strings.HasPrefix(storedHash, "$argon2id$"):
	valid, err = argonPasswords.Verify(password, storedHash)
default:
	return ErrUnsupportedPasswordHash
}
if err != nil {
	return err
}
if !valid {
	return ErrInvalidCredentials
}

if strings.HasPrefix(storedHash, "$2") {
	newHash, err := argonPasswords.Hash(password)
	if err != nil {
		return err
	}
	// Persist newHash after successful authentication.
}
```

Format detection and migration orchestration intentionally stay in the
application. Empty passwords are accepted by the hashing primitives;
password-strength and empty-password rules belong to the consuming
application. Applications can also supply another `PasswordHasher`.

Authorization ownership and resource policy stay outside this package.

## PostgreSQL

```go
pool, err := pgxutil.OpenPool(ctx, databaseURL)
value, err := pgxutil.TxValue(ctx, pool, func(tx pgx.Tx) (User, error) {
	return queries.WithTx(tx).CreateUser(ctx, params)
})

nickname := pgxutil.Text(optionalNickname) // *string -> pgtype.Text
optionalNickname = pgxutil.TextPtr(nickname)
```

Nullable converters follow one rule: a nil pointer maps to `Valid=false`.

## Logging

```go
logger := logx.New(logx.Config{
	Writer:      os.Stdout,
	Format:      logx.FormatJSON,
	Level:       "info",
	AddSource:   true,
	ReplaceAttr: replaceSensitiveAttributes,
})
```

Leaving `Format` empty preserves environment-based behavior: local/dev uses
text and other environments use JSON.

## Testing

```go
req := testx.JSONRequest(t, http.MethodPost, "/users", input)
rec := testx.Serve(t, handler, req)
testx.AssertStatus(t, rec.Code, http.StatusCreated)
body := testx.DecodeJSON[User](t, rec.Body)
```

PostgreSQL helpers do not embed schemas:

```go
pool := testx.PreparePostgres(t, databaseURL, runMigrations)
testx.CleanupPostgres(t, pool, truncateApplicationTables)
tx := testx.PostgresTx(t, pool) // rolled back by t.Cleanup
```

`testx.OpenPostgresEnv(t, "TEST_DATABASE_URL")` skips when the variable is not
set.

## Retry Backoff

```go
delay, err := retryx.Delay(retryx.Config{
	BaseDelay: 100 * time.Millisecond,
	Factor:    2,
	MaxDelay:  5 * time.Second,
	Jitter:    0.2,
}, attempt, rand.Float64)
```

`retryx` only calculates durations. It does not sleep, schedule, persist, or
run work.

See [MIGRATING.md](MIGRATING.md) for compatibility and deprecation notes.
