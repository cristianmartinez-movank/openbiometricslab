package documents

import (
	"encoding/json"
	"fmt"
	"strings"
)

func MergeScanResponses(front, back []byte) ([]byte, error) {
	var frontFields, backFields map[string]json.RawMessage
	if err := json.Unmarshal(front, &frontFields); err != nil || frontFields == nil {
		return nil, fmt.Errorf("decode front document scan")
	}
	if err := json.Unmarshal(back, &backFields); err != nil || backFields == nil {
		return nil, fmt.Errorf("decode back document scan")
	}

	var frontScan, backScan scanResponse
	if err := json.Unmarshal(front, &frontScan); err != nil {
		return nil, fmt.Errorf("decode front document scan: %w", err)
	}
	if err := json.Unmarshal(back, &backScan); err != nil {
		return nil, fmt.Errorf("decode back document scan: %w", err)
	}
	detected, err := json.Marshal(frontScan.Detected || backScan.Detected)
	if err != nil {
		return nil, fmt.Errorf("encode merged document detection status: %w", err)
	}
	hasFace, err := json.Marshal(frontScan.HasFace)
	if err != nil {
		return nil, fmt.Errorf("encode merged face status: %w", err)
	}
	frontFields["detected"] = detected
	frontFields["has_face"] = hasFace
	if frontScan.OCR == nil {
		frontScan.OCR = backScan.OCR
	} else if backScan.OCR != nil {
		frontScan.OCR.FullText = strings.TrimSpace(frontScan.OCR.FullText + "\n" + backScan.OCR.FullText)
		totalLines := len(frontScan.OCR.Lines) + len(backScan.OCR.Lines)
		if totalLines > 0 {
			weightedConfidence := frontScan.OCR.Confidence*float64(len(frontScan.OCR.Lines)) +
				backScan.OCR.Confidence*float64(len(backScan.OCR.Lines))
			frontScan.OCR.Confidence = weightedConfidence / float64(totalLines)
		}
		frontScan.OCR.Lines = append(frontScan.OCR.Lines, backScan.OCR.Lines...)
	}
	if frontScan.OCR != nil {
		ocr, err := json.Marshal(frontScan.OCR)
		if err != nil {
			return nil, fmt.Errorf("encode merged document OCR: %w", err)
		}
		frontFields["ocr"] = ocr
	} else {
		frontFields["ocr"] = json.RawMessage("null")
	}
	if string(frontScan.MRZ) == "" || string(frontScan.MRZ) == "null" {
		if len(backScan.MRZ) == 0 {
			frontFields["mrz"] = json.RawMessage("null")
		} else {
			frontFields["mrz"] = backScan.MRZ
		}
	}
	merged, err := json.Marshal(frontFields)
	if err != nil {
		return nil, fmt.Errorf("encode merged document scan: %w", err)
	}
	return merged, nil
}

type scanResponse struct {
	Detected bool             `json:"detected"`
	OCR      *scanOCRResponse `json:"ocr"`
	MRZ      json.RawMessage  `json:"mrz"`
	HasFace  bool             `json:"has_face"`
}

type scanOCRResponse struct {
	FullText   string            `json:"full_text"`
	Lines      []json.RawMessage `json:"lines"`
	Confidence float64           `json:"confidence"`
}
