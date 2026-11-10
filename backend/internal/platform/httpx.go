package platform

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strconv"
)

// Error is a domain error that maps to an HTTP response: {"error": Code, "message": Message}.
type Error struct {
	Status        int
	Code, Message string
}

func (e *Error) Error() string { return e.Code + ": " + e.Message }

var (
	ErrUnauthorized = &Error{401, "unauthorized", "login required"}
	ErrForbidden    = &Error{403, "forbidden", "not allowed"}
	ErrNotFound     = &Error{404, "not_found", "not found"}
)

// BadRequest returns a 400 error with code "bad_request".
func BadRequest(msg string) *Error { return &Error{400, "bad_request", msg} }

// Conflict returns a 409 error with the given code.
func Conflict(code, msg string) *Error { return &Error{409, code, msg} }

// WriteJSON writes v as JSON with the given status. Status 204 writes no body.
func WriteJSON(w http.ResponseWriter, status int, v any) {
	if status == http.StatusNoContent {
		w.WriteHeader(status)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("write json: %v", err)
	}
}

// WriteError writes err as an API error; unknown errors are logged and returned as 500.
func WriteError(w http.ResponseWriter, r *http.Request, err error) {
	var e *Error
	if errors.As(err, &e) {
		WriteJSON(w, e.Status, map[string]string{"error": e.Code, "message": e.Message})
		return
	}
	log.Printf("%s %s: %v", r.Method, r.URL.Path, err)
	WriteJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal", "message": "internal error"})
}

// DecodeJSON decodes a request body (max 1 MiB) into v.
func DecodeJSON(w http.ResponseWriter, r *http.Request, v any) error {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	if err := json.NewDecoder(r.Body).Decode(v); err != nil {
		return BadRequest("invalid JSON body")
	}
	return nil
}

// PathID parses a positive int64 path parameter.
func PathID(r *http.Request, name string) (int64, error) {
	id, err := strconv.ParseInt(r.PathValue(name), 10, 64)
	if err != nil || id <= 0 {
		return 0, BadRequest("invalid " + name)
	}
	return id, nil
}
