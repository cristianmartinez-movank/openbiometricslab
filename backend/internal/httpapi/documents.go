package httpapi

import (
	"context"
	"encoding/json"
	"log"
	"net/http"

	"openbiometrics-lab/backend/internal/documents"
)

type DocumentScanner interface {
	ScanDocument(context.Context, string, []byte, string) ([]byte, error)
}

func scanDocument(scanner DocumentScanner, maxImageBytes int64) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		images, err := readMultipartImageFields(w, r, maxImageBytes, []string{"image"}, []string{"back_image"})
		if err != nil {
			writeRequestError(w, err)
			return
		}
		image := images["image"]
		presentation, present, fieldErr := multipartField(r, "presentation")
		if fieldErr != nil {
			writeRequestError(w, fieldErr)
			return
		}
		if present && presentation != "physical" && presentation != "digital" {
			writeError(w, http.StatusBadRequest, "invalid_document_presentation")
			return
		}
		if !present {
			presentation = "physical"
		}
		result, engineErr := scanner.ScanDocument(r.Context(), image.contentType, image.data, presentation)
		if engineErr != nil {
			log.Printf("OpenBiometrics document scan request failed: %v", engineErr)
			writeError(w, http.StatusBadGateway, "biometrics_engine_error")
			return
		}
		if backImage, ok := images["back_image"]; ok {
			backResult, backErr := scanner.ScanDocument(r.Context(), backImage.contentType, backImage.data, presentation)
			if backErr != nil {
				log.Printf("OpenBiometrics document back scan request failed: %v", backErr)
				writeError(w, http.StatusBadGateway, "biometrics_engine_error")
				return
			}
			merged, mergeErr := documents.MergeScanResponses(result, backResult)
			if mergeErr != nil {
				log.Printf("Could not combine OpenBiometrics document sides: %v", mergeErr)
				writeError(w, http.StatusBadGateway, "invalid_engine_response")
				return
			}
			result = merged
		}
		enriched, enrichErr := documents.EnrichScanResponse(result, presentation)
		if enrichErr != nil {
			log.Printf("Could not enrich OpenBiometrics document response: %v", enrichErr)
			writeError(w, http.StatusBadGateway, "invalid_engine_response")
			return
		}
		writeJSONResult(w, enriched)
	}
}

func writeJSONResult(w http.ResponseWriter, result []byte) {
	if !json.Valid(result) {
		log.Printf("OpenBiometrics returned an invalid JSON response")
		writeError(w, http.StatusBadGateway, "invalid_engine_response")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(result)
}
