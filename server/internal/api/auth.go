package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/perleybrook/attic/server/internal/auth"
	"github.com/perleybrook/attic/server/internal/store"
)

type loginRequest struct {
	Username   string `json:"username"`
	Password   string `json:"password"`
	DeviceName string `json:"device_name"`
}

type tokenResponse struct {
	AccessToken  string  `json:"access_token"`
	RefreshToken string  `json:"refresh_token"`
	ExpiresAt    string  `json:"expires_at"`
	TokenType    string  `json:"token_type"`
	DeviceID     string  `json:"device_id"`
	User         userDTO `json:"user"`
}

// Login exchanges a username and password for an access token and a refresh
// token bound to the calling device.
func (s *Server) Login(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if !decodeJSON(w, r, &req) {
		return
	}

	req.Username = strings.TrimSpace(req.Username)
	req.DeviceName = strings.TrimSpace(req.DeviceName)
	if req.Username == "" || req.Password == "" {
		WriteError(w, http.StatusBadRequest, CodeBadRequest, "Username and password are required.")
		return
	}
	if req.DeviceName == "" {
		req.DeviceName = "Unnamed device"
	}

	user, err := s.store.UserByUsername(r.Context(), req.Username)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			// Spend the time a real verification would, so response timing
			// does not reveal which usernames exist.
			_ = auth.VerifyPassword(dummyHash, req.Password)
			s.unauthorizedLogin(w)
			return
		}
		s.internalError(w, r, "look up user", err)
		return
	}

	if err := auth.VerifyPassword(user.PasswordHash, req.Password); err != nil {
		if !errors.Is(err, auth.ErrMismatch) {
			s.log.Error("stored password hash is unreadable", "user_id", user.ID, "error", err)
		}
		s.unauthorizedLogin(w)
		return
	}

	// A successful login is the only moment the plaintext password is in hand,
	// so it is the only moment a weaker hash can be upgraded.
	if auth.NeedsRehash(user.PasswordHash) {
		if hash, err := auth.HashPassword(req.Password); err == nil {
			if err := s.store.UpdatePasswordHash(r.Context(), user.ID, hash); err != nil {
				s.log.Warn("could not upgrade password hash", "user_id", user.ID, "error", err)
			}
		}
	}

	s.issueSession(w, r, user, req.DeviceName)
}

func (s *Server) unauthorizedLogin(w http.ResponseWriter) {
	WriteError(w, http.StatusUnauthorized, CodeUnauthorized, "Incorrect username or password.")
}

// dummyHash is a real argon2id hash of a random string, used to keep failed
// logins for unknown usernames as slow as failed logins for known ones.
const dummyHash = "$argon2id$v=19$m=65536,t=3,p=2$Y2xvY2tlcXVhbGlzaW5n$Ej7wYQqVQ9jHLGWYyPjZ8vJ0V4h1Q0rN2sT5uX8aB1c"

type refreshRequest struct {
	RefreshToken string `json:"refresh_token"`
}

// Refresh rotates a refresh token and issues a new access token.
//
// Rotation is unconditional: every refresh invalidates the token that was
// presented, so a stolen refresh token is usable at most once before the real
// device's next refresh fails and the session is gone.
func (s *Server) Refresh(w http.ResponseWriter, r *http.Request) {
	var req refreshRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if strings.TrimSpace(req.RefreshToken) == "" {
		WriteError(w, http.StatusBadRequest, CodeBadRequest, "A refresh token is required.")
		return
	}

	oldHash := auth.HashRefreshToken(req.RefreshToken)

	next, err := s.tokens.NewRefreshToken()
	if err != nil {
		s.internalError(w, r, "generate refresh token", err)
		return
	}

	device, err := s.store.RotateRefreshToken(r.Context(), oldHash, next.Hash, next.ExpiresAt)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			WriteError(w, http.StatusUnauthorized, CodeUnauthorized,
				"This session has expired or been revoked. Please sign in again.")
			return
		}
		s.internalError(w, r, "rotate refresh token", err)
		return
	}

	user, err := s.store.UserByID(r.Context(), device.UserID)
	if err != nil {
		s.internalError(w, r, "look up user", err)
		return
	}

	access, expiresAt, err := s.tokens.IssueAccess(user.ID, user.Role)
	if err != nil {
		s.internalError(w, r, "issue access token", err)
		return
	}

	WriteJSON(w, http.StatusOK, tokenResponse{
		AccessToken:  access,
		RefreshToken: next.Plaintext,
		ExpiresAt:    expiresAt.UTC().Format(timeFormat),
		TokenType:    "Bearer",
		DeviceID:     device.ID.String(),
		User:         toUserDTO(*user),
	})
}

