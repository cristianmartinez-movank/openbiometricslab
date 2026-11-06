package documents

import (
	"encoding/json"
	"testing"
)

func TestExtractColombianID(t *testing.T) {
	result := ExtractColombianID(
		"CEDULA DE CIUDADANIA\nNUMERO 1.234.567.890\nAPELLIDOS PEREZ GOMEZ\nNOMBRES ANA MARIA\nFECHA DE NACIMIENTO 02-ENE-1990\nFECHA Y LUGAR DE EXPEDICION 12-MAR-2015 BOGOTA",
		"physical",
	)
	if result.DocumentType != "CC" || result.Presentation != "physical" {
		t.Fatalf("document metadata = %q/%q", result.DocumentType, result.Presentation)
	}
	if result.DocumentNumber == nil || *result.DocumentNumber != "1234567890" {
		t.Fatalf("document number = %v, want 1234567890", result.DocumentNumber)
	}
	if result.NumberFormatValid == nil || !*result.NumberFormatValid {
		t.Fatal("document number format should be valid")
	}
	if result.Surnames == nil || *result.Surnames != "PEREZ GOMEZ" {
		t.Fatalf("surnames = %v, want PEREZ GOMEZ", result.Surnames)
	}
	if result.GivenNames == nil || *result.GivenNames != "ANA MARIA" {
		t.Fatalf("given names = %v, want ANA MARIA", result.GivenNames)
	}
	if result.BirthDate == nil || *result.BirthDate != "1990-01-02" {
		t.Fatalf("birth date = %v, want 1990-01-02", result.BirthDate)
	}
	if result.BirthDateValid == nil || !*result.BirthDateValid {
		t.Fatal("birth date should be valid")
	}
	if result.IssueDate == nil || *result.IssueDate != "2015-03-12" {
		t.Fatalf("issue date = %v, want 2015-03-12", result.IssueDate)
	}
	if result.IssueDateValid == nil || !*result.IssueDateValid {
		t.Fatal("issue date should be valid")
	}
	if result.AuthenticityVerified || result.RegistryVerified {
		t.Fatal("local OCR checks must not claim official authenticity verification")
	}
}

func TestExtractColombianIDReportsInvalidLocalFormats(t *testing.T) {
	result := ExtractColombianID(
		"NUMERO 12345\nFECHA DE NACIMIENTO 02/01/2020\nFECHA DE EXPEDICION 01/01/2010",
		"digital",
	)
	if result.NumberFormatValid == nil || *result.NumberFormatValid {
		t.Fatal("five-digit number should fail the local format check")
	}
	if result.BirthDate == nil || result.BirthDateValid == nil || !*result.BirthDateValid {
		t.Fatal("valid birth date should pass the local date check")
	}
	if result.IssueDateValid == nil || *result.IssueDateValid {
		t.Fatal("issue date before birth date should be invalid")
	}
}

func TestExtractColombianIDRejectsInvalidCalendarDate(t *testing.T) {
	result := ExtractColombianID("FECHA DE NACIMIENTO 31/02/2020", "physical")
	if result.BirthDate == nil || result.BirthDateValid == nil || *result.BirthDateValid {
		t.Fatal("invalid calendar date should be surfaced as invalid")
	}
}

func TestExtractColombianIDHandlesMissingOCR(t *testing.T) {
	result := ExtractColombianID("", "other")
	if result.Presentation != "unspecified" {
		t.Fatalf("presentation = %q, want unspecified", result.Presentation)
	}
	if result.DocumentNumber != nil || result.BirthDate != nil {
		t.Fatal("missing OCR must not fabricate extracted values")
	}
	if len(result.Warnings) == 0 {
		t.Fatal("missing OCR should include an explanation")
	}
}

func TestEnrichScanResponse(t *testing.T) {
	response, err := EnrichScanResponse(
		[]byte(`{"detected":true,"ocr":{"full_text":"NUMERO 12345678","confidence":0.9},"mrz":null,"has_face":true}`),
		"digital",
	)
	if err != nil {
		t.Fatalf("EnrichScanResponse() error = %v", err)
	}
	var decoded map[string]json.RawMessage
	if err := json.Unmarshal(response, &decoded); err != nil {
		t.Fatalf("decode enriched response: %v", err)
	}
	var extraction ColombianIDExtraction
	if err := json.Unmarshal(decoded["colombian_id"], &extraction); err != nil {
		t.Fatalf("decode Colombian ID extraction: %v", err)
	}
	if extraction.Presentation != "digital" || extraction.DocumentNumber == nil {
		t.Fatalf("unexpected extraction: %+v", extraction)
	}
	if string(decoded["ocr"]) == "null" {
		t.Fatal("existing OCR response must be preserved")
	}
}

func TestMergeScanResponsesCombinesSidesWithoutLosingFrontFace(t *testing.T) {
	merged, err := MergeScanResponses(
		[]byte(`{"detected":true,"ocr":{"full_text":"NUMERO 12345678","lines":[{"text":"NUMERO 12345678","bbox":[],"confidence":0.8}],"confidence":0.8},"mrz":null,"has_face":true}`),
		[]byte(`{"detected":true,"ocr":{"full_text":"FECHA DE EXPEDICION 12-MAR-2015","lines":[{"text":"FECHA DE EXPEDICION 12-MAR-2015","bbox":[],"confidence":0.6}],"confidence":0.6},"mrz":null,"has_face":false}`),
	)
	if err != nil {
		t.Fatalf("MergeScanResponses() error = %v", err)
	}
	var response scanResponse
	if err := json.Unmarshal(merged, &response); err != nil {
		t.Fatalf("decode merged response: %v", err)
	}
	if !response.Detected || !response.HasFace {
		t.Fatalf("detected/has_face = %v/%v, want true/true", response.Detected, response.HasFace)
	}
	if response.OCR == nil || len(response.OCR.Lines) != 2 {
		t.Fatalf("merged OCR lines = %+v, want two lines", response.OCR)
	}
	if response.OCR.Confidence < 0.69 || response.OCR.Confidence > 0.71 {
		t.Fatalf("merged OCR confidence = %v, want weighted mean 0.7", response.OCR.Confidence)
	}
}
