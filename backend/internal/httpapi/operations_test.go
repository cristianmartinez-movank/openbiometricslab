package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"openbiometrics-lab/backend/internal/biometrics"
	"openbiometrics-lab/backend/internal/config"
)

type operationStub struct {
	engineHealthStub
	body                 []byte
	err                  error
	verifyCalled         bool
	verifyThreshold      float64
	verifyImage1         []byte
	verifyImage2         []byte
	documentCalled       bool
	documentImage        []byte
	documentPresentation string
	preset               string
	sessionID            string
	livenessFrameCall    bool
}

func (s *operationStub) Verify(_ context.Context, _ string, image1 []byte, _ string, image2 []byte, threshold float64) ([]byte, error) {
	s.verifyCalled = true
	s.verifyThreshold = threshold
	s.verifyImage1 = image1
	s.verifyImage2 = image2
	return s.body, s.err
}

func (s *operationStub) VerifyDocument(_ context.Context, _ string, document []byte, _ string, selfie []byte, threshold float64) ([]byte, error) {
	s.documentCalled = true
	s.verifyThreshold = threshold
	s.verifyImage1 = document
	s.documentImage = document
	s.verifyImage2 = selfie
	return s.body, s.err
}

func (s *operationStub) ScanDocument(_ context.Context, _ string, imageData []byte, presentation string) ([]byte, error) {
	s.documentCalled = true
	s.documentImage = imageData
	s.documentPresentation = presentation
	return s.body, s.err
}

func (s *operationStub) LivenessPresets(context.Context) ([]byte, error) {
	return s.body, s.err
}

func (s *operationStub) CreateLivenessSession(_ context.Context, preset string) ([]byte, error) {
	s.preset = preset
	return s.body, s.err
}

func (s *operationStub) SubmitLivenessFrame(_ context.Context, sessionID, _ string, imageData []byte) ([]byte, error) {
	s.sessionID = sessionID
	s.documentImage = imageData
	s.livenessFrameCall = true
	return s.body, s.err
}

func (s *operationStub) GetLivenessSession(_ context.Context, sessionID string) ([]byte, error) {
	s.sessionID = sessionID
	return s.body, s.err
}

func (s *operationStub) CancelLivenessSession(_ context.Context, sessionID string) ([]byte, error) {
	s.sessionID = sessionID
	return s.body, s.err
}

func TestVerifyAndDocumentRoutes(t *testing.T) {
	firstImage, secondImage := testJPEG(t), testPNG(t)
	tests := []struct {
		name        string
		path        string
		files       map[string][]byte
		fields      map[string]string
		response    string
		verify      bool
		wantField1  []byte
		wantField2  []byte
		wantDefault bool
	}{
		{
			name:       "face verification",
			path:       "/api/v1/biometrics/verify",
			files:      map[string][]byte{"image1": firstImage, "image2": secondImage},
			fields:     map[string]string{"threshold": "0.7"},
			response:   `{"is_match":true,"similarity":0.9}`,
			verify:     true,
			wantField1: firstImage,
			wantField2: secondImage,
		},
		{
			name:       "document scan",
			path:       "/api/v1/biometrics/documents/scan",
			files:      map[string][]byte{"image": secondImage},
			response:   `{"detected":true,"ocr":null,"mrz":null,"has_face":false}`,
			wantField1: secondImage,
		},
		{
			name:        "document verification",
			path:        "/api/v1/biometrics/documents/verify",
			files:       map[string][]byte{"document": secondImage, "selfie": firstImage},
			response:    `{"is_match":true,"similarity":0.9}`,
			verify:      true,
			wantField1:  secondImage,
			wantField2:  firstImage,
			wantDefault: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			stub := &operationStub{body: []byte(test.response)}
			request := operationRequest(t, test.path, test.files, test.fields)
			response := httptest.NewRecorder()

			NewHandler(stub, config.DefaultMaxImageBytes).ServeHTTP(response, request)

			if response.Code != http.StatusOK {
				t.Fatalf("status = %d, want %d; body: %s", response.Code, http.StatusOK, response.Body)
			}
			if test.verify {
				if !stub.verifyCalled && test.path == "/api/v1/biometrics/verify" {
					t.Fatal("face verifier was not called")
				}
				if !bytes.Equal(stub.verifyImage1, test.wantField1) || !bytes.Equal(stub.verifyImage2, test.wantField2) {
					t.Fatal("verification images were not forwarded in the expected order")
				}
				wantThreshold := 0.7
				if test.wantDefault {
					wantThreshold = 0.4
				}
				if stub.verifyThreshold != wantThreshold {
					t.Fatalf("threshold = %v, want %v", stub.verifyThreshold, wantThreshold)
				}
			} else if !stub.documentCalled || !bytes.Equal(stub.documentImage, test.wantField1) {
				t.Fatal("document image was not forwarded")
			}
		})
	}
}

