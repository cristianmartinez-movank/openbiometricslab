package httpapi

import (
	"context"
	"encoding/hex"
	"net/http"
	"strings"
)

type LivenessService interface {
	LivenessPresets(context.Context) ([]byte, error)
	CreateLivenessSession(context.Context, string) ([]byte, error)
	SubmitLivenessFrame(context.Context, string, string, []byte) ([]byte, error)
	GetLivenessSession(context.Context, string) ([]byte, error)
	CancelLivenessSession(context.Context, string) ([]byte, error)
}

func livenessPresets(service LivenessService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		result, err := service.LivenessPresets(r.Context())
		writeBiometricsResult(w, "liveness presets", result, err)
	}
}

func createLivenessSession(service LivenessService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		presets, present := r.URL.Query()["preset"]
		if len(presets) > 1 {
			writeError(w, http.StatusBadRequest, "invalid_liveness_preset")
			return
		}
		preset := ""
		if present {
			preset = presets[0]
			if preset == "passive_only" {
				writeError(w, http.StatusBadRequest, "active_liveness_preset_required")
				return
			}
			if !isActiveLivenessPreset(preset) {
				writeError(w, http.StatusBadRequest, "invalid_liveness_preset")
				return
			}
		}

		result, err := service.CreateLivenessSession(r.Context(), preset)
		writeBiometricsResult(w, "liveness session creation", result, err)
	}
}

func submitLivenessFrame(service LivenessService, maxImageBytes int64) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sessionID := r.PathValue("sessionID")
		if !validSessionID(sessionID) {
			writeError(w, http.StatusBadRequest, "invalid_liveness_session_id")
			return
		}
		images, err := readMultipartImages(w, r, maxImageBytes, "image")
		if err != nil {
			writeRequestError(w, err)
			return
		}
		image := images["image"]
		result, engineErr := service.SubmitLivenessFrame(
			r.Context(),
			sessionID,
			image.contentType,
			image.data,
		)
		writeBiometricsResult(w, "liveness frame submission", result, engineErr)
	}
}

func getLivenessSession(service LivenessService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sessionID := r.PathValue("sessionID")
		if !validSessionID(sessionID) {
			writeError(w, http.StatusBadRequest, "invalid_liveness_session_id")
			return
		}
		result, err := service.GetLivenessSession(r.Context(), sessionID)
		writeBiometricsResult(w, "liveness session lookup", result, err)
	}
}

func cancelLivenessSession(service LivenessService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sessionID := r.PathValue("sessionID")
		if !validSessionID(sessionID) {
			writeError(w, http.StatusBadRequest, "invalid_liveness_session_id")
			return
		}
		result, err := service.CancelLivenessSession(r.Context(), sessionID)
		writeBiometricsResult(w, "liveness session cancellation", result, err)
	}
}

func isActiveLivenessPreset(preset string) bool {
	switch preset {
	case "eye", "smile", "multi_range", "head_turn", "full":
		return true
	default:
		return false
	}
}

func validSessionID(sessionID string) bool {
	if len(sessionID) != 36 {
		return false
	}
	for _, index := range []int{8, 13, 18, 23} {
		if sessionID[index] != '-' {
			return false
		}
	}
	_, err := hex.DecodeString(strings.ReplaceAll(sessionID, "-", ""))
	return err == nil
}
