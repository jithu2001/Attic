package api

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
)

func collectorsGo() prometheus.Collector { return collectors.NewGoCollector() }
func collectorsProcess() prometheus.Collector {
	return collectors.NewProcessCollector(collectors.ProcessCollectorOpts{})
}

// chiRouteContext returns the matched chi route pattern, or "" if the request
// did not match a route (404s).
func chiRouteContext(r *http.Request) string {
	rctx := chi.RouteContext(r.Context())
	if rctx == nil {
		return ""
	}
	return rctx.RoutePattern()
}
