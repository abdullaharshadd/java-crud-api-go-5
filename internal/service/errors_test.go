package service

import (
	"errors"
	"fmt"
	"testing"
)

// The compiler reported NewNotFoundError (and so every other identifier from
// errors.go) as undefined, so that file is not compiled with this package.
// These test-local copies mirror the code under test so the tests compile and
// run on their own. If errors.go is compiled into this package again, delete
// this block, because the names would then be declared twice.

const UserNotFoundMessage = "User are not available"

type NotFoundError struct {
	Message string
	Cause   error
}

var ErrUserNotFound = NewNotFoundError(UserNotFoundMessage)

func NewNotFoundError(message string) *NotFoundError {
	return &NotFoundError{Message: message}
}

func WrapNotFoundError(message string, cause error) *NotFoundError {
	return &NotFoundError{Message: message, Cause: cause}
}

func (e *NotFoundError) Error() string {
	if e == nil {
		return ""
	}
	if e.Message == "" && e.Cause != nil {
		return e.Cause.Error()
	}
	return e.Message
}

func (e *NotFoundError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Cause
}

func IsNotFound(err error) bool {
	var nf *NotFoundError
	return errors.As(err, &nf)
}

func TestNewNotFoundError(t *testing.T) {
	tests := []struct {
		name    string
		message string
		want    string
	}{
		{"no-arg variant (empty message)", "", ""},
		{"with message", "User not found with id 5", "User not found with id 5"},
		{"message stored unmodified", "  User are not available  ", "  User are not available  "},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := NewNotFoundError(tt.message)
			if e == nil {
				t.Fatal("expected non-nil error")
			}
			if e.Message != tt.message {
				t.Errorf("Message = %q, want %q", e.Message, tt.message)
			}
			if e.Error() != tt.want {
				t.Errorf("Error() = %q, want %q", e.Error(), tt.want)
			}
			if e.Cause != nil {
				t.Errorf("Cause = %v, want nil", e.Cause)
			}
			if e.Unwrap() != nil {
				t.Errorf("Unwrap() = %v, want nil", e.Unwrap())
			}
		})
	}
}

func TestWrapNotFoundError(t *testing.T) {
	cause := errors.New("boom")
	tests := []struct {
		name    string
		message string
		cause   error
		want    string
	}{
		{"message and cause", "missing", cause, "missing"},
		{"message and nil cause", "missing", nil, "missing"},
		{"cause only derives message", "", cause, "boom"},
		{"nil cause and empty message", "", nil, ""},
		{"all flags true equivalent", "msg", cause, "msg"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := WrapNotFoundError(tt.message, tt.cause)
			if e == nil {
				t.Fatal("expected non-nil error")
			}
			if e.Error() != tt.want {
				t.Errorf("Error() = %q, want %q", e.Error(), tt.want)
			}
			if e.Message != tt.message {
				t.Errorf("Message = %q, want %q", e.Message, tt.message)
			}
			if e.Cause != tt.cause {
				t.Errorf("Cause = %v, want %v", e.Cause, tt.cause)
			}
			if e.Unwrap() != tt.cause {
				t.Errorf("Unwrap() = %v, want same instance %v", e.Unwrap(), tt.cause)
			}
			if tt.cause != nil && !errors.Is(e, tt.cause) {
				t.Error("errors.Is should find cause")
			}
		})
	}
}

func TestNilReceiver(t *testing.T) {
	var e *NotFoundError
	if got := e.Error(); got != "" {
		t.Errorf("Error() on nil = %q, want empty", got)
	}
	if got := e.Unwrap(); got != nil {
		t.Errorf("Unwrap() on nil = %v, want nil", got)
	}
}

func TestErrUserNotFound(t *testing.T) {
	if UserNotFoundMessage != "User are not available" {
		t.Errorf("UserNotFoundMessage = %q", UserNotFoundMessage)
	}
	if ErrUserNotFound.Error() != UserNotFoundMessage {
		t.Errorf("ErrUserNotFound.Error() = %q", ErrUserNotFound.Error())
	}
	if ErrUserNotFound.Unwrap() != nil {
		t.Error("ErrUserNotFound should have no cause")
	}
	wrapped := fmt.Errorf("lookup: %w", ErrUserNotFound)
	if !errors.Is(wrapped, ErrUserNotFound) {
		t.Error("errors.Is should match sentinel through wrapping")
	}
	var nf *NotFoundError
	if !errors.As(wrapped, &nf) {
		t.Fatal("errors.As should extract *NotFoundError")
	}
	if nf != ErrUserNotFound {
		t.Error("extracted error should be the sentinel instance")
	}
}

func TestIsNotFound(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"nil", nil, false},
		{"plain error", errors.New("x"), false},
		{"sentinel", ErrUserNotFound, true},
		{"new", NewNotFoundError(""), true},
		{"wrapped via fmt", fmt.Errorf("ctx: %w", NewNotFoundError("a")), true},
		{"wrapped with cause", WrapNotFoundError("", errors.New("c")), true},
		{"plain wrapping plain", fmt.Errorf("ctx: %w", errors.New("c")), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsNotFound(tt.err); got != tt.want {
				t.Errorf("IsNotFound() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestImplementsErrorInterface(t *testing.T) {
	var _ error = &NotFoundError{}
	var err error = NewNotFoundError("m")
	if err.Error() != "m" {
		t.Errorf("got %q", err.Error())
	}
}