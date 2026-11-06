package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"openbiometrics-lab/backend/internal/config"
)

type detectionStub struct {
	engineHealthStub
	called      bool
	contentType string
	imageData   []byte
	response    []byte
	err         error
}

func (s *detectionStub) Detect(_ context.Context, contentType string, imageData []byte) ([]byte, error) {
	s.called = true
	s.contentType = contentType
	s.imageData = imageData
	return s.response, s.err
}

func TestDetectForwardsValidPNG(t *testing.T) {
	imageData := testPNG(t)
	detector := &detectionStub{response: []byte(`{"faces":[],"count":0}`)}
	request := detectionRequest(t, imageData, "image.txt")
	response := httptest.NewRecorder()

	NewHandler(detector, config.DefaultMaxImageBytes).ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body: %s", response.Code, http.StatusOK, response.Body)
	}
	if !detector.called {
		t.Fatal("detector was not called")
	}
	if detector.contentType != "image/png" {
		t.Fatalf("content type = %q, want image/png", detector.contentType)
	}
	if !bytes.Equal(detector.imageData, imageData) {
		t.Fatal("forwarded image differs from uploaded image")
	}
	if contentType := response.Header().Get("Content-Type"); contentType != "application/json" {
		t.Fatalf("Content-Type = %q, want application/json", contentType)
	}
	if body := response.Body.String(); body != `{"faces":[],"count":0}` {
		t.Fatalf("body = %q, want upstream response", body)
	}
}

func TestDetectForwardsValidJPEG(t *testing.T) {
	imageData := testJPEG(t)
	detector := &detectionStub{response: []byte(`{"faces":[],"count":0}`)}
	request := detectionRequest(t, imageData, "image.png")
	response := httptest.NewRecorder()

	NewHandler(detector, config.DefaultMaxImageBytes).ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body: %s", response.Code, http.StatusOK, response.Body)
	}
	if detector.contentType != "image/jpeg" {
		t.Fatalf("content type = %q, want image/jpeg", detector.contentType)
	}
}

func TestDetectUsesConfiguredImageLimit(t *testing.T) {
	imageData := testPNG(t)
	detector := &detectionStub{response: []byte(`{"faces":[],"count":0}`)}
	request := detectionRequest(t, imageData, "face.png")
	response := httptest.NewRecorder()

	NewHandler(detector, int64(len(imageData)-1)).ServeHTTP(response, request)

	if response.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusRequestEntityTooLarge)
	}
	if detector.called {
		t.Fatal("detector was called for an image above the configured limit")
	}
}

func TestDetectRejectsInvalidUploads(t *testing.T) {
	validPNG := testPNG(t)
	tooLarge := bytes.Repeat([]byte{0}, int(config.DefaultMaxImageBytes+1))

	tests := []struct {
		name       string
		imageData  []byte
		fieldName  string
		wantStatus int
	}{
		{name: "missing file", fieldName: "other", wantStatus: http.StatusBadRequest},
		{name: "empty file", fieldName: "image", wantStatus: http.StatusBadRequest},
		{name: "unsupported content", imageData: []byte("plain text"), fieldName: "image", wantStatus: http.StatusUnsupportedMediaType},
		{name: "invalid png", imageData: []byte("\x89PNG\r\n\x1a\nnot-an-image"), fieldName: "image", wantStatus: http.StatusBadRequest},
		{name: "oversized file", imageData: tooLarge, fieldName: "image", wantStatus: http.StatusRequestEntityTooLarge},
		{name: "valid image control", imageData: validPNG, fieldName: "image", wantStatus: http.StatusOK},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			detector := &detectionStub{response: []byte(`{"faces":[],"count":0}`)}
			request := detectionRequestWithField(t, test.imageData, "upload.bin", test.fieldName)
			response := httptest.NewRecorder()

			NewHandler(detector, config.DefaultMaxImageBytes).ServeHTTP(response, request)

			if response.Code != test.wantStatus {
				t.Fatalf("status = %d, want %d; body: %s", response.Code, test.wantStatus, response.Body)
			}
			if test.wantStatus != http.StatusOK && detector.called {
				t.Fatal("detector was called for an invalid upload")
			}
		})
	}
}

