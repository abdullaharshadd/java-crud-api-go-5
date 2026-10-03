// Package httpapi contains the HTTP transport layer of the smart-contact
// service: handlers, request decoding and the central error-to-response
// mapping that replaces Spring's @ControllerAdvice exception handling.
package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/rs/zerolog/log"

	"migrated-app/internal/model"
	"migrated-app/internal/service"
)

// ErrBadRequest marks client errors that Spring's ResponseEntityExceptionHandler
// answered with 400 and an empty body (validation failure, missing/null/
// malformed JSON body, unparsable or out-of-int32 path variables).
var ErrBadRequest = errors.New("bad request")

// ErrUnsupportedMediaType marks a request whose Content-Type is missing or is
// not application/json (or application/*+json). Spring answered 415, empty body.
var ErrUnsupportedMediaType = errors.New("unsupported media type")

// MethodNotAllowedError reports that the route exists but not for the request
// method. It is answered with 405, an Allow header and an empty body.
type MethodNotAllowedError struct {
	// Allowed lists the methods supported by the matched route.
	Allowed []string
}

// Error implements the error interface.
func (e *MethodNotAllowedError) Error() string {
	return "method not allowed; allowed: " + strings.Join(e.Allowed, ", ")
}

// bootTimestampLayout matches Spring Boot 2.7's DefaultErrorAttributes
// timestamp rendering (ISO-8601 with milliseconds and numeric offset).
const bootTimestampLayout = "2006-01-02T15:04:05.000-07:00"

// bootError is Spring Boot's default /error JSON body (without message/trace,
// which Boot 2.7 omits by default).
type bootError struct {
	Timestamp string `json:"timestamp"`
	Status    int    `json:"status"`
	Error     string `json:"error"`
	Path      string `json:"path"`
}

// writeError maps an error returned by a handler or service to the HTTP
// response the original Spring application produced.
//
// It replaces RestResponseEntityExceptionHandling (@ControllerAdvice): a
// service.NotFoundError (Java UserNotFoundException, including "subclasses",
// i.e. anything wrapping it) becomes 404 with an ErrorMessage body carrying
// the status name and the error's message. The remaining branches reproduce
// the inherited ResponseEntityExceptionHandler behaviour and Boot's fallback.
func writeError(w http.ResponseWriter, r *http.Request, err error) {
	var nf *service.NotFoundError
	var mna *MethodNotAllowedError

	switch {
	case err == nil:
		return
	case errors.As(err, &nf):
		writeJSON(w, http.StatusNotFound,
			model.NewErrorMessageFromCode(http.StatusNotFound, nf.Error()))
	case errors.Is(err, ErrBadRequest):
		w.WriteHeader(http.StatusBadRequest)
	case errors.Is(err, ErrUnsupportedMediaType):
		w.WriteHeader(http.StatusUnsupportedMediaType)
	case errors.As(err, &mna):
		if len(mna.Allowed) > 0 {
			w.Header().Set("Allow", strings.Join(mna.Allowed, ", "))
		}
		w.WriteHeader(http.StatusMethodNotAllowed)
	default:
		log.Error().Err(err).
			Str("method", r.Method).
			Str("path", r.URL.Path).
			Msg("unhandled error while processing request")
		writeBootError(w, r, http.StatusInternalServerError)
	}
}

// NotFoundHandler answers requests for unknown paths with Spring Boot's
// default 404 error JSON. Register it as the router's fallback.
func NotFoundHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeBootError(w, r, http.StatusNotFound)
	})
}

// writeBootError writes Spring Boot's default error body for status.
func writeBootError(w http.ResponseWriter, r *http.Request, status int) {
	writeJSON(w, status, bootError{
		Timestamp: time.Now().Format(bootTimestampLayout),
		Status:    status,
		Error:     http.StatusText(status),
		Path:      r.URL.Path,
	})
}

// writeJSON serialises v as the response body with the given status.
func writeJSON(w http.ResponseWriter, status int, v any) {
	body, err := json.Marshal(v)
	if err != nil {
		log.Error().Err(err).Msg("failed to encode error response")
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if _, err := w.Write(body); err != nil {
		log.Debug().Err(err).Msg("failed to write error response")
	}
}

// isJSONContentType reports whether a Content-Type header value is acceptable
// to Spring's Jackson converter: application/json or application/*+json, with
// optional parameters such as charset.
func isJSONContentType(ct string) bool {
	if ct == "" {
		return false
	}
	mt := strings.ToLower(strings.TrimSpace(strings.SplitN(ct, ";", 2)[0]))
	return mt == "application/json" ||
		(strings.HasPrefix(mt, "application/") && strings.HasSuffix(mt, "+json"))
}