// Logout revokes the device holding the presented refresh token.
func (s *Server) Logout(w http.ResponseWriter, r *http.Request) {
	var req refreshRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if strings.TrimSpace(req.RefreshToken) == "" {
		WriteError(w, http.StatusBadRequest, CodeBadRequest, "A refresh token is required.")
		return
	}

	err := s.store.DeleteDeviceByRefreshToken(r.Context(), auth.HashRefreshToken(req.RefreshToken))
	// Logging out an already-dead session is a success from the client's point
	// of view: it wanted the session gone, and it is.
	if err != nil && !errors.Is(err, store.ErrNotFound) {
		s.internalError(w, r, "revoke device", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// issueSession mints a token pair and records the device.
func (s *Server) issueSession(w http.ResponseWriter, r *http.Request, user *store.User, deviceName string) {
	refresh, err := s.tokens.NewRefreshToken()
	if err != nil {
		s.internalError(w, r, "generate refresh token", err)
		return
	}

	device, err := s.store.CreateDevice(r.Context(), user.ID, deviceName, refresh.Hash, refresh.ExpiresAt)
	if err != nil {
		s.internalError(w, r, "record device", err)
		return
	}

	access, expiresAt, err := s.tokens.IssueAccess(user.ID, user.Role)
	if err != nil {
		s.internalError(w, r, "issue access token", err)
		return
	}

	WriteJSON(w, http.StatusOK, tokenResponse{
		AccessToken:  access,
		RefreshToken: refresh.Plaintext,
		ExpiresAt:    expiresAt.UTC().Format(timeFormat),
		TokenType:    "Bearer",
		DeviceID:     device.ID.String(),
		User:         toUserDTO(*user),
	})
}

type mediaTokenResponse struct {
	Token     string `json:"token"`
	ExpiresAt string `json:"expires_at"`
}

// MediaToken mints a token for `?token=` media URLs.
//
// Players hand a URL to the platform's media stack and never get to set a
// header on it again, so they need a credential that outlives the 15-minute
// access token. This one is accepted by media handlers only.
func (s *Server) MediaToken(w http.ResponseWriter, r *http.Request) {
	id := identity(r.Context())

	token, expiresAt, err := s.tokens.IssueMedia(id.UserID, id.Role)
	if err != nil {
		s.internalError(w, r, "issue media token", err)
		return
	}

	WriteJSON(w, http.StatusOK, mediaTokenResponse{
		Token:     token,
		ExpiresAt: expiresAt.UTC().Format(timeFormat),
	})
}

// Me returns the signed-in account.
func (s *Server) Me(w http.ResponseWriter, r *http.Request) {
	user, err := s.store.UserByID(r.Context(), identity(r.Context()).UserID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			WriteError(w, http.StatusUnauthorized, CodeUnauthorized, "This account no longer exists.")
			return
		}
		s.internalError(w, r, "look up user", err)
		return
	}
	WriteJSON(w, http.StatusOK, toUserDTO(*user))
}

// Devices lists the signed-in account's sessions.
func (s *Server) Devices(w http.ResponseWriter, r *http.Request) {
	devices, err := s.store.DevicesForUser(r.Context(), identity(r.Context()).UserID)
	if err != nil {
		s.internalError(w, r, "list devices", err)
		return
	}

	out := make([]deviceDTO, 0, len(devices))
	for _, d := range devices {
		out = append(out, deviceDTO{
			ID:       d.ID.String(),
			Name:     d.Name,
			LastSeen: d.LastSeen.UTC().Format(timeFormat),
		})
	}
	WriteJSON(w, http.StatusOK, map[string]any{"devices": out})
}

// decodeJSON reads a JSON body, rejecting unknown fields and oversized payloads.
func decodeJSON(w http.ResponseWriter, r *http.Request, dst any) bool {
	const maxBody = 1 << 20 // 1 MB: a playlist of ten thousand ids still fits
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxBody))
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(dst); err != nil {
		WriteError(w, http.StatusBadRequest, CodeBadRequest, "Request body is not valid JSON.")
		return false
	}
	return true
}
