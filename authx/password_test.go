package authx

import (
	"encoding/base64"
	"strings"
	"testing"

	"golang.org/x/crypto/argon2"
	"golang.org/x/crypto/bcrypt"
)

var testArgon2idOptions = Argon2idOptions{
	Memory:      8 * 1024,
	Iterations:  1,
	Parallelism: 1,
	SaltLength:  8,
	KeyLength:   16,
}

func TestBcryptHashAndVerify(t *testing.T) {
	hasher, err := NewBcryptHasher(BcryptOptions{Cost: bcrypt.MinCost})
	if err != nil {
		t.Fatal(err)
	}
	var passwordHasher PasswordHasher = hasher
	for _, password := range []string{"password", "", "گذرواژه-🔐"} {
		hash, err := passwordHasher.Hash(password)
		if err != nil {
			t.Fatal(err)
		}
		valid, err := passwordHasher.Verify(password, hash)
		if err != nil || !valid {
			t.Fatalf("verify %q: valid=%v err=%v", password, valid, err)
		}
		valid, err = passwordHasher.Verify(password+"wrong", hash)
		if err != nil || valid {
			t.Fatalf("mismatch %q: valid=%v err=%v", password, valid, err)
		}
	}
}

func TestBcryptMalformedHashAndOptions(t *testing.T) {
	hasher, err := NewBcryptHasher(BcryptOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := hasher.Verify("password", "$2b$broken"); err == nil {
		t.Fatal("expected malformed bcrypt error")
	}
	for _, cost := range []int{bcrypt.MinCost - 1, bcrypt.MaxCost + 1} {
		if _, err := NewBcryptHasher(BcryptOptions{Cost: cost}); err == nil {
			t.Fatalf("expected cost %d to be rejected", cost)
		}
	}
	if hasher.cost != bcrypt.DefaultCost {
		t.Fatalf("unexpected default cost: %d", hasher.cost)
	}
}

func TestArgon2idHashAndVerify(t *testing.T) {
	hasher, err := NewArgon2idHasher(testArgon2idOptions)
	if err != nil {
		t.Fatal(err)
	}
	var passwordHasher PasswordHasher = hasher
	for _, password := range []string{"password", "", "گذرواژه-🔐"} {
		hash, err := passwordHasher.Hash(password)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.HasPrefix(hash, "$argon2id$v=19$") {
			t.Fatalf("unexpected PHC string: %s", hash)
		}
		valid, err := passwordHasher.Verify(password, hash)
		if err != nil || !valid {
			t.Fatalf("verify %q: valid=%v err=%v", password, valid, err)
		}
		valid, err = passwordHasher.Verify(password+"wrong", hash)
		if err != nil || valid {
			t.Fatalf("mismatch %q: valid=%v err=%v", password, valid, err)
		}
	}
}

func TestArgon2idUsesUniqueSalts(t *testing.T) {
	hasher, err := NewArgon2idHasher(testArgon2idOptions)
	if err != nil {
		t.Fatal(err)
	}
	first, err := hasher.Hash("same")
	if err != nil {
		t.Fatal(err)
	}
	second, err := hasher.Hash("same")
	if err != nil {
		t.Fatal(err)
	}
	if first == second {
		t.Fatal("identical passwords produced identical hashes")
	}
}

func TestArgon2idDefaults(t *testing.T) {
	hasher, err := NewArgon2idHasher(Argon2idOptions{})
	if err != nil {
		t.Fatal(err)
	}
	want := Argon2idOptions{
		Memory:      defaultArgon2idMemory,
		Iterations:  defaultArgon2idIterations,
		Parallelism: defaultArgon2idParallelism,
		SaltLength:  defaultArgon2idSaltLength,
		KeyLength:   defaultArgon2idKeyLength,
	}
	if hasher.options != want {
		t.Fatalf("unexpected defaults: %#v", hasher.options)
	}
}

func TestArgon2idRejectsMalformedHashes(t *testing.T) {
	hasher, err := NewArgon2idHasher(testArgon2idOptions)
	if err != nil {
		t.Fatal(err)
	}
	hashes := []string{
		"$argon2id$v=18$m=8192,t=1,p=1$c2FsdHNhbHQ$a2V5a2V5a2V5a2V5a2V5aw",
		"$argon2id$v=19$m=8192,t=1$c2FsdHNhbHQ$a2V5a2V5a2V5a2V5a2V5aw",
		"$argon2id$v=19$m=x,t=1,p=1$c2FsdHNhbHQ$a2V5a2V5a2V5a2V5a2V5aw",
		"$argon2id$v=19$m=8192,t=1,p=1$***$a2V5a2V5a2V5a2V5a2V5aw",
		"$argon2id$v=19$m=8192,t=1,p=1$c2FsdHNhbHQ$***",
		"$argon2id$v=19$m=8192,t=1,p=1$c2FsdHNhbHQ$YQ",
		"$argon2id$v=19$m=262145,t=1,p=1$c2FsdHNhbHQ$a2V5a2V5a2V5a2V5a2V5aw",
		"$argon2id$v=19$m=8192,t=11,p=1$c2FsdHNhbHQ$a2V5a2V5a2V5a2V5a2V5aw",
		"$argon2id$v=19$m=8192,t=1,p=17$c2FsdHNhbHQ$a2V5a2V5a2V5a2V5a2V5aw",
		"$argon2id$v=19$m=8192,t=1,p=1$" +
			base64.RawStdEncoding.EncodeToString(make([]byte, maxArgon2idSaltLength+1)) +
			"$a2V5a2V5a2V5a2V5a2V5aw",
		"$argon2id$v=19$m=8192,t=1,p=1$c2FsdHNhbHQ$" +
			base64.RawStdEncoding.EncodeToString(make([]byte, maxArgon2idKeyLength+1)),
	}
	for _, hash := range hashes {
		if _, err := hasher.Verify("password", hash); err == nil {
			t.Fatalf("expected malformed hash error for %q", hash)
		}
	}
}

func TestArgon2idOptionValidation(t *testing.T) {
	tests := []Argon2idOptions{
		{Memory: maxArgon2idMemory + 1},
		{Iterations: maxArgon2idIterations + 1},
		{Parallelism: maxArgon2idParallelism + 1},
		{SaltLength: maxArgon2idSaltLength + 1},
		{KeyLength: maxArgon2idKeyLength + 1},
		{Memory: 8, Parallelism: 2},
	}
	for _, opts := range tests {
		if _, err := NewArgon2idHasher(opts); err == nil {
			t.Fatalf("expected options to be rejected: %#v", opts)
		}
	}
}

func TestArgon2VersionConstant(t *testing.T) {
	if argon2.Version != 19 {
		t.Fatalf("unexpected Argon2 version: %d", argon2.Version)
	}
}
