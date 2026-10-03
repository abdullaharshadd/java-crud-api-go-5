package model

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

// ErrorMessage is the JSON error payload returned to clients when a handled
// domain error (e.g. a missing user) is mapped to an HTTP response.
//
// The status is held as Spring's HttpStatus enum *name* (e.g. "NOT_FOUND"),
// not the numeric code, because that is what Jackson serialised on the wire:
//
//	{"status":"NOT_FOUND","message":"User are not available"}
//
// The zero value is a valid, empty ErrorMessage (equivalent to the Lombok
// no-args constructor).
type ErrorMessage struct {
	status  string
	message string
}

// errorMessageJSON is the wire representation of ErrorMessage. Field order
// matches the Java declaration order (status, message).
type errorMessageJSON struct {
	Status  *string `json:"status"`
	Message *string `json:"message"`
}

// NewErrorMessage returns an ErrorMessage populated with the given status
// name and message, in declaration order (status first, then message).
func NewErrorMessage(status, message string) *ErrorMessage {
	return &ErrorMessage{status: status, message: message}
}

// NewErrorMessageFromCode builds an ErrorMessage from a numeric HTTP status
// code, converting it to the Spring HttpStatus enum name via HTTPStatusName.
func NewErrorMessageFromCode(code int, message string) *ErrorMessage {
	return NewErrorMessage(HTTPStatusName(code), message)
}

// Status returns the HTTP status enum name (e.g. "NOT_FOUND").
func (e *ErrorMessage) Status() string { return e.status }

// SetStatus sets the HTTP status enum name.
func (e *ErrorMessage) SetStatus(status string) { e.status = status }

// Message returns the human-readable error message.
func (e *ErrorMessage) Message() string { return e.message }

// SetMessage sets the human-readable error message.
func (e *ErrorMessage) SetMessage(message string) { e.message = message }

// Equal reports whether e and other carry the same status and message.
// Two nil pointers are considered equal.
func (e *ErrorMessage) Equal(other *ErrorMessage) bool {
	if e == nil || other == nil {
		return e == other
	}
	return e.status == other.status && e.message == other.message
}

// String returns a Lombok-style representation including both fields.
func (e *ErrorMessage) String() string {
	if e == nil {
		return "ErrorMessage(nil)"
	}
	return fmt.Sprintf("ErrorMessage(status=%s, message=%s)", nullable(e.status), nullable(e.message))
}

// nullable mimics Java's rendering of an unset (null) field in toString.
func nullable(s string) string {
	if s == "" {
		return "null"
	}
	return s
}

// MarshalJSON encodes the ErrorMessage as {"status":...,"message":...}.
// Unset fields are rendered as JSON null, matching Jackson's default
// behaviour for null references.
func (e ErrorMessage) MarshalJSON() ([]byte, error) {
	var out errorMessageJSON
	if e.status != "" {
		s := e.status
		out.Status = &s
	}
	if e.message != "" {
		m := e.message
		out.Message = &m
	}
	return json.Marshal(out)
}

// UnmarshalJSON decodes an ErrorMessage from its wire representation.
func (e *ErrorMessage) UnmarshalJSON(data []byte) error {
	var in errorMessageJSON
	if err := json.Unmarshal(data, &in); err != nil {
		return fmt.Errorf("decode error message: %w", err)
	}
	e.status, e.message = "", ""
	if in.Status != nil {
		e.status = *in.Status
	}
	if in.Message != nil {
		e.message = *in.Message
	}
	return nil
}

// statusNameOverrides covers codes whose Spring HttpStatus enum name does not
// follow mechanically from Go's http.StatusText reason phrase.
var statusNameOverrides = map[int]string{
	http.StatusTeapot:                       "I_AM_A_TEAPOT",
	http.StatusRequestURITooLong:            "URI_TOO_LONG",
	http.StatusNonAuthoritativeInfo:         "NON_AUTHORITATIVE_INFORMATION",
	http.StatusRequestEntityTooLarge:        "PAYLOAD_TOO_LARGE",
	http.StatusRequestedRangeNotSatisfiable: "REQUESTED_RANGE_NOT_SATISFIABLE",
	http.StatusHTTPVersionNotSupported:      "HTTP_VERSION_NOT_SUPPORTED",
}

// HTTPStatusName converts a numeric HTTP status code into the corresponding
// Spring HttpStatus enum name (e.g. 404 -> "NOT_FOUND"). Unknown codes yield
// an empty string.
func HTTPStatusName(code int) string {
	if name, ok := statusNameOverrides[code]; ok {
		return name
	}
	text := http.StatusText(code)
	if text == "" {
		return ""
	}
	r := strings.NewReplacer(" ", "_", "-", "_", "'", "")
	return strings.ToUpper(r.Replace(text))
}
