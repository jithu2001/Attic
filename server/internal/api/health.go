package api

import (
	"net/http"
	"time"
)

// Version is stamped at build time via -ldflags.
var Version = "dev"

type healthResponse struct {
	Status  string `json:"status"`
	Version string `json:"version"`
	Uptime  string `json:"uptime"`
}

var startedAt = time.Now()

// Health reports process liveness. It deliberately does not touch the
// database: readiness of dependencies is reported through /metrics so a
// transient DB blip does not get the container killed by an orchestrator.
func (s *Server) Health(w http.ResponseWriter, r *http.Request) {
	WriteJSON(w, http.StatusOK, healthResponse{
		Status:  "ok",
		Version: Version,
		Uptime:  time.Since(startedAt).Round(time.Second).String(),
	})
}