func TestDetectRejectsNonMultipartRequest(t *testing.T) {
	detector := &detectionStub{}
	request := httptest.NewRequest(http.MethodPost, "/api/v1/biometrics/detect", strings.NewReader("body"))
	response := httptest.NewRecorder()

	NewHandler(detector, config.DefaultMaxImageBytes).ServeHTTP(response, request)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusBadRequest)
	}
}

func TestDetectReturnsSanitizedEngineError(t *testing.T) {
	detector := &detectionStub{err: errors.New("internal engine trace")}
	request := detectionRequest(t, testPNG(t), "face.png")
	response := httptest.NewRecorder()

	NewHandler(detector, config.DefaultMaxImageBytes).ServeHTTP(response, request)

	if response.Code != http.StatusBadGateway {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusBadGateway)
	}
	if strings.Contains(response.Body.String(), "internal engine trace") {
		t.Fatal("response leaked internal engine error")
	}
	if body := response.Body.String(); body != "{\"error\":\"biometrics_engine_error\"}\n" {
		t.Fatalf("body = %q, want sanitized error", body)
	}
}

func TestDetectRejectsUnexpectedEngineResponse(t *testing.T) {
	tests := []string{
		`not-json`,
		`{"faces":null,"count":0}`,
		`{"faces":[]}`,
		`{"faces":[],"count":1}`,
		`{"faces":[null],"count":1}`,
		`{"faces":[{}],"count":1}`,
	}

	for _, result := range tests {
		t.Run(result, func(t *testing.T) {
			detector := &detectionStub{response: []byte(result)}
			request := detectionRequest(t, testPNG(t), "face.png")
			response := httptest.NewRecorder()

			NewHandler(detector, config.DefaultMaxImageBytes).ServeHTTP(response, request)

			if response.Code != http.StatusBadGateway {
				t.Fatalf("status = %d, want %d", response.Code, http.StatusBadGateway)
			}
		})
	}
}

func TestDetectRejectsWrongMethod(t *testing.T) {
	request := detectionRequest(t, testPNG(t), "face.png")
	request.Method = http.MethodGet
	response := httptest.NewRecorder()

	NewHandler(&detectionStub{}, config.DefaultMaxImageBytes).ServeHTTP(response, request)

	if response.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusMethodNotAllowed)
	}
}

func TestDetectionResponseIsJSON(t *testing.T) {
	detector := &detectionStub{response: []byte(`{"faces":[],"count":0}`)}
	request := detectionRequest(t, testPNG(t), "face.png")
	response := httptest.NewRecorder()

	NewHandler(detector, config.DefaultMaxImageBytes).ServeHTTP(response, request)

	var result map[string]json.RawMessage
	if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil {
		t.Fatalf("response is not valid JSON: %v", err)
	}
}

func detectionRequest(t *testing.T, imageData []byte, filename string) *http.Request {
	t.Helper()
	return detectionRequestWithField(t, imageData, filename, "image")
}

func detectionRequestWithField(t *testing.T, imageData []byte, filename, fieldName string) *http.Request {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile(fieldName, filename)
	if err != nil {
		t.Fatalf("CreateFormFile() error = %v", err)
	}
	if _, err := part.Write(imageData); err != nil {
		t.Fatalf("write image form field: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close multipart writer: %v", err)
	}

	request := httptest.NewRequest(http.MethodPost, "/api/v1/biometrics/detect", &body)
	request.Header.Set("Content-Type", writer.FormDataContentType())
	return request
}

func testPNG(t *testing.T) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 2, 2))
	img.Set(0, 0, color.RGBA{R: 255, A: 255})

	var data bytes.Buffer
	if err := png.Encode(&data, img); err != nil {
		t.Fatalf("encode test PNG: %v", err)
	}
	return data.Bytes()
}

func testJPEG(t *testing.T) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 2, 2))
	img.Set(0, 0, color.RGBA{R: 255, A: 255})

	var data bytes.Buffer
	if err := jpeg.Encode(&data, img, nil); err != nil {
		t.Fatalf("encode test JPEG: %v", err)
	}
	return data.Bytes()
}
