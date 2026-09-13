package api

import (
	"net/http"
)

// TriggerScan asks for a library scan. Admin only.
//
// It returns 202 rather than blocking: the scan is a background job, and a
// large library takes minutes. River deduplicates, so a impatient double-tap
// does not queue two scans.
func (s *Server) TriggerScan(w http.ResponseWriter, r *http.Request) {
	if s.jobs == nil {
		WriteError(w, http.StatusServiceUnavailable, CodeInternal,
			"Background jobs are not running on this server.")
		return
	}

	if err := s.jobs.ScanLibrary(r.Context(), "manual"); err != nil {
		s.internalError(w, r, "enqueue scan", err)
		return
	}

	WriteJSON(w, http.StatusAccepted, map[string]string{"status": "scan_queued"})
}
