package api

import (
	"net/http"
	"testing"

	"github.com/perleybrook/attic/server/internal/auth"
)

// login performs a successful login and returns the token pair.
func (ts *testServer) login(t *testing.T, username, password, device string) tokenResponse {
	t.Helper()
	rec := ts.do(t, http.MethodPost, "/api/v1/auth/login", loginRequest{
		Username:   username,
		Password:   password,
		DeviceName: device,
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("login status = %d, want 200 (body: %s)", rec.Code, rec.Body.String())
	}
	return decode[tokenResponse](t, rec)
}

func TestLoginIssuesATokenPair(t *testing.T) {
	ts := newTestServer(t)
	user := ts.addUser(t, "ada", auth.RoleAdmin)

	tokens := ts.login(t, "ada", testPassword, "Pixel 8")

	if tokens.AccessToken == "" || tokens.RefreshToken == "" {
		t.Fatal("login returned an empty token")
	}
	if tokens.TokenType != "Bearer" {
		t.Errorf("token_type = %q, want Bearer", tokens.TokenType)
	}
	if tokens.User.Username != "ada" || tokens.User.Role != string(auth.RoleAdmin) {
		t.Errorf("user = %+v", tokens.User)
	}
	if tokens.User.ID != user.ID.String() {
		t.Errorf("user.id = %q, want %q", tokens.User.ID, user.ID)
	}
	if ts.store.deviceCount() != 1 {
		t.Errorf("device count = %d, want 1", ts.store.deviceCount())
	}
}

func TestLoginRejectsBadCredentials(t *testing.T) {
	ts := newTestServer(t)
	ts.addUser(t, "ada", auth.RoleMember)

	tests := map[string]loginRequest{
		"wrong password": {Username: "ada", Password: "nope", DeviceName: "d"},
		"unknown user":   {Username: "grace", Password: testPassword, DeviceName: "d"},
	}
	for name, req := range tests {
		t.Run(name, func(t *testing.T) {
			rec := ts.do(t, http.MethodPost, "/api/v1/auth/login", req)
			if rec.Code != http.StatusUnauthorized {
				t.Fatalf("status = %d, want 401", rec.Code)
			}
			if code := errorCode(t, rec); code != string(CodeUnauthorized) {
				t.Errorf("error.code = %q", code)
			}
			// The same message either way: a different one for an unknown
			// username would enumerate accounts.
			if ts.store.deviceCount() != 0 {
				t.Errorf("a failed login created a device")
			}
		})
	}
}

func TestLoginRequiresBothFields(t *testing.T) {
	ts := newTestServer(t)
	rec := ts.do(t, http.MethodPost, "/api/v1/auth/login", loginRequest{Username: "ada"})

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
	if code := errorCode(t, rec); code != string(CodeBadRequest) {
		t.Errorf("error.code = %q", code)
	}
}

func TestAccessTokenUnlocksTheAPI(t *testing.T) {
	ts := newTestServer(t)
	ts.addUser(t, "ada", auth.RoleMember)
	tokens := ts.login(t, "ada", testPassword, "Pixel 8")

	t.Run("without a token", func(t *testing.T) {
		rec := ts.do(t, http.MethodGet, "/api/v1/me", nil)
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("status = %d, want 401", rec.Code)
		}
	})

	t.Run("with a token", func(t *testing.T) {
		rec := ts.do(t, http.MethodGet, "/api/v1/me", nil, bearer(tokens.AccessToken))
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200 (body: %s)", rec.Code, rec.Body.String())
		}
		if user := decode[userDTO](t, rec); user.Username != "ada" {
			t.Errorf("username = %q", user.Username)
		}
	})

	t.Run("with a forged token", func(t *testing.T) {
		rec := ts.do(t, http.MethodGet, "/api/v1/me", nil, bearer(tokens.AccessToken+"x"))
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("status = %d, want 401", rec.Code)
		}
	})
}

func TestRefreshRotatesAndInvalidatesTheOldToken(t *testing.T) {
	ts := newTestServer(t)
	ts.addUser(t, "ada", auth.RoleMember)
	first := ts.login(t, "ada", testPassword, "Pixel 8")

	rec := ts.do(t, http.MethodPost, "/api/v1/auth/refresh", refreshRequest{RefreshToken: first.RefreshToken})
	if rec.Code != http.StatusOK {
		t.Fatalf("refresh status = %d, want 200 (body: %s)", rec.Code, rec.Body.String())
	}
	second := decode[tokenResponse](t, rec)

	if second.RefreshToken == first.RefreshToken {
		t.Error("refresh returned the same refresh token; it must rotate")
	}
	if second.AccessToken == "" {
		t.Error("refresh returned no access token")
	}
	if second.DeviceID != first.DeviceID {
		t.Errorf("device changed across refresh: %q then %q", first.DeviceID, second.DeviceID)
	}
	if ts.store.deviceCount() != 1 {
		t.Errorf("device count = %d, want 1: refresh must not create a session", ts.store.deviceCount())
	}

	// Replaying the consumed token must fail: that is what limits the damage
	// from a stolen refresh token to a single use.
	replay := ts.do(t, http.MethodPost, "/api/v1/auth/refresh", refreshRequest{RefreshToken: first.RefreshToken})
	if replay.Code != http.StatusUnauthorized {
		t.Fatalf("replayed refresh status = %d, want 401", replay.Code)
	}

	// The new one still works.
	again := ts.do(t, http.MethodPost, "/api/v1/auth/refresh", refreshRequest{RefreshToken: second.RefreshToken})
	if again.Code != http.StatusOK {
		t.Fatalf("second refresh status = %d, want 200", again.Code)
	}
}

