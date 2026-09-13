package auth

import (
	"errors"
	"strings"
	"testing"
)

func TestHashAndVerify(t *testing.T) {
	hash, err := HashPassword("correct horse battery staple")
	if err != nil {
		t.Fatalf("HashPassword() error = %v", err)
	}

	if !strings.HasPrefix(hash, "$argon2id$v=19$m=65536,t=3,p=2$") {
		t.Errorf("hash = %q, want the configured argon2id parameters", hash)
	}
	if err := VerifyPassword(hash, "correct horse battery staple"); err != nil {
		t.Errorf("VerifyPassword() on the right password = %v", err)
	}
	if err := VerifyPassword(hash, "Correct horse battery staple"); !errors.Is(err, ErrMismatch) {
		t.Errorf("VerifyPassword() on a wrong password = %v, want ErrMismatch", err)
	}
}

func TestHashesAreSalted(t *testing.T) {
	first, _ := HashPassword("same password")
	second, _ := HashPassword("same password")

	if first == second {
		t.Error("two hashes of the same password are identical; the salt is not random")
	}
}

func TestVerifyDistinguishesWrongPasswordFromBrokenHash(t *testing.T) {
	tests := map[string]string{
		"empty":             "",
		"not argon":         "$2y$10$abcdefghijklmnopqrstuv",
		"wrong field count": "$argon2id$v=19$m=65536,t=3,p=2$c2FsdA",
		"bad version":       "$argon2id$v=18$m=65536,t=3,p=2$c2FsdA$a2V5",
		"unreadable params": "$argon2id$v=19$m=abc,t=3,p=2$c2FsdA$a2V5",
		"bad base64 salt":   "$argon2id$v=19$m=65536,t=3,p=2$!!!$a2V5",
		"zeroed parameters": "$argon2id$v=19$m=0,t=0,p=0$c2FsdA$a2V5",
	}

	for name, hash := range tests {
		t.Run(name, func(t *testing.T) {
			err := VerifyPassword(hash, "anything")
			if err == nil {
				t.Fatal("VerifyPassword() succeeded on a malformed hash")
			}
			// A corrupt row is an operational problem, not a wrong password;
			// the caller needs to be able to tell them apart to log it.
			if errors.Is(err, ErrMismatch) {
				t.Errorf("error = %v, want something other than ErrMismatch", err)
			}
		})
	}
}

func TestNeedsRehash(t *testing.T) {
	current, _ := HashPassword("x")
	if NeedsRehash(current) {
		t.Error("a freshly made hash should not need rehashing")
	}

	weaker := "$argon2id$v=19$m=4096,t=1,p=1$c2FsdHNhbHRzYWx0$a2V5a2V5a2V5a2V5"
	if !NeedsRehash(weaker) {
		t.Error("a hash made with weaker parameters should be flagged for upgrade")
	}
	if !NeedsRehash("garbage") {
		t.Error("an unreadable hash should be flagged for upgrade")
	}
}
