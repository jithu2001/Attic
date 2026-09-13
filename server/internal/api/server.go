// Package api wires Attic's HTTP surface: the chi router, cross-cutting
// middleware (logging, metrics, recovery) and the handlers themselves.
package api

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/perleybrook/attic/server/internal/auth"
	"github.com/perleybrook/attic/server/internal/config"
	"github.com/perleybrook/attic/server/internal/ffmpeg"
	"github.com/perleybrook/attic/server/internal/stream"
)

// Deps are everything the HTTP layer needs to do its job.
type Deps struct {
	Config *config.Config
	Log    *slog.Logger
	Store  Store
	Tokens *auth.Tokens

	// Guard resolves media paths against the library roots. Required.
	Guard *stream.Guard

	// Covers finds extracted cover art. Optional: without it the cover
	// endpoint reports 404 rather than failing to start.
	Covers CoverLookup

	// FFmpeg powers on-the-fly transcoding. Optional.
	FFmpeg *ffmpeg.Tools

	// Jobs triggers background scans. Optional.
	Jobs Enqueuer
}

// Server owns the HTTP handler and its dependencies.
type Server struct {
	cfg     *config.Config
	log     *slog.Logger
	store   Store
	tokens  *auth.Tokens
	authn   *auth.Authenticator
	guard   *stream.Guard
	covers  CoverLookup
	ffmpeg  *ffmpeg.Tools
	jobs    Enqueuer
	metrics *Metrics
	reg     *prometheus.Registry
	router  chi.Router
}

// NewServer builds the router. The returned Server is an http.Handler.
func NewServer(deps Deps) *Server {
	reg := prometheus.NewRegistry()
	reg.MustRegister(
		collectorsGo(),
		collectorsProcess(),
	)

	s := &Server{
		cfg:     deps.Config,
		log:     deps.Log,
		store:   deps.Store,
		tokens:  deps.Tokens,
		authn:   auth.NewAuthenticator(deps.Tokens),
		guard:   deps.Guard,
		covers:  deps.Covers,
		ffmpeg:  deps.FFmpeg,
		jobs:    deps.Jobs,
		metrics: NewMetrics(reg),
		reg:     reg,
	}
	s.router = s.routes()
	return s
}

func (s *Server) routes() chi.Router {
	r := chi.NewRouter()

	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(s.requestLogger)
	r.Use(middleware.Recoverer)
	r.Use(s.metrics.Middleware)

	r.NotFound(NotFound)
	r.MethodNotAllowed(MethodNotAllowed)

	// Operational endpoints live outside /api/v1 and need no auth.
	r.Get("/healthz", s.Health)
	r.Handle("/metrics", promhttp.HandlerFor(s.reg, promhttp.HandlerOpts{Registry: s.reg}))

	r.Route("/api/v1", func(r chi.Router) {
		// Requests that are not streaming media get a hard time limit.
		// Streaming ones must not: a 40-minute FLAC outlasts any timeout.
		r.Group(func(r chi.Router) {
			r.Use(middleware.Timeout(30 * time.Second))

			// ---- public ----
			r.Get("/ping", s.Ping)
			r.Post("/auth/login", s.Login)
			r.Post("/auth/refresh", s.Refresh)
			r.Post("/auth/logout", s.Logout)

			// ---- authenticated ----
			r.Group(func(r chi.Router) {
				r.Use(s.requireAuth)

				r.Post("/auth/media-token", s.MediaToken)
				r.Get("/me", s.Me)
				r.Get("/me/devices", s.Devices)

				r.Get("/music/artists", s.Artists)
				r.Get("/music/artists/{id}/albums", s.ArtistAlbums)
				r.Get("/music/albums/{id}", s.AlbumDetail)
				r.Get("/music/tracks/{id}", s.TrackDetail)
				r.Get("/search", s.Search)

				r.Get("/playlists", s.ListPlaylists)
				r.Post("/playlists", s.CreatePlaylist)
				r.Get("/playlists/{id}", s.GetPlaylist)
				r.Put("/playlists/{id}", s.UpdatePlaylist)
				r.Delete("/playlists/{id}", s.DeletePlaylist)
				r.Put("/playlists/{id}/tracks", s.SetPlaylistTracks)

				r.With(s.requireAdmin).Post("/admin/scan", s.TriggerScan)
			})
		})

		// ---- media ----
		// No request timeout, and a media token in `?token=` is accepted
		// because players cannot set headers on a URL they have handed to the
		// platform's media stack.
		r.Group(func(r chi.Router) {
			r.Use(s.requireMediaAuth)

			r.Get("/music/tracks/{id}/audio", s.TrackAudio)
			r.Get("/covers/{hash}", s.Cover)
		})
	})

	return r
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.router.ServeHTTP(w, r)
}

// Ping is an unauthenticated liveness probe the app uses to validate a server
// address before it has any credentials.
func (s *Server) Ping(w http.ResponseWriter, r *http.Request) {
	WriteJSON(w, http.StatusOK, map[string]string{
		"pong":    time.Now().UTC().Format(timeFormat),
		"service": "attic",
		"version": Version,
	})
}

// requireAuth rejects requests without a valid bearer access token.
func (s *Server) requireAuth(next http.Handler) http.Handler {
	return s.authenticate(next, auth.AuthenticateOptions{})
}

// requireMediaAuth additionally accepts a media token in the query string.
func (s *Server) requireMediaAuth(next http.Handler) http.Handler {
	return s.authenticate(next, auth.AuthenticateOptions{AllowMediaToken: true})
}

func (s *Server) authenticate(next http.Handler, opts auth.AuthenticateOptions) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id, denial := s.authn.Authenticate(r, opts)
		if denial != nil {
			writeDenial(w, denial)
			return
		}
		next.ServeHTTP(w, r.WithContext(auth.WithIdentity(r.Context(), id)))
	})
}

// requireAdmin gates admin-only routes.
func (s *Server) requireAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if denial := auth.RequireRole(identity(r.Context()), auth.RoleAdmin); denial != nil {
			writeDenial(w, denial)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func writeDenial(w http.ResponseWriter, d *auth.Denial) {
	WriteError(w, d.Status, ErrorCode(d.Code), d.Message)
}

// internalError logs the cause and tells the client nothing about it.
func (s *Server) internalError(w http.ResponseWriter, r *http.Request, what string, err error) {
	s.log.Error("request failed",
		"what", what,
		"error", err,
		"path", r.URL.Path,
		"request_id", middleware.GetReqID(r.Context()),
	)
	WriteError(w, http.StatusInternalServerError, CodeInternal, "Something went wrong on the server.")
}

func (s *Server) requestLogger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
		start := time.Now()
		defer func() {
			attrs := []any{
				"method", r.Method,
				"path", r.URL.Path,
				"status", ww.Status(),
				"bytes", ww.BytesWritten(),
				"duration_ms", time.Since(start).Milliseconds(),
				"request_id", middleware.GetReqID(r.Context()),
			}
			// Range requests are the mechanism behind instant seeking, so make
			// them visible: a 206 in the log is proof it is working.
			if rng := r.Header.Get("Range"); rng != "" {
				attrs = append(attrs, "range", rng, "content_range", ww.Header().Get("Content-Range"))
			}
			s.log.Info("http request", attrs...)
		}()
		next.ServeHTTP(ww, r)
	})
}
