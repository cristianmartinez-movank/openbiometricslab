package biometrics

import (
	"context"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestClientDetectPostsImageMultipart(t *testing.T) {
	imageData := []byte("synthetic image payload")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %q, want %q", r.Method, http.MethodPost)
		}
		if r.URL.Path != "/api/v1/detect" {
			t.Errorf("path = %q, want /api/v1/detect", r.URL.Path)
		}

		mediaType, params, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
		if err != nil || mediaType != "multipart/form-data" {
			t.Errorf("Content-Type = %q, want multipart/form-data", r.Header.Get("Content-Type"))
			return
		}
		reader := multipart.NewReader(r.Body, params["boundary"])
		part, err := reader.NextPart()
		if err != nil {
			t.Errorf("read multipart image field: %v", err)
			return
		}
		if part.FormName() != "image" {
			t.Errorf("field name = %q, want image", part.FormName())
		}
		if part.FileName() != "image.png" {
			t.Errorf("filename = %q, want image.png", part.FileName())
		}
		if part.Header.Get("Content-Type") != "image/png" {
			t.Errorf("image Content-Type = %q, want image/png", part.Header.Get("Content-Type"))
		}
		uploaded, err := io.ReadAll(part)
		if err != nil {
			t.Errorf("read uploaded bytes: %v", err)
		}
		if string(uploaded) != string(imageData) {
			t.Errorf("uploaded data = %q, want %q", uploaded, imageData)
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"faces":[],"count":0}`))
	}))
	defer server.Close()

	client, err := NewClient(server.URL, server.Client())
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}
	result, err := client.Detect(context.Background(), "image/png", imageData)
	if err != nil {
		t.Fatalf("Detect() error = %v", err)
	}
	if string(result) != `{"faces":[],"count":0}` {
		t.Fatalf("Detect() = %s, want empty-face response", result)
	}
}

func TestClientDetectRejectsUnexpectedResponses(t *testing.T) {
	tests := []struct {
		name        string
		status      int
		contentType string
		body        string
	}{
		{name: "upstream error", status: http.StatusInternalServerError, contentType: "application/json", body: `{"detail":"trace"}`},
		{name: "non-json", status: http.StatusOK, contentType: "text/plain", body: "ok"},
		{name: "invalid json", status: http.StatusOK, contentType: "application/json", body: "not-json"},
		{name: "missing fields", status: http.StatusOK, contentType: "application/json", body: `{"faces":[]}`},
		{name: "bad count", status: http.StatusOK, contentType: "application/json", body: `{"faces":[],"count":1}`},
		{name: "missing detection object", status: http.StatusOK, contentType: "application/json", body: `{"faces":[{}],"count":1}`},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", test.contentType)
				w.WriteHeader(test.status)
				_, _ = io.WriteString(w, test.body)
			}))
			defer server.Close()

			client, err := NewClient(server.URL, server.Client())
			if err != nil {
				t.Fatalf("NewClient() error = %v", err)
			}
			if _, err := client.Detect(context.Background(), "image/jpeg", []byte("image")); err == nil {
				t.Fatal("Detect() error = nil, want unexpected response error")
			}
		})
	}
}

func TestClientDetectRejectsOversizedResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, strings.Repeat("x", maxDetectionResponseBytes+1))
	}))
	defer server.Close()

	client, err := NewClient(server.URL, server.Client())
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}
	if _, err := client.Detect(context.Background(), "image/jpeg", []byte("image")); err == nil {
		t.Fatal("Detect() error = nil, want response-size error")
	}
}

func TestClientDetectHonorsTimeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(50 * time.Millisecond)
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"faces":[],"count":0}`)
	}))
	defer server.Close()

	httpClient := server.Client()
	httpClient.Timeout = time.Millisecond
	client, err := NewClient(server.URL, httpClient)
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}
	if _, err := client.Detect(context.Background(), "image/jpeg", []byte("image")); err == nil {
		t.Fatal("Detect() error = nil, want timeout error")
	}
}