func TestDocumentScanAddsColombianIDValidation(t *testing.T) {
	stub := &operationStub{body: []byte(
		`{"detected":true,"ocr":{"full_text":"NUMERO 12345678\nAPELLIDOS PEREZ\nNOMBRES ANA\nFECHA DE NACIMIENTO 02/01/1990","confidence":0.9},"mrz":null,"has_face":true}`,
	)}
	request := operationRequest(t, "/api/v1/biometrics/documents/scan", map[string][]byte{
		"image": testPNG(t),
	}, map[string]string{"presentation": "digital"})
	response := httptest.NewRecorder()

	NewHandler(stub, config.DefaultMaxImageBytes).ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body: %s", response.Code, http.StatusOK, response.Body)
	}
	var body struct {
		ColombianID struct {
			Presentation   string  `json:"presentation"`
			DocumentNumber *string `json:"document_number"`
			GivenNames     *string `json:"given_names"`
			BirthDate      *string `json:"birth_date"`
		} `json:"colombian_id"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.ColombianID.Presentation != "digital" {
		t.Fatalf("presentation = %q, want digital", body.ColombianID.Presentation)
	}
	if stub.documentPresentation != "digital" {
		t.Fatalf("engine presentation = %q, want digital", stub.documentPresentation)
	}
	if body.ColombianID.DocumentNumber == nil || *body.ColombianID.DocumentNumber != "12345678" {
		t.Fatalf("document number = %v, want 12345678", body.ColombianID.DocumentNumber)
	}
	if body.ColombianID.GivenNames == nil || *body.ColombianID.GivenNames != "ANA" {
		t.Fatalf("given names = %v, want ANA", body.ColombianID.GivenNames)
	}
	if body.ColombianID.BirthDate == nil || *body.ColombianID.BirthDate != "1990-01-02" {
		t.Fatalf("birth date = %v, want 1990-01-02", body.ColombianID.BirthDate)
	}
}

func TestDocumentScanRejectsUnsupportedPresentation(t *testing.T) {
	stub := &operationStub{body: []byte(`{"detected":false,"ocr":null,"mrz":null,"has_face":false}`)}
	request := operationRequest(t, "/api/v1/biometrics/documents/scan", map[string][]byte{
		"image": testPNG(t),
	}, map[string]string{"presentation": "passport"})
	response := httptest.NewRecorder()

	NewHandler(stub, config.DefaultMaxImageBytes).ServeHTTP(response, request)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusBadRequest)
	}
	if stub.documentCalled {
		t.Fatal("scanner was called for an unsupported document presentation")
	}
}

func TestLivenessSessionRoutes(t *testing.T) {
	const sessionID = "7c45542a-f5a2-4e31-bd6e-c54a8c9a50c2"
	tests := []struct {
		name         string
		method       string
		path         string
		files        map[string][]byte
		wantPreset   string
		wantFrame    bool
		responseBody string
	}{
		{
			name:         "presets",
			method:       http.MethodGet,
			path:         "/api/v1/biometrics/liveness/presets",
			responseBody: `[{"name":"head_turn"}]`,
		},
		{
			name:         "create session",
			method:       http.MethodPost,
			path:         "/api/v1/biometrics/liveness/sessions?preset=head_turn",
			wantPreset:   "head_turn",
			responseBody: `{"session_id":"` + sessionID + `","state":"pending","challenges":[]}`,
		},
		{
			name:         "submit frame",
			method:       http.MethodPost,
			path:         "/api/v1/biometrics/liveness/sessions/" + sessionID + "/frame",
			files:        map[string][]byte{"image": testPNG(t)},
			wantFrame:    true,
			responseBody: `{"session_id":"` + sessionID + `","passed":true,"confidence":0.9,"challenges_remaining":0,"session_complete":true,"is_live":true,"state":"completed"}`,
		},
		{
			name:         "get session",
			method:       http.MethodGet,
			path:         "/api/v1/biometrics/liveness/sessions/" + sessionID,
			responseBody: `{"session_id":"` + sessionID + `","state":"pending","challenges":[]}`,
		},
		{
			name:         "cancel session",
			method:       http.MethodDelete,
			path:         "/api/v1/biometrics/liveness/sessions/" + sessionID,
			responseBody: `{"cancelled":"` + sessionID + `"}`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			stub := &operationStub{body: []byte(test.responseBody)}
			request := operationRequest(t, test.path, test.files, nil)
			request.Method = test.method
			response := httptest.NewRecorder()

			NewHandler(stub, config.DefaultMaxImageBytes).ServeHTTP(response, request)

			if response.Code != http.StatusOK {
				t.Fatalf("status = %d, want %d; body: %s", response.Code, http.StatusOK, response.Body)
			}
			if test.wantPreset != "" && stub.preset != test.wantPreset {
				t.Fatalf("preset = %q, want %q", stub.preset, test.wantPreset)
			}
			if test.wantFrame && (!stub.livenessFrameCall || stub.sessionID != sessionID) {
				t.Fatal("liveness frame was not forwarded to the expected session")
			}
		})
	}
}

func TestInvalidThresholdAndLivenessPresetAreRejected(t *testing.T) {
	tests := []struct {
		name   string
		path   string
		files  map[string][]byte
		fields map[string]string
	}{
		{
			name:   "NaN threshold",
			path:   "/api/v1/biometrics/verify",
			files:  map[string][]byte{"image1": testPNG(t), "image2": testPNG(t)},
			fields: map[string]string{"threshold": "NaN"},
		},
		{
			name: "passive preset cannot create an active session",
			path: "/api/v1/biometrics/liveness/sessions?preset=passive_only",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			stub := &operationStub{body: []byte(`{}`)}
			request := operationRequest(t, test.path, test.files, test.fields)
			response := httptest.NewRecorder()

			NewHandler(stub, config.DefaultMaxImageBytes).ServeHTTP(response, request)

			if response.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want %d", response.Code, http.StatusBadRequest)
			}
			if stub.verifyCalled || stub.documentCalled || stub.preset != "" {
				t.Fatal("engine was called for invalid request parameters")
			}
		})
	}
}

func TestBiometricsOperationReturnsSanitizedEngineError(t *testing.T) {
	stub := &operationStub{err: errors.New("private upstream error")}
	request := operationRequest(t, "/api/v1/biometrics/documents/scan", map[string][]byte{
		"image": testPNG(t),
	}, nil)
	response := httptest.NewRecorder()

	NewHandler(stub, config.DefaultMaxImageBytes).ServeHTTP(response, request)

	if response.Code != http.StatusBadGateway {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusBadGateway)
	}
	if bytes.Contains(response.Body.Bytes(), []byte("private upstream error")) {
		t.Fatal("response leaked internal engine error")
	}
}

func TestDocumentVerificationReturnsActionableFaceErrors(t *testing.T) {
	tests := []struct {
		detail string
		code   string
	}{
		{detail: "No face found on document", code: "document_face_not_found"},
		{detail: "Could not process document face", code: "document_face_not_detected"},
		{detail: "No face detected in selfie", code: "selfie_face_not_detected"},
	}
	for _, test := range tests {
		t.Run(test.code, func(t *testing.T) {
			stub := &operationStub{err: &biometrics.EngineHTTPError{
				StatusCode: http.StatusBadRequest,
				Detail:     test.detail,
			}}
			request := operationRequest(t, "/api/v1/biometrics/documents/verify", map[string][]byte{
				"document": testPNG(t),
				"selfie":   testJPEG(t),
			}, nil)
			response := httptest.NewRecorder()

			NewHandler(stub, config.DefaultMaxImageBytes).ServeHTTP(response, request)

			if response.Code != http.StatusUnprocessableEntity {
				t.Fatalf("status = %d, want %d; body: %s", response.Code, http.StatusUnprocessableEntity, response.Body)
			}
			if body := response.Body.String(); body != `{"error":"`+test.code+`"}`+"\n" {
				t.Fatalf("body = %q, want error code %q", body, test.code)
			}
		})
	}
}

func operationRequest(
	t *testing.T,
	target string,
	files map[string][]byte,
	fields map[string]string,
) *http.Request {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	for name, data := range files {
		part, err := writer.CreateFormFile(name, "upload.bin")
		if err != nil {
			t.Fatalf("CreateFormFile(%q) error = %v", name, err)
		}
		if _, err := part.Write(data); err != nil {
			t.Fatalf("write %q: %v", name, err)
		}
	}
	for name, value := range fields {
		if err := writer.WriteField(name, value); err != nil {
			t.Fatalf("WriteField(%q) error = %v", name, err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close multipart writer: %v", err)
	}
	request := httptest.NewRequest(http.MethodPost, target, &body)
	request.Header.Set("Content-Type", writer.FormDataContentType())
	return request
}
