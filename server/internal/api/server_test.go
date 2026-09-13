package api

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/perleybrook/attic/server/internal/auth"
	"github.com/perleybrook/attic/server/internal/config"
	"github.com/perleybrook/attic/server/internal/store"
	"github.com/perleybrook/attic/server/internal/stream"
)

const testPassword = "correct horse battery staple"

// testServer is a Server wired to fakes, plus handles on them.
type testServer struct {
	*Server
	store *fakeStore
	jobs  *fakeEnqueuer
	cfg   *config.Config

	// root is the library root media files must live under.
	root string
}

func newTestServer(t *testing.T, opts ...func(*Deps)) *testServer {
	t.Helper()

	t.Setenv("ATTIC_JWT_SECRET", "0123456789abcdef0123456789abcdef")
	root := t.TempDir()
	t.Setenv("ATTIC_LIBRARY_ROOTS", root)
	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("config.Load() error = %v", err)
	}

	guard, err := stream.NewGuard(cfg.LibraryRoots)
	if err != nil {
		t.Fatalf("stream.NewGuard() error = %v", err)
	}

	fake := newFakeStore()
	enqueuer := &fakeEnqueuer{}

	deps := Deps{
		Config: cfg,
		Log:    slog.New(slog.NewTextHandler(io.Discard, nil)),
		Store:  fake,
		Tokens: cfg.NewTokens(),
		Guard:  guard,
		Jobs:   enqueuer,
	}
	for _, opt := range opts {
		opt(&deps)
	}

	return &testServer{Server: NewServer(deps), store: fake, jobs: enqueuer, cfg: cfg, root: root}
}

// addUser creates an account with a known password and returns it.
func (ts *testServer) addUser(t *testing.T, username string, role auth.Role) *store.User {
	t.Helper()
	hash, err := auth.HashPassword(testPassword)
	if err != nil {
		t.Fatalf("HashPassword() error = %v", err)
	}
	user := &store.User{ID: uuid.New(), Username: username, PasswordHash: hash, Role: role}
	ts.store.addUser(user)
	return user
}

// do performs a request and returns the recorder.
func (ts *testServer) do(t *testing.T, method, path string, body any, headers ...[2]string) *httptest.ResponseRecorder {
	t.Helper()

	var reader io.Reader
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal body: %v", err)
		}
		reader = bytes.NewReader(encoded)
	}

	req := httptest.NewRequest(method, path, reader)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	for _, h := range headers {
		req.Header.Set(h[0], h[1])
	}

	rec := httptest.NewRecorder()
	ts.ServeHTTP(rec, req)
	return rec
}

func bearer(token string) [2]string { return [2]string{"Authorization", "Bearer " + token} }

// decode unmarshals a response body, failing the test if it is not JSON.
func decode[T any](t *testing.T, rec *httptest.ResponseRecorder) T {
	t.Helper()
	var out T
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode %s: %v (body: %s)", rec.Result().Status, err, rec.Body.String())
	}
	return out
}

// errorCode pulls the code out of an error envelope.
func errorCode(t *testing.T, rec *httptest.ResponseRecorder) string {
	t.Helper()
	var env struct {
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
		t.Fatalf("decode error envelope: %v (body: %s)", err, rec.Body.String())
	}
	if env.Error.Message == "" {
		t.Errorf("error envelope has no message: %s", rec.Body.String())
	}
	return env.Error.Code
}

// ------------------------------------------------------------ operational ----

func TestHealthz(t *testing.T) {
	ts := newTestServer(t)
	rec := ts.do(t, http.MethodGet, "/healthz", nil)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	body := decode[map[string]string](t, rec)
	if body["status"] != "ok" {
		t.Errorf("status field = %q, want ok", body["status"])
	}
	if body["version"] == "" {
		t.Error("version field is empty")
	}
}

func TestMetricsExposesAtticCollectors(t *testing.T) {
	ts := newTestServer(t)
	ts.do(t, http.MethodGet, "/healthz", nil)

	rec := ts.do(t, http.MethodGet, "/metrics", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	for _, want := range []string{"attic_http_requests_total", "attic_http_request_duration_seconds", "go_goroutines"} {
		if !strings.Contains(rec.Body.String(), want) {
			t.Errorf("/metrics output is missing %q", want)
		}
	}
}

func TestNotFoundUsesErrorEnvelope(t *testing.T) {
	ts := newTestServer(t)
	rec := ts.do(t, http.MethodGet, "/api/v1/nope", nil)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
	if code := errorCode(t, rec); code != string(CodeNotFound) {
		t.Errorf("error.code = %q, want %q", code, CodeNotFound)
	}
}

func TestPingIsPublicAndJSON(t *testing.T) {
	ts := newTestServer(t)
	rec := ts.do(t, http.MethodGet, "/api/v1/ping", nil)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
		t.Errorf("Content-Type = %q", ct)
	}
	if body := decode[map[string]string](t, rec); body["service"] != "attic" {
		t.Errorf("service = %q, want attic", body["service"])
	}
}
