// Package service contains the business layer of the smart-contact service
// together with the domain errors it reports to the HTTP layer.
package service

import "errors"

// UserNotFoundMessage is the exact detail message the original service used
// when a user lookup returned nothing. It is surfaced verbatim in the HTTP
// error body, so it must not be "corrected".
const UserNotFoundMessage = "User are not available"

// NotFoundError reports that a requested user could not be found.
//
// It replaces the Java checked exception UserNotFoundException. The five Java
// constructors collapse into this single type plus the constructor helpers
// below; the HTTP layer extracts it with errors.As and maps it to a
// 404 response carrying Error() as the message.
type NotFoundError struct {
	// Message is the detail message; it may be empty.
	Message string
	// Cause is the underlying error, if any; it may be nil.
	Cause error
}

// ErrUserNotFound is the sentinel returned when a user lookup by ID finds no
// row. Match it with errors.Is, or extract a *NotFoundError with errors.As.
var ErrUserNotFound = NewNotFoundError(UserNotFoundMessage)

// NewNotFoundError creates a NotFoundError with the given detail message and
// no cause (Java: UserNotFoundException(String)). Pass "" for the no-arg
// variant (Java: UserNotFoundException()).
func NewNotFoundError(message string) *NotFoundError {
	return &NotFoundError{Message: message}
}

// WrapNotFoundError creates a NotFoundError with a detail message and an
// underlying cause (Java: UserNotFoundException(String, Throwable)). If
// message is empty, the message is derived from the cause
// (Java: UserNotFoundException(Throwable)).
//
// MIGRATION_NOTE: Java's protected constructor controlling suppression and
// stack-trace writability has no Go equivalent; Go errors carry no stack
// trace or suppressed list, so those flags are intentionally dropped.
func WrapNotFoundError(message string, cause error) *NotFoundError {
	return &NotFoundError{Message: message, Cause: cause}
}

// Error implements the error interface. When no explicit message was given
// but a cause exists, the message is derived from the cause, mirroring
// Java's Throwable(Throwable) behaviour.
func (e *NotFoundError) Error() string {
	if e == nil {
		return ""
	}
	if e.Message == "" && e.Cause != nil {
		return e.Cause.Error()
	}
	return e.Message
}

// Unwrap returns the underlying cause so errors.Is/errors.As can traverse it.
func (e *NotFoundError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Cause
}

// IsNotFound reports whether err (or any error it wraps) is a NotFoundError.
func IsNotFound(err error) bool {
	var nf *NotFoundError
	return errors.As(err, &nf)
}
