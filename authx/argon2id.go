package authx

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"golang.org/x/crypto/argon2"
)

const (
	defaultArgon2idMemory      uint32 = 64 * 1024
	defaultArgon2idIterations  uint32 = 3
	defaultArgon2idParallelism uint8  = 2
	defaultArgon2idSaltLength  uint32 = 16
	defaultArgon2idKeyLength   uint32 = 32

	maxArgon2idMemory      uint32 = 256 * 1024
	maxArgon2idIterations  uint32 = 10
	maxArgon2idParallelism uint8  = 16
	maxArgon2idSaltLength  uint32 = 64
	maxArgon2idKeyLength   uint32 = 64
)

// Argon2idOptions configures Argon2id password hashing. Memory is measured in
// KiB. Zero fields use secure package defaults.
type Argon2idOptions struct {
	Memory      uint32
	Iterations  uint32
	Parallelism uint8
	SaltLength  uint32
	KeyLength   uint32
}

// Argon2idHasher implements PasswordHasher using Argon2id and PHC strings.
type Argon2idHasher struct {
	options Argon2idOptions
}

// NewArgon2idHasher creates a validated Argon2id password hasher.
func NewArgon2idHasher(opts Argon2idOptions) (*Argon2idHasher, error) {
	opts = argon2idDefaults(opts)
	if err := validateArgon2idOptions(opts); err != nil {
		return nil, err
	}
	return &Argon2idHasher{options: opts}, nil
}

// Hash hashes a password and returns a standard Argon2id PHC string.
func (h *Argon2idHasher) Hash(password string) (string, error) {
	if h == nil {
		return "", errors.New("authx: nil Argon2id hasher")
	}
	salt := make([]byte, h.options.SaltLength)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("authx: generate Argon2id salt: %w", err)
	}
	key := argon2.IDKey(
		[]byte(password),
		salt,
		h.options.Iterations,
		h.options.Memory,
		h.options.Parallelism,
		h.options.KeyLength,
	)
	encoding := base64.RawStdEncoding
	return fmt.Sprintf(
		"$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version,
		h.options.Memory,
		h.options.Iterations,
		h.options.Parallelism,
		encoding.EncodeToString(salt),
		encoding.EncodeToString(key),
	), nil
}

// Verify checks a password against an Argon2id PHC string.
func (h *Argon2idHasher) Verify(password, encodedHash string) (bool, error) {
	if h == nil {
		return false, errors.New("authx: nil Argon2id hasher")
	}
	parsed, err := parseArgon2idHash(encodedHash)
	if err != nil {
		return false, err
	}
	key := argon2.IDKey(
		[]byte(password),
		parsed.salt,
		parsed.options.Iterations,
		parsed.options.Memory,
		parsed.options.Parallelism,
		uint32(len(parsed.key)),
	)
	return subtle.ConstantTimeCompare(key, parsed.key) == 1, nil
}

type parsedArgon2id struct {
	options Argon2idOptions
	salt    []byte
	key     []byte
}

func parseArgon2idHash(encodedHash string) (parsedArgon2id, error) {
	parts := strings.Split(encodedHash, "$")
	if len(parts) != 6 || parts[0] != "" || parts[1] != "argon2id" {
		return parsedArgon2id{}, errors.New("authx: malformed Argon2id PHC string")
	}
	version, err := parsePHCValue(parts[2], "v")
	if err != nil {
		return parsedArgon2id{}, err
	}
	if version != argon2.Version {
		return parsedArgon2id{}, fmt.Errorf("authx: unsupported Argon2 version %d", version)
	}
	options, err := parseArgon2idParameters(parts[3])
	if err != nil {
		return parsedArgon2id{}, err
	}

	encoding := base64.RawStdEncoding.Strict()
	salt, err := encoding.DecodeString(parts[4])
	if err != nil {
		return parsedArgon2id{}, fmt.Errorf("authx: invalid Argon2id salt: %w", err)
	}
	key, err := encoding.DecodeString(parts[5])
	if err != nil {
		return parsedArgon2id{}, fmt.Errorf("authx: invalid Argon2id key: %w", err)
	}
	options.SaltLength = uint32(len(salt))
	options.KeyLength = uint32(len(key))
	if err := validateArgon2idOptions(options); err != nil {
		return parsedArgon2id{}, fmt.Errorf("authx: invalid Argon2id hash parameters: %w", err)
	}
	return parsedArgon2id{options: options, salt: salt, key: key}, nil
}