func TestRefreshRejectsAnUnknownToken(t *testing.T) {
	ts := newTestServer(t)
	rec := ts.do(t, http.MethodPost, "/api/v1/auth/refresh", refreshRequest{RefreshToken: "not-a-token"})

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
	if code := errorCode(t, rec); code != string(CodeUnauthorized) {
		t.Errorf("error.code = %q", code)
	}
}

func TestLogoutRevokesTheDevice(t *testing.T) {
	ts := newTestServer(t)
	ts.addUser(t, "ada", auth.RoleMember)
	tokens := ts.login(t, "ada", testPassword, "Pixel 8")

	rec := ts.do(t, http.MethodPost, "/api/v1/auth/logout", refreshRequest{RefreshToken: tokens.RefreshToken})
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204 (body: %s)", rec.Code, rec.Body.String())
	}
	if ts.store.deviceCount() != 0 {
		t.Errorf("device count = %d, want 0", ts.store.deviceCount())
	}

	// The revoked refresh token can no longer buy an access token.
	after := ts.do(t, http.MethodPost, "/api/v1/auth/refresh", refreshRequest{RefreshToken: tokens.RefreshToken})
	if after.Code != http.StatusUnauthorized {
		t.Fatalf("refresh after logout = %d, want 401", after.Code)
	}
}

func TestLogoutIsIdempotent(t *testing.T) {
	ts := newTestServer(t)
	rec := ts.do(t, http.MethodPost, "/api/v1/auth/logout", refreshRequest{RefreshToken: "already-gone"})

	// The caller wanted the session gone and it is gone; that is a success.
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", rec.Code)
	}
}

func TestLogoutRevokesOnlyTheCallingDevice(t *testing.T) {
	ts := newTestServer(t)
	ts.addUser(t, "ada", auth.RoleMember)
	phone := ts.login(t, "ada", testPassword, "Pixel 8")
	tv := ts.login(t, "ada", testPassword, "Living room TV")

	ts.do(t, http.MethodPost, "/api/v1/auth/logout", refreshRequest{RefreshToken: phone.RefreshToken})

	rec := ts.do(t, http.MethodPost, "/api/v1/auth/refresh", refreshRequest{RefreshToken: tv.RefreshToken})
	if rec.Code != http.StatusOK {
		t.Fatalf("the TV session was revoked too: status = %d, want 200", rec.Code)
	}
}

func TestDevicesListsSessions(t *testing.T) {
	ts := newTestServer(t)
	ts.addUser(t, "ada", auth.RoleMember)
	tokens := ts.login(t, "ada", testPassword, "Pixel 8")
	ts.login(t, "ada", testPassword, "Living room TV")

	rec := ts.do(t, http.MethodGet, "/api/v1/me/devices", nil, bearer(tokens.AccessToken))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}

	body := decode[struct {
		Devices []deviceDTO `json:"devices"`
	}](t, rec)
	if len(body.Devices) != 2 {
		t.Fatalf("device count = %d, want 2", len(body.Devices))
	}
	for _, d := range body.Devices {
		if d.Name == "" || d.LastSeen == "" {
			t.Errorf("incomplete device: %+v", d)
		}
	}
}

func TestAdminRoutesRejectNonAdmins(t *testing.T) {
	ts := newTestServer(t)
	ts.addUser(t, "kid", auth.RoleKid)
	ts.addUser(t, "boss", auth.RoleAdmin)

	kid := ts.login(t, "kid", testPassword, "Tablet")
	rec := ts.do(t, http.MethodPost, "/api/v1/admin/scan", nil, bearer(kid.AccessToken))
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", rec.Code)
	}
	if code := errorCode(t, rec); code != string(CodeForbidden) {
		t.Errorf("error.code = %q", code)
	}
	if calls := ts.jobs.calls(); len(calls) != 0 {
		t.Errorf("a non-admin queued a scan: %v", calls)
	}

	boss := ts.login(t, "boss", testPassword, "Laptop")
	rec = ts.do(t, http.MethodPost, "/api/v1/admin/scan", nil, bearer(boss.AccessToken))
	if rec.Code != http.StatusAccepted {
		t.Fatalf("admin scan status = %d, want 202 (body: %s)", rec.Code, rec.Body.String())
	}
	if calls := ts.jobs.calls(); len(calls) != 1 || calls[0] != "manual" {
		t.Errorf("scan calls = %v, want [manual]", calls)
	}
}

func TestMediaTokenIsNotAGeneralCredential(t *testing.T) {
	ts := newTestServer(t)
	ts.addUser(t, "ada", auth.RoleMember)
	tokens := ts.login(t, "ada", testPassword, "Pixel 8")

	rec := ts.do(t, http.MethodPost, "/api/v1/auth/media-token", nil, bearer(tokens.AccessToken))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body: %s)", rec.Code, rec.Body.String())
	}
	media := decode[mediaTokenResponse](t, rec)
	if media.Token == "" || media.ExpiresAt == "" {
		t.Fatalf("incomplete media token response: %+v", media)
	}

	// A media token must not open the rest of the API, even as a bearer header.
	denied := ts.do(t, http.MethodGet, "/api/v1/me", nil, bearer(media.Token))
	if denied.Code != http.StatusUnauthorized {
		t.Fatalf("media token reached /me: status = %d, want 401", denied.Code)
	}
}

func TestAccessTokenIsRejectedInTheQueryString(t *testing.T) {
	ts := newTestServer(t)
	ts.addUser(t, "ada", auth.RoleMember)
	tokens := ts.login(t, "ada", testPassword, "Pixel 8")

	// Only media tokens may ride in a URL, where they end up in logs.
	rec := ts.do(t, http.MethodGet, "/api/v1/covers/"+emptyHash+"?token="+tokens.AccessToken, nil)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
}

const emptyHash = "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
