package documents

import (
	"encoding/json"
	"fmt"
)

func EnrichScanResponse(body []byte, presentation string) ([]byte, error) {
	var response map[string]json.RawMessage
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("decode document scan response: %w", err)
	}
	if response == nil {
		return nil, fmt.Errorf("document scan response must be an object")
	}
	var ocr *struct {
		FullText string `json:"full_text"`
	}
	if raw, ok := response["ocr"]; ok && string(raw) != "null" {
		var parsed struct {
			FullText string `json:"full_text"`
		}
		if err := json.Unmarshal(raw, &parsed); err != nil {
			return nil, fmt.Errorf("decode document OCR response: %w", err)
		}
		ocr = &parsed
	}
	text := ""
	if ocr != nil {
		text = ocr.FullText
	}
	extraction := ExtractColombianID(text, presentation)
	encoded, err := json.Marshal(extraction)
	if err != nil {
		return nil, fmt.Errorf("encode Colombian ID extraction: %w", err)
	}
	response["colombian_id"] = encoded
	enriched, err := json.Marshal(response)
	if err != nil {
		return nil, fmt.Errorf("encode document scan response: %w", err)
	}
	return enriched, nil
}
