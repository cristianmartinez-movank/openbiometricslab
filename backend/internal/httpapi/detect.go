package httpapi

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
)

const (
	maxMultipartOverhead int64 = 64 << 10
	maxImagePixels             = 25_000_000
)

type detectionResponse struct {
	Faces json.RawMessage `json:"faces"`
	Count *int            `json:"count"`
}

type FaceDetector interface {
	Detect(context.Context, string, []byte) ([]byte, error)
}

func detect(detector FaceDetector, maxImageBytes int64) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		images, err := readMultipartImages(w, r, maxImageBytes, "image")
		if err != nil {
			writeRequestError(w, err)
			return
		}

		image := images["image"]
		result, engineErr := detector.Detect(r.Context(), image.contentType, image.data)
		if engineErr != nil {
			log.Printf("OpenBiometrics detection request failed: %v", engineErr)
			writeError(w, http.StatusBadGateway, "biometrics_engine_error")
			return
		}
		if err := validateDetectionPayload(result); err != nil {
			log.Printf("OpenBiometrics returned an invalid detection response")
			writeError(w, http.StatusBadGateway, "invalid_engine_response")
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(result)
	}
}

func validateDetectionPayload(data []byte) error {
	var result detectionResponse
	if err := json.Unmarshal(data, &result); err != nil {
		return fmt.Errorf("decode JSON: %w", err)
	}
	var faces []json.RawMessage
	if err := json.Unmarshal(result.Faces, &faces); err != nil {
		return fmt.Errorf("decode faces: %w", err)
	}
	if faces == nil || result.Count == nil || *result.Count != len(faces) {
		return fmt.Errorf("count must match the number of faces")
	}
	for index, face := range faces {
		var fields map[string]json.RawMessage
		if err := json.Unmarshal(face, &fields); err != nil || fields == nil {
			return fmt.Errorf("face at index %d must be an object", index)
		}
		detection, ok := fields["detection"]
		var detectionObject map[string]json.RawMessage
		if !ok || json.Unmarshal(detection, &detectionObject) != nil || detectionObject == nil {
			return fmt.Errorf("face at index %d has no detection object", index)
		}
	}

	return nil
}

func writeError(w http.ResponseWriter, status int, code string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(struct {
		Error string `json:"error"`
	}{Error: code})
}
