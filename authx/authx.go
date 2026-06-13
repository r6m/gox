// Package authx provides reusable authentication primitives.
//
// Password hashing supports bcrypt and Argon2id without imposing password
// strength policy. Bearer tokens, JWTs, identity context values, and simple
// authorization middleware remain independent of password storage.
package authx

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/golang-jwt/jwt/v5"
)

// Claims contains common application JWT claims.
type Claims struct {
	UserID string   `json:"user_id"`
	Roles  []string `json:"roles,omitempty"`
	jwt.RegisteredClaims
}

// BearerToken extracts a bearer token from the Authorization header.
func BearerToken(r *http.Request) (string, error) {
	parts := strings.Fields(r.Header.Get("Authorization"))
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") || parts[1] == "" {
		return "", errors.New("missing or malformed bearer token")
	}
	return parts[1], nil
}

// SignJWT signs claims using HMAC SHA-256.
func SignJWT(claims Claims, secret []byte) (string, error) {
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(secret)
}

// ParseJWT parses and validates an HMAC SHA-256 token.
func ParseJWT(token string, secret []byte) (*Claims, error) {
	claims := new(Claims)
	parsed, err := jwt.ParseWithClaims(token, claims, func(token *jwt.Token) (any, error) {
		if token.Method != jwt.SigningMethodHS256 {
			return nil, errors.New("unexpected JWT signing method")
		}
		return secret, nil
	}, jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}))
	if err != nil || !parsed.Valid {
		return nil, errors.Join(errors.New("invalid JWT"), err)
	}
	return claims, nil
}

type contextKey uint8

const (
	userIDKey contextKey = iota
	rolesKey
)

// WithUserID returns a context containing the authenticated user ID.
func WithUserID(ctx context.Context, userID string) context.Context {
	return context.WithValue(ctx, userIDKey, userID)
}

// UserID returns the authenticated user ID from a context.
func UserID(ctx context.Context) (string, bool) {
	value, ok := ctx.Value(userIDKey).(string)
	return value, ok
}

// WithRoles returns a context containing a defensive copy of roles.
func WithRoles(ctx context.Context, roles []string) context.Context {
	return context.WithValue(ctx, rolesKey, append([]string(nil), roles...))
}

// Roles returns a defensive copy of roles from a context.
func Roles(ctx context.Context) []string {
	roles, _ := ctx.Value(rolesKey).([]string)
	return append([]string(nil), roles...)
}

// RequireAuth authenticates a request and stores identity values in its context.
func RequireAuth(parse func(*http.Request) (string, []string, error)) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			userID, roles, err := parse(r)
			if err != nil || userID == "" {
				writeAuthError(w, http.StatusUnauthorized, "unauthorized")
				return
			}
			ctx := WithRoles(WithUserID(r.Context(), userID), roles)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// RequireRole permits requests containing the given context role.
func RequireRole(role string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			for _, candidate := range Roles(r.Context()) {
				if candidate == role {
					next.ServeHTTP(w, r)
					return
				}
			}
			writeAuthError(w, http.StatusForbidden, "forbidden")
		})
	}
}

func writeAuthError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_, _ = w.Write([]byte(`{"error":{"message":"` + message + `"}}`))
}
