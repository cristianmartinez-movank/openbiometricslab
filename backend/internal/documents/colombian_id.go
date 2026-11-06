package documents

import (
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"
)

type ColombianIDExtraction struct {
	DocumentType         string   `json:"document_type"`
	Presentation         string   `json:"presentation"`
	DocumentNumber       *string  `json:"document_number"`
	NumberFormatValid    *bool    `json:"number_format_valid"`
	Surnames             *string  `json:"surnames"`
	GivenNames           *string  `json:"given_names"`
	BirthDate            *string  `json:"birth_date"`
	BirthDateValid       *bool    `json:"birth_date_valid"`
	IssueDate            *string  `json:"issue_date"`
	IssueDateValid       *bool    `json:"issue_date_valid"`
	AuthenticityVerified bool     `json:"authenticity_verified"`
	RegistryVerified     bool     `json:"registry_verified"`
	Warnings             []string `json:"warnings"`
}

var (
	numberPattern = regexp.MustCompile(`[0-9](?:[0-9 .-]{0,20}[0-9])?`)
	datePattern   = regexp.MustCompile(`(?i)\b([0-9]{1,2})[./ -]([0-9]{1,2}|[A-Z]{3})[./ -]([0-9]{4})\b`)
)

var spanishMonths = map[string]string{
	"ENE": "01", "FEB": "02", "MAR": "03", "ABR": "04",
	"MAY": "05", "JUN": "06", "JUL": "07", "AGO": "08",
	"SEP": "09", "OCT": "10", "NOV": "11", "DIC": "12",
}

func ExtractColombianID(ocrText, presentation string) ColombianIDExtraction {
	presentation = strings.ToLower(strings.TrimSpace(presentation))
	if presentation != "physical" && presentation != "digital" {
		presentation = "unspecified"
	}

	result := ColombianIDExtraction{
		DocumentType:         "CC",
		Presentation:         presentation,
		AuthenticityVerified: false,
		RegistryVerified:     false,
		Warnings:             make([]string, 0),
	}
	lines := normalizedLines(ocrText)
	result.DocumentNumber = extractNumber(lines)
	result.NumberFormatValid = validateNumber(result.DocumentNumber)
	result.Surnames = extractLabelValue(lines, "APELLIDOS", "NOMBRES", "FECHA")
	result.GivenNames = extractLabelValue(lines, "NOMBRES", "APELLIDOS", "FECHA")

	result.BirthDate, result.BirthDateValid = extractDate(lines,
		[]string{"FECHA DE NACIMIENTO", "FECHA NACIMIENTO"})
	result.IssueDate, result.IssueDateValid = extractDate(lines,
		[]string{"FECHA Y LUGAR DE EXPEDICION", "FECHA DE EXPEDICION", "FECHA EXPEDICION"})
	if result.IssueDate != nil && result.BirthDate != nil &&
		result.IssueDateValid != nil && *result.IssueDateValid &&
		result.BirthDateValid != nil && *result.BirthDateValid {
		birthDate, _ := time.Parse("2006-01-02", *result.BirthDate)
		issueDate, _ := time.Parse("2006-01-02", *result.IssueDate)
		if issueDate.Before(birthDate) {
			invalid := false
			result.IssueDateValid = &invalid
		}
	}

	if result.DocumentNumber == nil {
		result.Warnings = append(result.Warnings, "No se pudo extraer el número de cédula.")
	} else if !*result.NumberFormatValid {
		result.Warnings = append(result.Warnings, "El número extraído no tiene el formato local esperado de 6 a 10 dígitos.")
	}
	if result.Surnames == nil {
		result.Warnings = append(result.Warnings, "No se pudieron extraer los apellidos.")
	}
	if result.GivenNames == nil {
		result.Warnings = append(result.Warnings, "No se pudieron extraer los nombres.")
	}
	if result.BirthDate == nil {
		result.Warnings = append(result.Warnings, "No se pudo extraer la fecha de nacimiento.")
	} else if !*result.BirthDateValid {
		result.Warnings = append(result.Warnings, "La fecha de nacimiento es inválida o posterior a hoy.")
	}
	if result.IssueDate != nil && !*result.IssueDateValid {
		result.Warnings = append(result.Warnings, "La fecha de expedición es inválida o no puede ser anterior al nacimiento.")
	}
	if strings.TrimSpace(ocrText) == "" {
		result.Warnings = append([]string{"No hay texto OCR disponible para validar."}, result.Warnings...)
	}
	return result
}

