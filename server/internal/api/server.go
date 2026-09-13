// Package api wires Attic's HTTP surface: the chi router, cross-cutting
// middleware (logging, metrics, recovery) and the operational endpoints.
// Feature routes are mounted here by later phases.
package api

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/perleybrook/attic/server/internal/config"
)

// Server owns the HTTP handler and its dependencies.
type Server struct {
	cfg     *config.Config
	log     *slog.Logger
	metrics *Metrics
	reg     *prometheus.Registry
	router  chi.Router
}

// NewServer builds the router. The returned Server is an http.Handler.
func NewServer(cfg *config.Config, log *slog.Logger) *Server {
	reg := prometheus.NewRegistry()
	reg.MustRegister(
		collectorsGo(),
		collectorsProcess(),
	)

	s := &Server{
		cfg:     cfg,
		log:     log,
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
	r.Use(middleware.Timeout(60 * time.Second))
	r.Use(s.metrics.Middleware)

	r.NotFound(NotFound)
	r.MethodNotAllowed(MethodNotAllowed)

	// Operational endpoints live outside /api/v1 and need no auth.
	r.Get("/healthz", s.Health)
	r.Handle("/metrics", promhttp.HandlerFor(s.reg, promhttp.HandlerOpts{Registry: s.reg}))

	// Versioned API. Feature routers (auth, photos, music, video) are
	// mounted here in later phases; routes shipped here never change shape.
	r.Route("/api/v1", func(r chi.Router) {
		r.Get("/ping", func(w http.ResponseWriter, r *http.Request) {
			WriteJSON(w, http.StatusOK, map[string]string{"pong": time.Now().UTC().Format(time.RFC3339)})
		})
	})

	return r
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.router.ServeHTTP(w, r)
}

func (s *Server) requestLogger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
		start := time.Now()
		defer func() {
			s.log.Info("http request",
				"method", r.Method,
				"path", r.URL.Path,
				"status", ww.Status(),
				"bytes", ww.BytesWritten(),
				"duration_ms", time.Since(start).Milliseconds(),
				"request_id", middleware.GetReqID(r.Context()),
			)
		}()
		next.ServeHTTP(ww, r)
	})
}
