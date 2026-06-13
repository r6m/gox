package authx

import (
	"errors"
	"fmt"

	"golang.org/x/crypto/bcrypt"
)

// PasswordHasher hashes and verifies passwords using a selected algorithm.
//
// Verify returns false with a nil error for an ordinary password mismatch.
// Malformed hashes and invalid parameters return an error.
type PasswordHasher interface {
	Hash(password string) (string, error)
	Verify(password, encodedHash string) (bool, error)
}

// BcryptOptions configures bcrypt password hashing.
type BcryptOptions struct {
	// Cost defaults to bcrypt.DefaultCost when zero.
	Cost int
}

// BcryptHasher implements PasswordHasher with bcrypt.
type BcryptHasher struct {
	cost int
}

// NewBcryptHasher creates a validated bcrypt password hasher.
func NewBcryptHasher(opts BcryptOptions) (*BcryptHasher, error) {
	cost := opts.Cost
	if cost == 0 {
		cost = bcrypt.DefaultCost
	}
	if cost < bcrypt.MinCost || cost > bcrypt.MaxCost {
		return nil, fmt.Errorf(
			"authx: bcrypt cost must be between %d and %d",
			bcrypt.MinCost,
			bcrypt.MaxCost,
		)
	}
	return &BcryptHasher{cost: cost}, nil
}

// Hash hashes a password using the configured bcrypt cost.
func (h *BcryptHasher) Hash(password string) (string, error) {
	if h == nil {
		return "", errors.New("authx: nil bcrypt hasher")
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), h.cost)
	if err != nil {
		return "", fmt.Errorf("authx: bcrypt hash: %w", err)
	}
	return string(hash), nil
}

// Verify checks a password against a standard bcrypt hash.
func (h *BcryptHasher) Verify(password, encodedHash string) (bool, error) {
	if h == nil {
		return false, errors.New("authx: nil bcrypt hasher")
	}
	err := bcrypt.CompareHashAndPassword([]byte(encodedHash), []byte(password))
	if err == nil {
		return true, nil
	}
	if errors.Is(err, bcrypt.ErrMismatchedHashAndPassword) {
		return false, nil
	}
	return false, fmt.Errorf("authx: bcrypt verify: %w", err)
}

// HashPassword hashes a password with bcrypt's default cost.
//
// Deprecated: use NewBcryptHasher and BcryptHasher.Hash.
func HashPassword(password string) (string, error) {
	hasher, err := NewBcryptHasher(BcryptOptions{})
	if err != nil {
		return "", err
	}
	return hasher.Hash(password)
}

// CheckPassword verifies a password against a bcrypt hash.
//
// Deprecated: use NewBcryptHasher and BcryptHasher.Verify.
func CheckPassword(hash string, password string) error {
	hasher, err := NewBcryptHasher(BcryptOptions{})
	if err != nil {
		return err
	}
	valid, err := hasher.Verify(password, hash)
	if err != nil {
		return err
	}
	if !valid {
		return bcrypt.ErrMismatchedHashAndPassword
	}
	return nil
}
