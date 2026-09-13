package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

// Role is a user's permission level.
type Role string

const (
	RoleAdmin  Role = "admin"
	RoleMember Role = "member"
	RoleKid    Role = "kid"
)

// Valid reports whether r is a role Attic knows.
func (r Role) Valid() bool {
	switch r {
	case RoleAdmin, RoleMember, RoleKid:
		return true
	}
	return false
}

// Token purposes. Media tokens exist because a player cannot rewrite an
// Authorization header mid-stream: they ride in `?token=` instead, and are
// accepted *only* by media handlers.
const (
	purposeAccess = "access"
	purposeMedia  = "media"
)

// Claims is the payload of an Attic access or media token.
type Claims struct {
	jwt.RegisteredClaims
	Role    Role   `json:"role"`
	Purpose string `json:"purpose"`
}

// UserID returns the subject as a UUID.
func (c *Claims) UserID() (uuid.UUID, error) { return uuid.Parse(c.Subject) }

// IsMedia reports whether these claims came from a media token.
func (c *Claims) IsMedia() bool { return c.Purpose == purposeMedia }

var (
	// ErrInvalidToken covers every reason a token was rejected. Callers must
	// not distinguish them to clients: "expired" versus "forged" is an oracle.
	ErrInvalidToken = errors.New("auth: invalid token")

	// ErrExpiredToken lets the API return a distinct code so the app knows to
	// refresh rather than sign out.
	ErrExpiredToken = errors.New("auth: token expired")
)

// Tokens issues and verifies Attic's tokens.
type Tokens struct {
	secret     []byte
	accessTTL  time.Duration
	refreshTTL time.Duration
	mediaTTL   time.Duration

	// now is swappable in tests.
	now func() time.Time
}

// NewTokens builds a token issuer. The secret is the configured JWT secret.
func NewTokens(secret []byte, accessTTL, refreshTTL, mediaTTL time.Duration) *Tokens {
	return &Tokens{
		secret:     secret,
		accessTTL:  accessTTL,
		refreshTTL: refreshTTL,
		mediaTTL:   mediaTTL,
		now:        time.Now,
	}
}

// SetClock replaces the time source. Test-only.
func (t *Tokens) SetClock(now func() time.Time) { t.now = now }

// AccessTTL is how long issued access tokens live.
func (t *Tokens) AccessTTL() time.Duration { return t.accessTTL }

// RefreshTTL is how long issued refresh tokens live.
func (t *Tokens) RefreshTTL() time.Duration { return t.refreshTTL }

// MediaTTL is how long issued media tokens live.
func (t *Tokens) MediaTTL() time.Duration { return t.mediaTTL }

// IssueAccess mints a short-lived bearer token for the API.
func (t *Tokens) IssueAccess(userID uuid.UUID, role Role) (string, time.Time, error) {
	return t.issue(userID, role, purposeAccess, t.accessTTL)
}

// IssueMedia mints a token for `?token=` URLs on media endpoints.
func (t *Tokens) IssueMedia(userID uuid.UUID, role Role) (string, time.Time, error) {
	return t.issue(userID, role, purposeMedia, t.mediaTTL)
}

func (t *Tokens) issue(userID uuid.UUID, role Role, purpose string, ttl time.Duration) (string, time.Time, error) {
	now := t.now()
	expires := now.Add(ttl)

	claims := Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID.String(),
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(expires),
			ID:        uuid.NewString(),
			Issuer:    "attic",
		},
		Role:    role,
		Purpose: purpose,
	}

	signed, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(t.secret)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("auth: sign token: %w", err)
	}
	return signed, expires, nil
}

// Parse verifies a token's signature and expiry and returns its claims.
// It does not check the purpose; callers decide which purposes they accept.
func (t *Tokens) Parse(token string) (*Claims, error) {
	parsed, err := jwt.ParseWithClaims(token, &Claims{},
		func(tok *jwt.Token) (any, error) {
			// Pin the algorithm: without this, a token signed with "none" or
			// with the public key as an HMAC secret would be accepted.
			if _, ok := tok.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("auth: unexpected signing method %v", tok.Header["alg"])
			}
			return t.secret, nil
		},
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
		jwt.WithIssuer("attic"),
		jwt.WithTimeFunc(t.now),
	)
	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return nil, ErrExpiredToken
		}
		return nil, ErrInvalidToken
	}

	claims, ok := parsed.Claims.(*Claims)
	if !ok || !parsed.Valid {
		return nil, ErrInvalidToken
	}
	if claims.Purpose != purposeAccess && claims.Purpose != purposeMedia {
		return nil, ErrInvalidToken
	}
	if !claims.Role.Valid() {
		return nil, ErrInvalidToken
	}
	return claims, nil
}

// RefreshToken is an opaque secret handed to a device once. Only its hash is
// stored, so the plaintext exists solely in the client's secure storage.
type RefreshToken struct {
	Plaintext string
	Hash      []byte
	ExpiresAt time.Time
}

// NewRefreshToken generates a 256-bit opaque token.
func (t *Tokens) NewRefreshToken() (RefreshToken, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return RefreshToken{}, fmt.Errorf("auth: read random: %w", err)
	}
	plaintext := base64.RawURLEncoding.EncodeToString(raw)

	return RefreshToken{
		Plaintext: plaintext,
		Hash:      HashRefreshToken(plaintext),
		ExpiresAt: t.now().Add(t.refreshTTL),
	}, nil
}

// HashRefreshToken is the one-way function used to store and look up refresh
// tokens. SHA-256 rather than argon2id on purpose: the token is 256 bits of
// entropy, so there is nothing to brute-force, and lookup happens on every
// refresh.
func HashRefreshToken(plaintext string) []byte {
	sum := sha256.Sum256([]byte(plaintext))
	return sum[:]
}
