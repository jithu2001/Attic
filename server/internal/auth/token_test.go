package auth

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

var testSecret = []byte("0123456789abcdef0123456789abcdef")

func newTokens() *Tokens {
	return NewTokens(testSecret, 15*time.Minute, 90*24*time.Hour, 6*time.Hour)
}

func TestAccessTokenRoundTrips(t *testing.T) {
	tokens := newTokens()
	userID := uuid.New()

	token, expiresAt, err := tokens.IssueAccess(userID, RoleAdmin)
	if err != nil {
		t.Fatalf("IssueAccess() error = %v", err)
	}
	if time.Until(expiresAt) > 16*time.Minute {
		t.Errorf("access token lives until %v, want ~15 minutes", expiresAt)
	}

	claims, err := tokens.Parse(token)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if got, _ := claims.UserID(); got != userID {
		t.Errorf("subject = %v, want %v", got, userID)
	}
	if claims.Role != RoleAdmin {
		t.Errorf("role = %q, want admin", claims.Role)
	}
	if claims.IsMedia() {
		t.Error("an access token reports itself as a media token")
	}
}

func TestExpiredTokenIsReportedDistinctly(t *testing.T) {
	tokens := newTokens()
	token, _, err := tokens.IssueAccess(uuid.New(), RoleMember)
	if err != nil {
		t.Fatalf("IssueAccess() error = %v", err)
	}

	// The app needs to tell "refresh me" from "sign in again".
	tokens.SetClock(func() time.Time { return time.Now().Add(time.Hour) })
	if _, err := tokens.Parse(token); !errors.Is(err, ErrExpiredToken) {
		t.Fatalf("Parse() = %v, want ErrExpiredToken", err)
	}
}

func TestTokenSignedWithAnotherSecretIsRejected(t *testing.T) {
	token, _, err := NewTokens([]byte("aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"), time.Minute, time.Hour, time.Hour).
		IssueAccess(uuid.New(), RoleAdmin)
	if err != nil {
		t.Fatalf("IssueAccess() error = %v", err)
	}

	if _, err := newTokens().Parse(token); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("Parse() = %v, want ErrInvalidToken", err)
	}
}

func TestUnsignedTokenIsRejected(t *testing.T) {
	// The classic JWT forgery: claim alg "none" and hope the server believes
	// the header over its own configuration.
	claims := Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   uuid.NewString(),
			Issuer:    "attic",
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
		},
		Role:    RoleAdmin,
		Purpose: "access",
	}
	forged, err := jwt.NewWithClaims(jwt.SigningMethodNone, claims).SignedString(jwt.UnsafeAllowNoneSignatureType)
	if err != nil {
		t.Fatalf("could not build the forged token: %v", err)
	}

	if _, err := newTokens().Parse(forged); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("Parse() accepted an unsigned token: %v", err)
	}
}

func TestRefreshTokensAreOpaqueAndHashed(t *testing.T) {
	tokens := newTokens()

	first, err := tokens.NewRefreshToken()
	if err != nil {
		t.Fatalf("NewRefreshToken() error = %v", err)
	}
	second, err := tokens.NewRefreshToken()
	if err != nil {
		t.Fatalf("NewRefreshToken() error = %v", err)
	}

	if first.Plaintext == second.Plaintext {
		t.Error("two refresh tokens are identical")
	}
	if len(first.Plaintext) < 40 {
		t.Errorf("refresh token is only %d characters; want 256 bits of entropy", len(first.Plaintext))
	}
	if len(first.Hash) != 32 {
		t.Errorf("hash is %d bytes, want a 32-byte SHA-256", len(first.Hash))
	}
	if string(first.Hash) == first.Plaintext {
		t.Error("the stored hash is the plaintext")
	}
	// Lookup depends on the hash being deterministic.
	if string(HashRefreshToken(first.Plaintext)) != string(first.Hash) {
		t.Error("HashRefreshToken is not deterministic")
	}
}

func TestAuthenticateAcceptsOnlyTheRightCredential(t *testing.T) {
	tokens := newTokens()
	authn := NewAuthenticator(tokens)
	userID := uuid.New()

	access, _, _ := tokens.IssueAccess(userID, RoleMember)
	mediaToken, _, _ := tokens.IssueMedia(userID, RoleMember)

	tests := []struct {
		name       string
		setup      func(*http.Request)
		opts       AuthenticateOptions
		wantStatus int // 0 means the request should be allowed
	}{
		{
			name:  "bearer access token on an API route",
			setup: func(r *http.Request) { r.Header.Set("Authorization", "Bearer "+access) },
		},
		{
			name:       "no credentials",
			setup:      func(*http.Request) {},
			wantStatus: http.StatusUnauthorized,
		},
		{
			name:       "wrong scheme",
			setup:      func(r *http.Request) { r.Header.Set("Authorization", "Basic "+access) },
			wantStatus: http.StatusUnauthorized,
		},
		{
			name:       "media token as a bearer on an API route",
			setup:      func(r *http.Request) { r.Header.Set("Authorization", "Bearer "+mediaToken) },
			wantStatus: http.StatusUnauthorized,
		},
		{
			name:  "media token in the query on a media route",
			setup: func(r *http.Request) { r.URL.RawQuery = "token=" + mediaToken },
			opts:  AuthenticateOptions{AllowMediaToken: true},
		},
		{
			name:       "media token in the query on an API route",
			setup:      func(r *http.Request) { r.URL.RawQuery = "token=" + mediaToken },
			wantStatus: http.StatusUnauthorized,
		},
		{
			name:       "access token in the query on a media route",
			setup:      func(r *http.Request) { r.URL.RawQuery = "token=" + access },
			opts:       AuthenticateOptions{AllowMediaToken: true},
			wantStatus: http.StatusUnauthorized,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/api/v1/anything", nil)
			tc.setup(req)

			id, denial := authn.Authenticate(req, tc.opts)
			if tc.wantStatus == 0 {
				if denial != nil {
					t.Fatalf("denied with %d %s, want success", denial.Status, denial.Message)
				}
				if id.UserID != userID {
					t.Errorf("user = %v, want %v", id.UserID, userID)
				}
				return
			}
			if denial == nil {
				t.Fatal("request was allowed, want it denied")
			}
			if denial.Status != tc.wantStatus {
				t.Errorf("status = %d, want %d", denial.Status, tc.wantStatus)
			}
		})
	}
}

func TestRequireRole(t *testing.T) {
	admin := Identity{Role: RoleAdmin}
	kid := Identity{Role: RoleKid}

	if denial := RequireRole(admin, RoleAdmin); denial != nil {
		t.Errorf("an admin was refused an admin route: %+v", denial)
	}
	denial := RequireRole(kid, RoleAdmin)
	if denial == nil {
		t.Fatal("a kid was allowed onto an admin route")
	}
	if denial.Status != http.StatusForbidden {
		t.Errorf("status = %d, want 403", denial.Status)
	}
	if denial := RequireRole(kid, RoleAdmin, RoleKid); denial != nil {
		t.Errorf("a kid was refused a route that allows kids: %+v", denial)
	}
}
