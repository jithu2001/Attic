package api

import (
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5/middleware"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// Metrics holds the process-wide Prometheus collectors.
type Metrics struct {
	Requests *prometheus.CounterVec
	Latency  *prometheus.HistogramVec
	InFlight prometheus.Gauge
}

// NewMetrics registers Attic's HTTP collectors on reg.
func NewMetrics(reg prometheus.Registerer) *Metrics {
	f := promauto.With(reg)
	return &Metrics{
		Requests: f.NewCounterVec(prometheus.CounterOpts{
			Name: "attic_http_requests_total",
			Help: "Total HTTP requests handled, by route, method and status.",
		}, []string{"route", "method", "status"}),
		Latency: f.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "attic_http_request_duration_seconds",
			Help:    "HTTP request latency, by route and method.",
			Buckets: prometheus.DefBuckets,
		}, []string{"route", "method"}),
		InFlight: f.NewGauge(prometheus.GaugeOpts{
			Name: "attic_http_requests_in_flight",
			Help: "HTTP requests currently being served.",
		}),
	}
}

// Middleware records request counts and latency. Labels use the chi route
// pattern (e.g. /api/v1/assets/{id}) rather than the raw path, so ids do not
// explode cardinality.
func (m *Metrics) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		m.InFlight.Inc()
		defer m.InFlight.Dec()

		ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
		start := time.Now()
		next.ServeHTTP(ww, r)

		route := routePattern(r)
		m.Requests.WithLabelValues(route, r.Method, strconv.Itoa(ww.Status())).Inc()
		m.Latency.WithLabelValues(route, r.Method).Observe(time.Since(start).Seconds())
	})
}

func routePattern(r *http.Request) string {
	if rctx := chiRouteContext(r); rctx != "" {
		return rctx
	}
	return "unmatched"
}
