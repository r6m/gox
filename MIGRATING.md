# Migration Notes

## `envx` to `configx`

`envx` remains source-compatible but is deprecated. Replace scattered lookups:

```go
port := envx.Int("PORT", 8080)
databaseURL := envx.Required("DATABASE_URL")
```

with one typed configuration:

```go
type Config struct {
	Port        int    `env:"PORT" envDefault:"8080"`
	DatabaseURL string `env:"DATABASE_URL,required"`
}

cfg, err := configx.Load[Config]()
```

`configx` returns errors for missing, malformed, and invalid values. It does
not panic during normal loading.

## Raw HTTP responses

`httpx.JSON`, `OK`, and `Created` still produce the original `{"data": ...}`
envelope. They are deprecated because response shape is application policy.

Use:

```go
httpx.WriteJSON(w, http.StatusOK, value)
httpx.WriteNoContent(w)
```

`httpx.Handler` retains the original error envelope. Use
`HandlerWithErrorWriter` to map and serialize application errors.

## Password hashing

`authx.HashPassword` and `CheckPassword` remain available and use bcrypt, but
are deprecated. They preserve the original signatures, including
`CheckPassword(hash, password) error`.

The previous algorithm-neutral API was:

```go
hasher := authx.BcryptHasher{Cost: cost}
err := hasher.Verify(hash, password)
```

Construct and verify with the new mismatch-aware API:

```go
hasher, err := authx.NewBcryptHasher(authx.BcryptOptions{Cost: cost})
valid, err := hasher.Verify(password, hash)
```

An ordinary mismatch now returns `(false, nil)`. Malformed hashes and invalid
parameters return errors.

To create Argon2id hashes while accepting existing bcrypt hashes, keep format
detection and migration in application code:

```go
bcryptHasher, err := authx.NewBcryptHasher(authx.BcryptOptions{Cost: oldCost})
argonHasher, err := authx.NewArgon2idHasher(authx.Argon2idOptions{})

if strings.HasPrefix(storedHash, "$2") {
	valid, err := bcryptHasher.Verify(password, storedHash)
	if valid {
		replacement, err := argonHasher.Hash(password)
		// Persist replacement.
	}
}
```

`authx` intentionally does not provide a password service or account workflow.

## Validator registration

Package-level `validx.Struct` remains compatible. Package-level
`RegisterValidation` is deprecated because it mutates shared state. Use:

```go
validate := validx.New()
validate.RegisterValidation("tag", fn)
```

## Logging

The original `logx.Config{Env, Level}` continues to work. New fields configure
the writer, explicit text/JSON format, source locations, and
`slog.ReplaceAttr`.
