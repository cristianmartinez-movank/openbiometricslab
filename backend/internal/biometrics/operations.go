package biometrics

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"net/url"
	"strconv"
)

const maxOperationResponseBytes = 1 << 20
const maxEngineErrorResponseBytes = 4 << 10

type imagePart struct {
	name        string
	contentType string
	data        []byte
}

type EngineHTTPError struct {
	StatusCode int
	Detail     string
}

func (e *EngineHTTPError) Error() string {
	return fmt.Sprintf("OpenBiometrics returned HTTP %d", e.StatusCode)
}

func (c *Client) Verify(
	ctx context.Context,
	contentType1 string,
	image1 []byte,
	contentType2 string,
	image2 []byte,
	threshold float64,
) ([]byte, error) {
	body, contentType, err := buildMultipart(
		[]imagePart{
			{name: "image1", contentType: contentType1, data: image1},
			{name: "image2", contentType: contentType2, data: image2},
		},
		url.Values{"threshold": {strconv.FormatFloat(threshold, 'f', -1, 64)}},
	)
	if err != nil {
		return nil, err
	}

	result, err := c.requestJSON(ctx, http.MethodPost, "/api/v1/verify", body, contentType)
	if err != nil {
		return nil, err
	}
	if err := validateVerifyResponse(result); err != nil {
		return nil, fmt.Errorf("invalid OpenBiometrics verification response: %w", err)
	}
	return result, nil
}

func (c *Client) ScanDocument(
	ctx context.Context,
	contentType string,
	imageData []byte,
	presentation string,
) ([]byte, error) {
	body, multipartType, err := buildMultipart(
		[]imagePart{{name: "image", contentType: contentType, data: imageData}},
		url.Values{"presentation": {presentation}},
	)
	if err != nil {
		return nil, err
	}

	result, err := c.requestJSON(ctx, http.MethodPost, "/api/v1/documents/scan", body, multipartType)
	if err != nil {
		return nil, err
	}
	if err := validateDocumentScanResponse(result); err != nil {
		return nil, fmt.Errorf("invalid OpenBiometrics document response: %w", err)
	}
	return result, nil
}

func (c *Client) VerifyDocument(
	ctx context.Context,
	documentContentType string,
	document []byte,
	selfieContentType string,
	selfie []byte,
	threshold float64,
) ([]byte, error) {
	body, contentType, err := buildMultipart(
		[]imagePart{
			{name: "document", contentType: documentContentType, data: document},
			{name: "selfie", contentType: selfieContentType, data: selfie},
		},
		url.Values{"threshold": {strconv.FormatFloat(threshold, 'f', -1, 64)}},
	)
	if err != nil {
		return nil, err
	}

	result, err := c.requestJSON(ctx, http.MethodPost, "/api/v1/documents/verify", body, contentType)
	if err != nil {
		return nil, err
	}
	if err := validateVerifyResponse(result); err != nil {
		return nil, fmt.Errorf("invalid OpenBiometrics document verification response: %w", err)
	}
	return result, nil
}

func (c *Client) LivenessPresets(ctx context.Context) ([]byte, error) {
	result, err := c.requestJSON(ctx, http.MethodGet, "/api/v1/liveness/presets", nil, "")
	if err != nil {
		return nil, err
	}
	if err := validateLivenessPresets(result); err != nil {
		return nil, fmt.Errorf("invalid OpenBiometrics liveness presets response: %w", err)
	}
	return result, nil
}

func (c *Client) CreateLivenessSession(ctx context.Context, preset string) ([]byte, error) {
	target := "/api/v1/liveness/sessions"
	if preset != "" {
		query := url.Values{"preset": {preset}}
		target += "?" + query.Encode()
	}
	result, err := c.requestJSON(ctx, http.MethodPost, target, nil, "")
	if err != nil {
		return nil, err
	}
	if err := validateLivenessSessionResponse(result); err != nil {
		return nil, fmt.Errorf("invalid OpenBiometrics liveness session response: %w", err)
	}
	return result, nil
}

func (c *Client) SubmitLivenessFrame(
	ctx context.Context,
	sessionID string,
	contentType string,
	imageData []byte,
) ([]byte, error) {
	body, multipartType, err := buildMultipart(
		[]imagePart{{name: "image", contentType: contentType, data: imageData}},
		nil,
	)
	if err != nil {
		return nil, err
	}
	target := "/api/v1/liveness/sessions/" + url.PathEscape(sessionID) + "/frame"
	result, err := c.requestJSON(ctx, http.MethodPost, target, body, multipartType)
	if err != nil {
		return nil, err
	}
	if err := validateLivenessFrameResponse(result); err != nil {
		return nil, fmt.Errorf("invalid OpenBiometrics liveness frame response: %w", err)
	}
	return result, nil
}

