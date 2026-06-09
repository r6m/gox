package authx

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func TestPassword(t *testing.T) {
	hash, err := HashPassword("secret")
	if err != nil || CheckPassword(hash, "secret") != nil {
		t.Fatalf("password check failed: %v", err)
	}
	if CheckPassword(hash, "wrong") == nil {
		t.Fatal("wrong password accepted")
	}
}

func TestBearerToken(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer abc")
	token, err := BearerToken(req)
	if err != nil || token != "abc" {
		t.Fatalf("unexpected result: %q %v", token, err)
	}
	req.Header.Set("Authorization", "Basic abc")
	if _, err := BearerToken(req); err == nil {
		t.Fatal("malformed header accepted")
	}
}

func TestJWT(t *testing.T) {
	claims := Claims{
		UserID: "user-1",
		Roles:  []string{"admin"},
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Minute)),
		},
	}
	token, err := SignJWT(claims, []byte("secret"))
	if err != nil {
		t.Fatal(err)
	}
	got, err := ParseJWT(token, []byte("secret"))
	if err != nil || got.UserID != claims.UserID || len(got.Roles) != 1 {
		t.Fatalf("unexpected claims: %#v %v", got, err)
	}
}

func TestContextHelpers(t *testing.T) {
	ctx := WithRoles(WithUserID(context.Background(), "user-1"), []string{"admin"})
	if id, ok := UserID(ctx); !ok || id != "user-1" {
		t.Fatalf("unexpected user: %q %v", id, ok)
	}
	roles := Roles(ctx)
	roles[0] = "changed"
	if Roles(ctx)[0] != "admin" {
		t.Fatal("roles were not copied")
	}
}

func TestMiddleware(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id, _ := UserID(r.Context())
		_, _ = w.Write([]byte(id))
	})
	handler := RequireAuth(func(*http.Request) (string, []string, error) {
		return "user-1", []string{"admin"}, nil
	})(RequireRole("admin")(next))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	if rec.Code != http.StatusOK || rec.Body.String() != "user-1" {
		t.Fatalf("unexpected response: %d %s", rec.Code, rec.Body.String())
	}

	denied := RequireAuth(func(*http.Request) (string, []string, error) {
		return "", nil, errors.New("bad token")
	})(next)
	rec = httptest.NewRecorder()
	denied.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("unexpected status: %d", rec.Code)
	}
}
