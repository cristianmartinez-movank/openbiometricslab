package httpapi

import (
	"context"
	"errors"
	"log"
	"net/http"

	"openbiometrics-lab/backend/internal/biometrics"
)

type FaceVerifier interface {
	Verify(context.Context, string, []byte, string, []byte, float64) ([]byte, error)
}

type DocumentVerifier interface {
	VerifyDocument(context.Context, string, []byte, string, []byte, float64) ([]byte, error)
}

func verify(verifier FaceVerifier, maxImageBytes int64) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		images, err := readMultipartImages(w, r, maxImageBytes, "image1", "image2")
		if err != nil {
			writeRequestError(w, err)
			return
		}
		threshold, err := parseThreshold(r)
		if err != nil {
			writeRequestError(w, err)
			return
		}

		image1, image2 := images["image1"], images["image2"]
		result, engineErr := verifier.Verify(
			r.Context(),
			image1.contentType,
			image1.data,
			image2.contentType,
			image2.data,
			threshold,
		)
		writeBiometricsResult(w, "face verification", result, engineErr)
	}
}

func verifyDocument(verifier DocumentVerifier, maxImageBytes int64) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		images, err := readMultipartImages(w, r, maxImageBytes, "document", "selfie")
		if err != nil {
			writeRequestError(w, err)
			return
		}
		threshold, err := parseThreshold(r)
		if err != nil {
			writeRequestError(w, err)
			return
		}

		document, selfie := images["document"], images["selfie"]
		result, engineErr := verifier.VerifyDocument(
			r.Context(),
			document.contentType,
			document.data,
			selfie.contentType,
			selfie.data,
			threshold,
		)
		writeBiometricsResult(w, "document verification", result, engineErr)
	}
}

func writeBiometricsResult(w http.ResponseWriter, operation string, result []byte, err error) {
	if err != nil {
		log.Printf("OpenBiometrics %s request failed: %v", operation, err)
		var engineErr *biometrics.EngineHTTPError
		if errors.As(err, &engineErr) {
			if code := documentVerificationErrorCode(engineErr.Detail); code != "" {
				writeError(w, http.StatusUnprocessableEntity, code)
				return
			}
		}
		writeError(w, http.StatusBadGateway, "biometrics_engine_error")
		return
	}
	writeJSONResult(w, result)
}

func documentVerificationErrorCode(detail string) string {
	switch detail {
	case "No face found on document":
		return "document_face_not_found"
	case "Could not process document face":
		return "document_face_not_detected"
	case "No face detected in selfie":
		return "selfie_face_not_detected"
	default:
		return ""
	}
}
