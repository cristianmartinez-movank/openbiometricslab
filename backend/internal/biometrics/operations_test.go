package biometrics

import (
	"context"
	"errors"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestClientVerifyDocumentPreservesClientErrorDetail(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_, _ = io.WriteString(w, `{"detail":"No face detected in selfie"}`)
	}))
	defer server.Close()

	client, err := NewClient(server.URL, server.Client())
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	_, err = client.VerifyDocument(
		context.Background(),
		"image/jpeg",
		[]byte("document"),
		"image/jpeg",
		[]byte("selfie"),
		0.4,
	)
	var engineErr *EngineHTTPError
	if !errors.As(err, &engineErr) {
		t.Fatalf("VerifyDocument() error = %v, want *EngineHTTPError", err)
	}
	if engineErr.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", engineErr.StatusCode, http.StatusBadRequest)
	}
	if engineErr.Detail != "No face detected in selfie" {
		t.Fatalf("detail = %q, want selfie face error", engineErr.Detail)
	}
}

func TestClientBiometricsOperations(t *testing.T) {
	const sessionID = "7c45542a-f5a2-4e31-bd6e-c54a8c9a50c2"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/verify":
			checkMultipartRequest(t, r, map[string]string{
				"image1": "image/jpeg",
				"image2": "image/png",
			}, map[string]string{"threshold": "0.65"})
			_, _ = io.WriteString(w, `{"is_match":true,"similarity":0.8}`)
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/documents/scan":
			checkMultipartRequest(t, r, map[string]string{"image": "image/png"}, map[string]string{"presentation": "digital"})
			_, _ = io.WriteString(w, `{"detected":true,"ocr":null,"mrz":null,"has_face":false}`)
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/documents/verify":
			checkMultipartRequest(t, r, map[string]string{
				"document": "image/png",
				"selfie":   "image/jpeg",
			}, map[string]string{"threshold": "0.5"})
			_, _ = io.WriteString(w, `{"is_match":true,"similarity":0.8}`)
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/liveness/presets":
			_, _ = io.WriteString(w, `[{"name":"head_turn","description":"Head movement","num_challenges":2,"challenge_types":["turn_left"],"timeout_seconds":5,"require_passive":true}]`)
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/liveness/sessions":
			if got := r.URL.Query().Get("preset"); got != "head_turn" {
				t.Errorf("preset = %q, want head_turn", got)
			}
			_, _ = io.WriteString(w, `{"session_id":"`+sessionID+`","state":"pending","challenges":[]}`)
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/liveness/sessions/"+sessionID+"/frame":
			checkMultipartImages(t, r, map[string]string{"image": "image/jpeg"})
			_, _ = io.WriteString(w, `{"session_id":"`+sessionID+`","passed":true,"confidence":0.9,"challenges_remaining":0,"session_complete":true,"is_live":true,"state":"completed"}`)
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/liveness/sessions/"+sessionID:
			_, _ = io.WriteString(w, `{"session_id":"`+sessionID+`","state":"completed","challenges":[]}`)
		case r.Method == http.MethodDelete && r.URL.Path == "/api/v1/liveness/sessions/"+sessionID:
			_, _ = io.WriteString(w, `{"cancelled":"`+sessionID+`"}`)
		default:
			t.Errorf("unexpected upstream request: %s %s", r.Method, r.URL.String())
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client, err := NewClient(server.URL, server.Client())
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	if _, err := client.Verify(context.Background(), "image/jpeg", []byte("first"), "image/png", []byte("second"), 0.65); err != nil {
		t.Fatalf("Verify() error = %v", err)
	}
	if _, err := client.ScanDocument(context.Background(), "image/png", []byte("document"), "digital"); err != nil {
		t.Fatalf("ScanDocument() error = %v", err)
	}
	if _, err := client.VerifyDocument(context.Background(), "image/png", []byte("document"), "image/jpeg", []byte("selfie"), 0.5); err != nil {
		t.Fatalf("VerifyDocument() error = %v", err)
	}
	if _, err := client.LivenessPresets(context.Background()); err != nil {
		t.Fatalf("LivenessPresets() error = %v", err)
	}
	if _, err := client.CreateLivenessSession(context.Background(), "head_turn"); err != nil {
		t.Fatalf("CreateLivenessSession() error = %v", err)
	}
	if _, err := client.SubmitLivenessFrame(context.Background(), sessionID, "image/jpeg", []byte("frame")); err != nil {
		t.Fatalf("SubmitLivenessFrame() error = %v", err)
	}
	if _, err := client.GetLivenessSession(context.Background(), sessionID); err != nil {
		t.Fatalf("GetLivenessSession() error = %v", err)
	}
	if _, err := client.CancelLivenessSession(context.Background(), sessionID); err != nil {
		t.Fatalf("CancelLivenessSession() error = %v", err)
	}
}

