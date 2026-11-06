package httpapi

import (
	"bytes"
	"errors"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"math"
	"net/http"
	"strconv"
)

type uploadedImage struct {
	contentType string
	data        []byte
}

type requestError struct {
	status int
	code   string
}

func readMultipartImages(
	w http.ResponseWriter,
	r *http.Request,
	maxImageBytes int64,
	fieldNames ...string,
) (map[string]uploadedImage, *requestError) {
	return readMultipartImageFields(w, r, maxImageBytes, fieldNames, nil)
}

func readMultipartImageFields(
	w http.ResponseWriter,
	r *http.Request,
	maxImageBytes int64,
	requiredFields []string,
	optionalFields []string,
) (map[string]uploadedImage, *requestError) {
	fieldNames := append(append([]string(nil), requiredFields...), optionalFields...)
	maxRequestBytes := maxImageBytes*int64(len(fieldNames)) + maxMultipartOverhead
	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBytes)
	if err := r.ParseMultipartForm(maxImageBytes); err != nil {
		var maxBytesError *http.MaxBytesError
		if errors.As(err, &maxBytesError) {
			return nil, &requestError{status: http.StatusRequestEntityTooLarge, code: "image_too_large"}
		}
		return nil, &requestError{status: http.StatusBadRequest, code: "invalid_multipart_request"}
	}
	if r.MultipartForm == nil {
		return nil, &requestError{status: http.StatusBadRequest, code: "invalid_multipart_request"}
	}
	defer r.MultipartForm.RemoveAll()

	images := make(map[string]uploadedImage, len(fieldNames))
	for _, fieldName := range fieldNames {
		files := r.MultipartForm.File[fieldName]
		if len(files) == 0 && containsString(optionalFields, fieldName) {
			continue
		}
		if len(files) != 1 {
			return nil, &requestError{status: http.StatusBadRequest, code: "exactly_one_image_required"}
		}
		file, err := files[0].Open()
		if err != nil {
			return nil, &requestError{status: http.StatusBadRequest, code: "invalid_image"}
		}
		imageData, readErr := io.ReadAll(io.LimitReader(file, maxImageBytes+1))
		closeErr := file.Close()
		if readErr != nil || closeErr != nil {
			return nil, &requestError{status: http.StatusBadRequest, code: "invalid_image"}
		}
		if len(imageData) == 0 {
			return nil, &requestError{status: http.StatusBadRequest, code: "empty_image"}
		}
		if int64(len(imageData)) > maxImageBytes {
			return nil, &requestError{status: http.StatusRequestEntityTooLarge, code: "image_too_large"}
		}

		contentType := http.DetectContentType(imageData)
		if contentType != "image/jpeg" && contentType != "image/png" {
			return nil, &requestError{status: http.StatusUnsupportedMediaType, code: "unsupported_image_type"}
		}
		if err := validateImage(imageData); err != nil {
			return nil, &requestError{status: http.StatusBadRequest, code: "invalid_image"}
		}
		images[fieldName] = uploadedImage{contentType: contentType, data: imageData}
	}
	return images, nil
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func multipartField(r *http.Request, name string) (string, bool, *requestError) {
	values := r.MultipartForm.Value[name]
	if len(values) > 1 {
		return "", false, &requestError{status: http.StatusBadRequest, code: "invalid_" + name}
	}
	if len(values) == 0 {
		return "", false, nil
	}
	return values[0], true, nil
}

func parseThreshold(r *http.Request) (float64, *requestError) {
	value, present, err := multipartField(r, "threshold")
	if err != nil {
		return 0, err
	}
	if !present {
		return 0.4, nil
	}
	threshold, parseErr := strconv.ParseFloat(value, 64)
	if parseErr != nil || math.IsNaN(threshold) || math.IsInf(threshold, 0) || threshold < 0 || threshold > 1 {
		return 0, &requestError{status: http.StatusBadRequest, code: "invalid_threshold"}
	}
	return threshold, nil
}

func validateImage(data []byte) error {
	config, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return err
	}
	if config.Width <= 0 || config.Height <= 0 || config.Width > maxImagePixels/config.Height {
		return errors.New("image dimensions exceed limit")
	}
	if _, _, err := image.Decode(bytes.NewReader(data)); err != nil {
		return err
	}
	return nil
}

func writeRequestError(w http.ResponseWriter, err *requestError) {
	writeError(w, err.status, err.code)
}