func (c *Client) GetLivenessSession(ctx context.Context, sessionID string) ([]byte, error) {
	target := "/api/v1/liveness/sessions/" + url.PathEscape(sessionID)
	result, err := c.requestJSON(ctx, http.MethodGet, target, nil, "")
	if err != nil {
		return nil, err
	}
	if err := validateLivenessSessionResponse(result); err != nil {
		return nil, fmt.Errorf("invalid OpenBiometrics liveness session response: %w", err)
	}
	return result, nil
}

func (c *Client) CancelLivenessSession(ctx context.Context, sessionID string) ([]byte, error) {
	target := "/api/v1/liveness/sessions/" + url.PathEscape(sessionID)
	result, err := c.requestJSON(ctx, http.MethodDelete, target, nil, "")
	if err != nil {
		return nil, err
	}
	fields, err := decodeJSONObject(result)
	if err != nil {
		return nil, err
	}
	if err := requireString(fields, "cancelled"); err != nil {
		return nil, err
	}
	return result, nil
}

func (c *Client) requestJSON(
	ctx context.Context,
	method string,
	path string,
	body io.Reader,
	contentType string,
) ([]byte, error) {
	request, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, body)
	if err != nil {
		return nil, fmt.Errorf("create OpenBiometrics request: %w", err)
	}
	if contentType != "" {
		request.Header.Set("Content-Type", contentType)
	}

	response, err := c.httpClient.Do(request)
	if err != nil {
		return nil, fmt.Errorf("request OpenBiometrics: %w", err)
	}
	defer response.Body.Close()

	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		if response.StatusCode >= http.StatusBadRequest && response.StatusCode < http.StatusInternalServerError {
			body, err := io.ReadAll(io.LimitReader(response.Body, maxEngineErrorResponseBytes+1))
			if err != nil {
				return nil, fmt.Errorf("read OpenBiometrics error response: %w", err)
			}
			if len(body) <= maxEngineErrorResponseBytes {
				var errorBody struct {
					Detail string `json:"detail"`
				}
				if json.Unmarshal(body, &errorBody) == nil {
					return nil, &EngineHTTPError{
						StatusCode: response.StatusCode,
						Detail:     errorBody.Detail,
					}
				}
			}
		}
		return nil, fmt.Errorf("OpenBiometrics returned HTTP %d", response.StatusCode)
	}
	responseContentType, _, err := mime.ParseMediaType(response.Header.Get("Content-Type"))
	if err != nil || responseContentType != "application/json" {
		return nil, fmt.Errorf("OpenBiometrics returned a non-JSON response")
	}

	result, err := io.ReadAll(io.LimitReader(response.Body, maxOperationResponseBytes+1))
	if err != nil {
		return nil, fmt.Errorf("read OpenBiometrics response: %w", err)
	}
	if len(result) > maxOperationResponseBytes {
		return nil, fmt.Errorf("OpenBiometrics response exceeds %d bytes", maxOperationResponseBytes)
	}
	if !json.Valid(result) {
		return nil, fmt.Errorf("OpenBiometrics returned invalid JSON")
	}
	return result, nil
}

func buildMultipart(images []imagePart, values url.Values) (*bytes.Buffer, string, error) {
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	for _, image := range images {
		header := make(textproto.MIMEHeader)
		header.Set("Content-Disposition", mime.FormatMediaType("form-data", map[string]string{
			"name":     image.name,
			"filename": imageFilename(image.contentType),
		}))
		header.Set("Content-Type", image.contentType)
		part, err := writer.CreatePart(header)
		if err != nil {
			return nil, "", fmt.Errorf("create OpenBiometrics image field: %w", err)
		}
		if _, err := part.Write(image.data); err != nil {
			return nil, "", fmt.Errorf("write OpenBiometrics image field: %w", err)
		}
	}
	for name, entries := range values {
		for _, value := range entries {
			if err := writer.WriteField(name, value); err != nil {
				return nil, "", fmt.Errorf("write OpenBiometrics form field: %w", err)
			}
		}
	}
	if err := writer.Close(); err != nil {
		return nil, "", fmt.Errorf("finish OpenBiometrics request: %w", err)
	}
	return &body, writer.FormDataContentType(), nil
}