func normalizedLines(text string) []string {
	replacer := strings.NewReplacer(
		"Á", "A", "É", "E", "Í", "I", "Ó", "O", "Ú", "U", "Ü", "U",
	)
	text = replacer.Replace(strings.ToUpper(text))
	lines := strings.Split(text, "\n")
	for index := range lines {
		lines[index] = strings.TrimSpace(lines[index])
	}
	return lines
}

func extractNumber(lines []string) *string {
	for index, line := range lines {
		labelIndex := strings.Index(line, "NUMERO")
		if labelIndex < 0 {
			labelIndex = strings.Index(line, "NUIP")
		}
		if labelIndex < 0 {
			continue
		}
		value := line[labelIndex:]
		if strings.HasPrefix(value, "NUMERO") {
			value = strings.TrimPrefix(value, "NUMERO")
		} else {
			value = strings.TrimPrefix(value, "NUIP")
		}
		if number := firstNumber(value); number != nil {
			return number
		}
		if index+1 < len(lines) {
			if number := firstNumber(lines[index+1]); number != nil {
				return number
			}
		}
	}
	return nil
}

func firstNumber(text string) *string {
	match := numberPattern.FindString(text)
	if match == "" {
		return nil
	}
	digits := strings.Map(func(r rune) rune {
		if r >= '0' && r <= '9' {
			return r
		}
		return -1
	}, match)
	if digits == "" {
		return nil
	}
	return &digits
}

func validateNumber(number *string) *bool {
	if number == nil {
		return nil
	}
	valid := len(*number) >= 6 && len(*number) <= 10
	return &valid
}

func extractLabelValue(lines []string, label string, stopLabels ...string) *string {
	for index, line := range lines {
		labelIndex := strings.Index(line, label)
		if labelIndex < 0 {
			continue
		}
		value := strings.TrimSpace(strings.TrimLeft(line[labelIndex+len(label):], " :-"))
		if value == "" {
			if index+1 >= len(lines) {
				continue
			}
			value = strings.TrimSpace(lines[index+1])
		}
		for _, stop := range stopLabels {
			if stopIndex := strings.Index(value, stop); stopIndex >= 0 {
				value = strings.TrimSpace(value[:stopIndex])
			}
		}
		value = cleanName(value)
		if value != "" {
			return &value
		}
	}
	return nil
}

func cleanName(value string) string {
	var cleaned strings.Builder
	lastSpace := false
	for _, character := range value {
		switch {
		case unicode.IsLetter(character):
			cleaned.WriteRune(character)
			lastSpace = false
		case unicode.IsSpace(character), character == '-':
			if !lastSpace && cleaned.Len() > 0 {
				cleaned.WriteRune(character)
				lastSpace = true
			}
		default:
			if cleaned.Len() > 0 {
				break
			}
		}
	}
	return strings.TrimSpace(cleaned.String())
}

func extractDate(lines, labels []string) (*string, *bool) {
	for index, line := range lines {
		for _, label := range labels {
			labelIndex := strings.Index(line, label)
			if labelIndex < 0 {
				continue
			}
			if date, valid := parseDate(line[labelIndex+len(label):]); date != nil {
				return date, valid
			}
			if index+1 < len(lines) {
				if date, valid := parseDate(lines[index+1]); date != nil {
					return date, valid
				}
			}
		}
	}
	return nil, nil
}

func parseDate(text string) (*string, *bool) {
	match := datePattern.FindStringSubmatch(text)
	if len(match) != 4 {
		return nil, nil
	}
	month := match[2]
	if len(month) == 3 {
		var ok bool
		month, ok = spanishMonths[month]
		if !ok {
			return invalidDate(match[0])
		}
	}
	monthNumber, err := strconv.Atoi(month)
	if err != nil || monthNumber < 1 || monthNumber > 12 {
		return invalidDate(match[0])
	}
	date, err := time.Parse("02-01-2006", match[1]+"-"+month+"-"+match[3])
	if err != nil {
		return invalidDate(match[0])
	}
	value := date.Format("2006-01-02")
	valid := !date.After(time.Now())
	return &value, &valid
}

func invalidDate(value string) (*string, *bool) {
	valid := false
	return &value, &valid
}
