package biometrics

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestClientCheck(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("method = %q, want %q", r.Method, http.MethodGet)
		}
		if r.URL.Path != "/api/v1/health" {
			t.Errorf("path = %q, want /api/v1/health", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"ok","models_loaded":true}`))
	}))
	defer server.Close()

	client, err := NewClient(server.URL+"/", server.Client())
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}
	if err := client.Check(context.Background()); err != nil {
		t.Fatalf("Check() error = %v", err)
	}
}

func TestClientCheckRejectsEngineErrorStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer server.Close()

	client, err := NewClient(server.URL, server.Client())
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}
	if err := client.Check(context.Background()); err == nil {
		t.Fatal("Check() error = nil, want an error for HTTP 503")
	}
}

func TestClientCheckRejectsUnhealthyEngine(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{name: "detector not loaded", body: `{"status":"ok","models_loaded":false}`},
		{name: "unexpected JSON", body: `{"unexpected":true}`},
		{name: "invalid JSON", body: `not-json`},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				_, _ = w.Write([]byte(test.body))
			}))
			defer server.Close()

			client, err := NewClient(server.URL, server.Client())
			if err != nil {
				t.Fatalf("NewClient() error = %v", err)
			}
			if err := client.Check(context.Background()); err == nil {
				t.Fatal("Check() error = nil, want unhealthy response error")
			}
		})
	}
}

func TestClientCheckHonorsTimeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(50 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	httpClient := server.Client()
	httpClient.Timeout = time.Millisecond
	client, err := NewClient(server.URL, httpClient)
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}
	if err := client.Check(context.Background()); err == nil {
		t.Fatal("Check() error = nil, want a timeout error")
	}
}

func TestNewClientRejectsInvalidBaseURL(t *testing.T) {
	tests := []string{
		"",
		"ftp://example.com",
		"http://user:password@example.com",
		"http://example.com?token=secret",
	}

	for _, baseURL := range tests {
		t.Run(baseURL, func(t *testing.T) {
			if _, err := NewClient(baseURL, http.DefaultClient); err == nil {
				t.Fatal("NewClient() error = nil, want validation error")
			}
		})
	}
}
