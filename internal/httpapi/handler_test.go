package httpapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestWriteErrorBadRequest(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	writeError(rec, req, fmt.Errorf("%w: bad id", ErrBadRequest))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("code=%d", rec.Code)
	}
	if rec.Body.Len() != 0 {
		t.Fatalf("body=%q", rec.Body.String())
	}
}

func TestWriteErrorUnsupportedMediaType(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/x", nil)
	writeError(rec, req, ErrUnsupportedMediaType)
	if rec.Code != http.StatusUnsupportedMediaType {
		t.Fatalf("code=%d", rec.Code)
	}
}

func TestWriteErrorMethodNotAllowed(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/x", nil)
	writeError(rec, req, &MethodNotAllowedError{Allowed: []string{"GET", "HEAD", "OPTIONS"}})
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("code=%d", rec.Code)
	}
	if got := rec.Header().Get("Allow"); got != "GET, HEAD, OPTIONS" {
		t.Fatalf("allow=%q", got)
	}
}

func TestWriteErrorNil(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	writeError(rec, req, nil)
	if rec.Body.Len() != 0 {
		t.Fatalf("body=%q", rec.Body.String())
	}
}

func TestWriteErrorGeneric(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/boom", nil)
	writeError(rec, req, errors.New("db down"))
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("code=%d", rec.Code)
	}
	var be bootError
	if err := json.Unmarshal(rec.Body.Bytes(), &be); err != nil {
		t.Fatal(err)
	}
	if be.Status != 500 || be.Path != "/boom" || be.Error != "Internal Server Error" {
		t.Fatalf("got %+v", be)
	}
}

func TestNotFoundHandler(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/nope", nil)
	NotFoundHandler().ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("code=%d", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Fatalf("ct=%q", ct)
	}
	var be bootError
	if err := json.Unmarshal(rec.Body.Bytes(), &be); err != nil {
		t.Fatal(err)
	}
	if be.Status != 404 || be.Path != "/nope" {
		t.Fatalf("got %+v", be)
	}
}

func TestIsJSONContentType(t *testing.T) {
	tests := []struct {
		ct   string
		want bool
	}{
		{"application/json", true},
		{"application/json;charset=UTF-8", true},
		{"Application/JSON", true},
		{"application/problem+json", true},
		{"text/plain", false},
		{"", false},
	}
	for _, tc := range tests {
		if got := isJSONContentType(tc.ct); got != tc.want {
			t.Fatalf("%q: got %v want %v", tc.ct, got, tc.want)
		}
	}
}