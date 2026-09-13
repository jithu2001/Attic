package auth

import (
	"context"
	"net/http"
	"strings"

	"github.com/google/uuid"
)

type contextKey struct{ name string }

var identityKey = contextKey{"attic.identity"}

// Identity is the authenticated caller attached to a request context.
type Identity struct {
	UserID uuid.UUID
	Role   Role

	// FromMediaToken is true when the caller authenticated with a `?token=`
	// media token rather than a bearer header. Such tokens may only reach
	// media handlers.
	FromMediaToken bool
}

// IsAdmin reports whether the caller may perform admin-only actions.
func (i Identity) IsAdmin() bool { return i.Role == RoleAdmin }

// WithIdentity returns a copy of ctx carrying id. Exported for tests.
func WithIdentity(ctx context.Context, id Identity) context.Context {
	return context.WithValue(ctx, identityKey, id)
}

// IdentityFrom returns the authenticated caller, if any.
func IdentityFrom(ctx context.Context) (Identity, bool) {
	id, ok := ctx.Value(identityKey).(Identity)
	return id, ok
}

// Denial explains why a request was rejected, so the HTTP layer can map it to
// a status and error code without importing the API package here.
type Denial struct {
	Status  int
	Code    string
	Message string
}

// Authenticator verifies requests. The API package wraps it in middleware that
// renders Denial as Attic's error envelope.
type Authenticator struct {
	tokens *Tokens
}

// NewAuthenticator builds an Authenticator over the given token issuer.
func NewAuthenticator(tokens *Tokens) *Authenticator { return &Authenticator{tokens: tokens} }

// AuthenticateOptions tunes what a handler accepts.
type AuthenticateOptions struct {
	// AllowMediaToken lets the request authenticate via `?token=`. Only media
	// handlers set this; everything else requires a bearer header.
	AllowMediaToken bool
}

// Authenticate extracts and verifies the caller's credentials.
func (a *Authenticator) Authenticate(r *http.Request, opts AuthenticateOptions) (Identity, *Denial) {
	token, fromQuery := extractToken(r, opts.AllowMediaToken)
	if token == "" {
		return Identity{}, &Denial{
			Status:  http.StatusUnauthorized,
			Code:    "unauthorized",
			Message: "Authentication required.",
		}
	}

	claims, err := a.tokens.Parse(token)
	switch {
	case err == ErrExpiredToken:
		return Identity{}, &Denial{
			Status:  http.StatusUnauthorized,
			Code:    "token_expired",
			Message: "Access token expired.",
		}
	case err != nil:
		return Identity{}, &Denial{
			Status:  http.StatusUnauthorized,
			Code:    "unauthorized",
			Message: "Invalid credentials.",
		}
	}

	// A media token is not a general-purpose credential: it is long-lived and
	// travels in URLs, where it lands in logs and history.
	if claims.IsMedia() && !opts.AllowMediaToken {
		return Identity{}, &Denial{
			Status:  http.StatusUnauthorized,
			Code:    "unauthorized",
			Message: "This token may only be used for media playback.",
		}
	}
	// Conversely, a query parameter is only ever trusted for media tokens.
	if fromQuery && !claims.IsMedia() {
		return Identity{}, &Denial{
			Status:  http.StatusUnauthorized,
			Code:    "unauthorized",
			Message: "Only media tokens may be passed in the query string.",
		}
	}

	userID, err := claims.UserID()
	if err != nil {
		return Identity{}, &Denial{
			Status:  http.StatusUnauthorized,
			Code:    "unauthorized",
			Message: "Invalid credentials.",
		}
	}

	return Identity{UserID: userID, Role: claims.Role, FromMediaToken: claims.IsMedia()}, nil
}

// RequireRole reports a Denial unless id holds one of roles.
func RequireRole(id Identity, roles ...Role) *Denial {
	for _, role := range roles {
		if id.Role == role {
			return nil
		}
	}
	return &Denial{
		Status:  http.StatusForbidden,
		Code:    "forbidden",
		Message: "Your account does not have access to this.",
	}
}

func extractToken(r *http.Request, allowQuery bool) (token string, fromQuery bool) {
	if header := r.Header.Get("Authorization"); header != "" {
		const prefix = "Bearer "
		if len(header) > len(prefix) && strings.EqualFold(header[:len(prefix)], prefix) {
			return strings.TrimSpace(header[len(prefix):]), false
		}
		return "", false
	}
	if allowQuery {
		if q := strings.TrimSpace(r.URL.Query().Get("token")); q != "" {
			return q, true
		}
	}
	return "", false
}
