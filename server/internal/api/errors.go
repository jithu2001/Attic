package api

import (
	"encoding/json"
	"log/slog"
	"net/http"
)

// ErrorCode is the stable, machine-readable half of an API error.
// Clients switch on these; the message is for humans and may change.
type ErrorCode string

const (
	CodeBadRequest   ErrorCode = "bad_request"
	CodeUnauthorized ErrorCode = "unauthorized"
	CodeForbidden    ErrorCode = "forbidden"
	CodeNotFound     ErrorCode = "not_found"
	CodeConflict     ErrorCode = "conflict"
	CodeInternal     ErrorCode = "internal"
)

type errorBody struct {
	Code    ErrorCode `json:"code"`
	Message string    `json:"message"`
}

type errorEnvelope struct {
	Error errorBody `json:"error"`
}

// WriteJSON writes v as a JSON response with the given status.
func WriteJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if v == nil {
		return
	}
	if err := json.NewEncoder(w).Encode(v); err != nil {
		slog.Error("write json response", "error", err)
	}
}

// WriteError writes the canonical error envelope:
//
//	{"error": {"code": "...", "message": "..."}}
func WriteError(w http.ResponseWriter, status int, code ErrorCode, message string) {
	WriteJSON(w, status, errorEnvelope{Error: errorBody{Code: code, Message: message}})
}

// NotFound is the router-level 404 handler.
func NotFound(w http.ResponseWriter, r *http.Request) {
	WriteError(w, http.StatusNotFound, CodeNotFound, "no such endpoint")
}

// MethodNotAllowed is the router-level 405 handler.
func MethodNotAllowed(w http.ResponseWriter, r *http.Request) {
	WriteError(w, http.StatusMethodNotAllowed, CodeBadRequest, "method not allowed")
}