func decodeJSONObject(body []byte) (map[string]json.RawMessage, error) {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(body, &fields); err != nil {
		return nil, fmt.Errorf("decode JSON object: %w", err)
	}
	if fields == nil {
		return nil, fmt.Errorf("response must be a JSON object")
	}
	return fields, nil
}

func requireString(fields map[string]json.RawMessage, name string) error {
	var value string
	if err := json.Unmarshal(fields[name], &value); err != nil || value == "" {
		return fmt.Errorf("%s must be a non-empty string", name)
	}
	return nil
}

func requireBool(fields map[string]json.RawMessage, name string) error {
	var value bool
	if err := json.Unmarshal(fields[name], &value); err != nil {
		return fmt.Errorf("%s must be a boolean", name)
	}
	return nil
}

func requireNumber(fields map[string]json.RawMessage, name string) error {
	var value float64
	if err := json.Unmarshal(fields[name], &value); err != nil {
		return fmt.Errorf("%s must be a number", name)
	}
	return nil
}

func validateVerifyResponse(body []byte) error {
	fields, err := decodeJSONObject(body)
	if err != nil {
		return err
	}
	if err := requireBool(fields, "is_match"); err != nil {
		return err
	}
	return requireNumber(fields, "similarity")
}

func validateDocumentScanResponse(body []byte) error {
	fields, err := decodeJSONObject(body)
	if err != nil {
		return err
	}
	if err := requireBool(fields, "detected"); err != nil {
		return err
	}
	return requireBool(fields, "has_face")
}

func validateLivenessPresets(body []byte) error {
	var presets []json.RawMessage
	if err := json.Unmarshal(body, &presets); err != nil || presets == nil {
		return fmt.Errorf("presets must be an array")
	}
	for index, preset := range presets {
		fields, err := decodeJSONObject(preset)
		if err != nil {
			return fmt.Errorf("preset at index %d must be an object", index)
		}
		for _, name := range []string{"name", "description"} {
			if err := requireString(fields, name); err != nil {
				return fmt.Errorf("preset at index %d: %w", index, err)
			}
		}
		if err := requireNumber(fields, "num_challenges"); err != nil {
			return fmt.Errorf("preset at index %d: %w", index, err)
		}
		if err := requireNumber(fields, "timeout_seconds"); err != nil {
			return fmt.Errorf("preset at index %d: %w", index, err)
		}
		if err := requireBool(fields, "require_passive"); err != nil {
			return fmt.Errorf("preset at index %d: %w", index, err)
		}
		var challengeTypes []json.RawMessage
		if err := json.Unmarshal(fields["challenge_types"], &challengeTypes); err != nil || challengeTypes == nil {
			return fmt.Errorf("preset at index %d: challenge_types must be an array", index)
		}
	}
	return nil
}

func validateLivenessSessionResponse(body []byte) error {
	fields, err := decodeJSONObject(body)
	if err != nil {
		return err
	}
	for _, name := range []string{"session_id", "state"} {
		if err := requireString(fields, name); err != nil {
			return err
		}
	}
	var challenges []json.RawMessage
	if err := json.Unmarshal(fields["challenges"], &challenges); err != nil || challenges == nil {
		return fmt.Errorf("challenges must be an array")
	}
	for index, challenge := range challenges {
		challengeFields, err := decodeJSONObject(challenge)
		if err != nil {
			return fmt.Errorf("challenge at index %d must be an object", index)
		}
		for _, name := range []string{"type", "instruction"} {
			if err := requireString(challengeFields, name); err != nil {
				return fmt.Errorf("challenge at index %d: %w", index, err)
			}
		}
		if err := requireNumber(challengeFields, "timeout_seconds"); err != nil {
			return fmt.Errorf("challenge at index %d: %w", index, err)
		}
	}
	return nil
}

func validateLivenessFrameResponse(body []byte) error {
	fields, err := decodeJSONObject(body)
	if err != nil {
		return err
	}
	if err := requireString(fields, "session_id"); err != nil {
		return err
	}
	if err := requireBool(fields, "passed"); err != nil {
		return err
	}
	for _, name := range []string{"confidence", "challenges_remaining"} {
		if err := requireNumber(fields, name); err != nil {
			return err
		}
	}
	if err := requireBool(fields, "session_complete"); err != nil {
		return err
	}
	return requireString(fields, "state")
}