func parseArgon2idParameters(value string) (Argon2idOptions, error) {
	parts := strings.Split(value, ",")
	if len(parts) != 3 {
		return Argon2idOptions{}, errors.New("authx: malformed Argon2id parameters")
	}
	memory, err := parsePHCValue(parts[0], "m")
	if err != nil {
		return Argon2idOptions{}, err
	}
	iterations, err := parsePHCValue(parts[1], "t")
	if err != nil {
		return Argon2idOptions{}, err
	}
	parallelism, err := parsePHCValue(parts[2], "p")
	if err != nil {
		return Argon2idOptions{}, err
	}
	if memory > uint64(^uint32(0)) ||
		iterations > uint64(^uint32(0)) ||
		parallelism > uint64(^uint8(0)) {
		return Argon2idOptions{}, errors.New("authx: Argon2id parameter overflow")
	}
	return Argon2idOptions{
		Memory:      uint32(memory),
		Iterations:  uint32(iterations),
		Parallelism: uint8(parallelism),
	}, nil
}

func parsePHCValue(value, name string) (uint64, error) {
	prefix := name + "="
	if !strings.HasPrefix(value, prefix) || len(value) == len(prefix) {
		return 0, fmt.Errorf("authx: malformed Argon2id %s parameter", name)
	}
	number, err := strconv.ParseUint(strings.TrimPrefix(value, prefix), 10, 64)
	if err != nil {
		return 0, fmt.Errorf("authx: malformed Argon2id %s parameter: %w", name, err)
	}
	return number, nil
}

func argon2idDefaults(opts Argon2idOptions) Argon2idOptions {
	if opts.Memory == 0 {
		opts.Memory = defaultArgon2idMemory
	}
	if opts.Iterations == 0 {
		opts.Iterations = defaultArgon2idIterations
	}
	if opts.Parallelism == 0 {
		opts.Parallelism = defaultArgon2idParallelism
	}
	if opts.SaltLength == 0 {
		opts.SaltLength = defaultArgon2idSaltLength
	}
	if opts.KeyLength == 0 {
		opts.KeyLength = defaultArgon2idKeyLength
	}
	return opts
}

func validateArgon2idOptions(opts Argon2idOptions) error {
	if opts.Memory < 8*uint32(opts.Parallelism) {
		return errors.New("authx: Argon2id memory must be at least 8 KiB per parallelism lane")
	}
	if opts.Memory > maxArgon2idMemory {
		return fmt.Errorf("authx: Argon2id memory exceeds %d KiB", maxArgon2idMemory)
	}
	if opts.Iterations < 1 || opts.Iterations > maxArgon2idIterations {
		return fmt.Errorf("authx: Argon2id iterations must be between 1 and %d", maxArgon2idIterations)
	}
	if opts.Parallelism < 1 || opts.Parallelism > maxArgon2idParallelism {
		return fmt.Errorf("authx: Argon2id parallelism must be between 1 and %d", maxArgon2idParallelism)
	}
	if opts.SaltLength < 8 || opts.SaltLength > maxArgon2idSaltLength {
		return fmt.Errorf("authx: Argon2id salt length must be between 8 and %d", maxArgon2idSaltLength)
	}
	if opts.KeyLength < 16 || opts.KeyLength > maxArgon2idKeyLength {
		return fmt.Errorf("authx: Argon2id key length must be between 16 and %d", maxArgon2idKeyLength)
	}
	return nil
}