func TestClientCreateLivenessSessionEscapesPreset(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("preset"); got != "full & safe" {
			t.Errorf("preset = %q, want decoded query value", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"session_id":"session","state":"pending","challenges":[]}`)
	}))
	defer server.Close()

	client, err := NewClient(server.URL, server.Client())
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}
	if _, err := client.CreateLivenessSession(context.Background(), "full & safe"); err != nil {
		t.Fatalf("CreateLivenessSession() error = %v", err)
	}
}

func TestClientOperationsRejectInvalidResponses(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		response string
		call     func(*Client) error
	}{
		{
			name:     "verification contract",
			path:     "/api/v1/verify",
			response: `{"is_match":true}`,
			call: func(client *Client) error {
				_, err := client.Verify(context.Background(), "image/jpeg", []byte("a"), "image/jpeg", []byte("b"), 0.4)
				return err
			},
		},
		{
			name:     "document contract",
			path:     "/api/v1/documents/scan",
			response: `{"detected":true}`,
			call: func(client *Client) error {
				_, err := client.ScanDocument(context.Background(), "image/jpeg", []byte("a"), "physical")
				return err
			},
		},
		{
			name:     "liveness session contract",
			path:     "/api/v1/liveness/sessions",
			response: `{"state":"pending"}`,
			call: func(client *Client) error {
				_, err := client.CreateLivenessSession(context.Background(), "")
				return err
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != test.path {
					t.Errorf("path = %q, want %q", r.URL.Path, test.path)
				}
				w.Header().Set("Content-Type", "application/json")
				_, _ = io.WriteString(w, test.response)
			}))
			defer server.Close()

			client, err := NewClient(server.URL, server.Client())
			if err != nil {
				t.Fatalf("NewClient() error = %v", err)
			}
			if err := test.call(client); err == nil {
				t.Fatal("operation error = nil, want invalid response error")
			}
		})
	}
}

func checkMultipartImages(t *testing.T, r *http.Request, expected map[string]string) {
	checkMultipartRequest(t, r, expected, nil)
}

func checkMultipartRequest(t *testing.T, r *http.Request, expectedImages, expectedValues map[string]string) {
	t.Helper()
	mediaType, params, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || mediaType != "multipart/form-data" {
		t.Errorf("Content-Type = %q, want multipart/form-data", r.Header.Get("Content-Type"))
		return
	}
	reader := multipart.NewReader(r.Body, params["boundary"])
	foundImages := make(map[string]bool, len(expectedImages))
	foundValues := make(map[string]bool, len(expectedValues))
	for {
		part, err := reader.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Errorf("read multipart part: %v", err)
			return
		}
		name := part.FormName()
		if want, ok := expectedImages[name]; ok {
			foundImages[name] = true
			if got := part.Header.Get("Content-Type"); got != want {
				t.Errorf("%s content type = %q, want %q", name, got, want)
			}
			_, _ = io.Copy(io.Discard, part)
		} else if want, ok := expectedValues[name]; ok {
			foundValues[name] = true
			value, err := io.ReadAll(part)
			if err != nil {
				t.Errorf("read multipart field %q: %v", name, err)
			} else if string(value) != want {
				t.Errorf("%s = %q, want %q", name, value, want)
			}
		}
	}
	for name := range expectedImages {
		if !foundImages[name] {
			t.Errorf("multipart field %q not found", name)
		}
	}
	for name := range expectedValues {
		if !foundValues[name] {
			t.Errorf("multipart field %q not found", name)
		}
	}
}
