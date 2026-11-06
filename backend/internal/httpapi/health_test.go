package httpapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"openbiometrics-lab/backend/internal/config"
)

type engineHealthStub struct {
	err           error
	detectBody    []byte
	detectErr     error
	operationBody []byte
	operationErr  error
}

func (s engineHealthStub) Check(context.Context) error {
	return s.err
}

func (s engineHealthStub) Detect(context.Context, string, []byte) ([]byte, error) {
	return s.detectBody, s.detectErr
}

func (s engineHealthStub) Verify(context.Context, string, []byte, string, []byte, float64) ([]byte, error) {
	return s.operationBody, s.operationErr
}

func (s engineHealthStub) VerifyDocument(context.Context, string, []byte, string, []byte, float64) ([]byte, error) {
	return s.operationBody, s.operationErr
}

func (s engineHealthStub) ScanDocument(context.Context, string, []byte, string) ([]byte, error) {
	return s.operationBody, s.operationErr
}

func (s engineHealthStub) LivenessPresets(context.Context) ([]byte, error) {
	return s.operationBody, s.operationErr
}

func (s engineHealthStub) CreateLivenessSession(context.Context, string) ([]byte, error) {
	return s.operationBody, s.operationErr
}

func (s engineHealthStub) SubmitLivenessFrame(context.Context, string, string, []byte) ([]byte, error) {
	return s.operationBody, s.operationErr
}

func (s engineHealthStub) GetLivenessSession(context.Context, string) ([]byte, error) {
	return s.operationBody, s.operationErr
}

func (s engineHealthStub) CancelLivenessSession(context.Context, string) ([]byte, error) {
	return s.operationBody, s.operationErr
}

func TestHealth(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	response := httptest.NewRecorder()

	NewHandler(engineHealthStub{}, config.DefaultMaxImageBytes).ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusOK)
	}
	if contentType := response.Header().Get("Content-Type"); contentType != "application/json" {
		t.Fatalf("Content-Type = %q, want application/json", contentType)
	}
	if body := response.Body.String(); body != "{\"status\":\"ok\"}\n" {
		t.Fatalf("body = %q, want %q", body, "{\"status\":\"ok\"}\n")
	}
}

func TestHealthRejectsNonGetMethod(t *testing.T) {
	request := httptest.NewRequest(http.MethodPost, "/healthz", nil)
	response := httptest.NewRecorder()

	NewHandler(engineHealthStub{}, config.DefaultMaxImageBytes).ServeHTTP(response, request)

	if response.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusMethodNotAllowed)
	}
}

func TestEngineHealth(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/api/v1/biometrics/health", nil)
	response := httptest.NewRecorder()

	NewHandler(engineHealthStub{}, config.DefaultMaxImageBytes).ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusOK)
	}
	if body := response.Body.String(); body != "{\"status\":\"ok\",\"engine\":\"available\"}\n" {
		t.Fatalf("body = %q, want healthy engine response", body)
	}
}

func TestEngineHealthReturnsUnavailable(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/api/v1/biometrics/health", nil)
	response := httptest.NewRecorder()

	NewHandler(
		engineHealthStub{err: context.DeadlineExceeded},
		config.DefaultMaxImageBytes,
	).ServeHTTP(response, request)

	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusServiceUnavailable)
	}
	if body := response.Body.String(); body != "{\"status\":\"error\",\"engine\":\"unavailable\"}\n" {
		t.Fatalf("body = %q, want sanitized unavailable response", body)
	}
}
