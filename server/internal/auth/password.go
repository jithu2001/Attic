// Package auth holds Attic's password hashing, token issuing and the HTTP
// middleware that enforces both.
package auth

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"

	"golang.org/x/crypto/argon2"
)

// Argon2id parameters. These are the values Attic ships with; a hash records
// the parameters it was made with, so raising them later still verifies old
// passwords (and Needs­Rehash reports which ones to upgrade).
const (
	argonTime    uint32 = 3
	argonMemory  uint32 = 64 * 1024 // KiB, i.e. 64 MB
	argonThreads uint8  = 2
	argonSaltLen uint32 = 16
	argonKeyLen  uint32 = 32
)

// ErrMismatch is returned when a password does not match its hash.
var ErrMismatch = errors.New("auth: password does not match")

type argonParams struct {
	time    uint32
	memory  uint32
	threads uint8
}

// HashPassword returns a PHC-formatted argon2id hash:
//
//	$argon2id$v=19$m=65536,t=3,p=2$<salt>$<key>
func HashPassword(password string) (string, error) {
	salt := make([]byte, argonSaltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("auth: read salt: %w", err)
	}

	key := argon2.IDKey([]byte(password), salt, argonTime, argonMemory, argonThreads, argonKeyLen)

	return fmt.Sprintf(
		"$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version, argonMemory, argonTime, argonThreads,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(key),
	), nil
}

// VerifyPassword reports whether password matches encoded. It returns
// ErrMismatch for a wrong password and a different error for a malformed hash,
// so callers can tell "wrong password" from "corrupt row".
func VerifyPassword(encoded, password string) error {
	params, salt, want, err := decodeHash(encoded)
	if err != nil {
		return err
	}

	got := argon2.IDKey([]byte(password), salt, params.time, params.memory, params.threads, uint32(len(want)))
	if subtle.ConstantTimeCompare(got, want) != 1 {
		return ErrMismatch
	}
	return nil
}

// NeedsRehash reports whether encoded was made with weaker parameters than the
// ones Attic currently uses, so the caller can transparently upgrade it on the
// next successful login.
func NeedsRehash(encoded string) bool {
	params, _, _, err := decodeHash(encoded)
	if err != nil {
		return true
	}
	return params.time < argonTime || params.memory < argonMemory || params.threads < argonThreads
}

func decodeHash(encoded string) (argonParams, []byte, []byte, error) {
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 || parts[1] != "argon2id" {
		return argonParams{}, nil, nil, errors.New("auth: not an argon2id hash")
	}

	var version int
	if _, err := fmt.Sscanf(parts[2], "v=%d", &version); err != nil {
		return argonParams{}, nil, nil, fmt.Errorf("auth: unreadable hash version: %w", err)
	}
	if version != argon2.Version {
		return argonParams{}, nil, nil, fmt.Errorf("auth: unsupported argon2 version %d", version)
	}

	var p argonParams
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &p.memory, &p.time, &p.threads); err != nil {
		return argonParams{}, nil, nil, fmt.Errorf("auth: unreadable hash parameters: %w", err)
	}

	salt, err := base64.RawStdEncoding.Strict().DecodeString(parts[4])
	if err != nil {
		return argonParams{}, nil, nil, fmt.Errorf("auth: unreadable salt: %w", err)
	}
	key, err := base64.RawStdEncoding.Strict().DecodeString(parts[5])
	if err != nil {
		return argonParams{}, nil, nil, fmt.Errorf("auth: unreadable key: %w", err)
	}
	if p.threads == 0 || p.memory == 0 || p.time == 0 || len(salt) == 0 || len(key) == 0 {
		return argonParams{}, nil, nil, errors.New("auth: hash has zeroed parameters")
	}

	return p, salt, key, nil
}
