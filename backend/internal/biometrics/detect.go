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
	"strings"
)

const maxDetectionResponseBytes = 1 << 20

type detectionResponse struct {
	Faces []json.RawMessage `json:"faces"`
	Count *int              `json:"count"`
}

func (c *Client) Detect(ctx context.Context, contentType string, imageData []byte) ([]byte, error) {
	var requestBody bytes.Buffer
	writer := multipart.NewWriter(&requestBody)

	header := make(textproto.MIMEHeader)
	header["Content-Disposition"] = []string{
		mime.FormatMediaType("form-data", map[string]string{
			"name":     "image",
			"filename": imageFilename(contentType),
		}),
	}
	header["Content-Type"] = []string{contentType}
	part, err := writer.CreatePart(header)
	if err != nil {
		return nil, fmt.Errorf("create OpenBiometrics image field: %w", err)
	}
	if _, err := part.Write(imageData); err != nil {
		return nil, fmt.Errorf("write OpenBiometrics image field: %w", err)
	}
	if err := writer.Close(); err != nil {
		return nil, fmt.Errorf("finish OpenBiometrics request: %w", err)
	}

	request, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		c.baseURL+"/api/v1/detect",
		&requestBody,
	)
	if err != nil {
		return nil, fmt.Errorf("create OpenBiometrics detection request: %w", err)
	}
	request.Header.Set("Content-Type", writer.FormDataContentType())

	response, err := c.httpClient.Do(request)
	if err != nil {
		return nil, fmt.Errorf("request OpenBiometrics detection: %w", err)
	}
	defer response.Body.Close()

	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("OpenBiometrics detection returned HTTP %d", response.StatusCode)
	}

	responseContentType, _, err := mime.ParseMediaType(response.Header.Get("Content-Type"))
	if err != nil || responseContentType != "application/json" {
		return nil, fmt.Errorf("OpenBiometrics detection returned a non-JSON response")
	}

	body, err := io.ReadAll(io.LimitReader(response.Body, maxDetectionResponseBytes+1))
	if err != nil {
		return nil, fmt.Errorf("read OpenBiometrics detection response: %w", err)
	}
	if len(body) > maxDetectionResponseBytes {
		return nil, fmt.Errorf("OpenBiometrics detection response exceeds %d bytes", maxDetectionResponseBytes)
	}
	if err := validateDetectionResponse(body); err != nil {
		return nil, fmt.Errorf("invalid OpenBiometrics detection response: %w", err)
	}

	return body, nil
}

func validateDetectionResponse(body []byte) error {
	var result detectionResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return fmt.Errorf("decode JSON: %w", err)
	}
	if result.Faces == nil {
		return fmt.Errorf("faces must be an array")
	}
	if result.Count == nil || *result.Count != len(result.Faces) {
		return fmt.Errorf("count must match the number of faces")
	}
	for index, face := range result.Faces {
		var fields map[string]json.RawMessage
		if err := json.Unmarshal(face, &fields); err != nil || fields == nil {
			return fmt.Errorf("face at index %d must be an object", index)
		}
		if detection, ok := fields["detection"]; !ok || !json.Valid(detection) || !strings.HasPrefix(strings.TrimSpace(string(detection)), "{") {
			return fmt.Errorf("face at index %d has no detection object", index)
		}
	}

	return nil
}

func imageFilename(contentType string) string {
	switch contentType {
	case "image/jpeg":
		return "image.jpg"
	case "image/png":
		return "image.png"
	default:
		return "image"
	}
}
