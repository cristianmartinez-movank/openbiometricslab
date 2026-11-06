package httpapi

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
)

type healthResponse struct {
	Status string `json:"status"`
	Engine string `json:"engine,omitempty"`
}

type EngineHealthChecker interface {
	Check(context.Context) error
}

type BiometricsClient interface {
	EngineHealthChecker
	FaceDetector
	Verify(context.Context, string, []byte, string, []byte, float64) ([]byte, error)
	VerifyDocument(context.Context, string, []byte, string, []byte, float64) ([]byte, error)
	ScanDocument(context.Context, string, []byte, string) ([]byte, error)
	LivenessPresets(context.Context) ([]byte, error)
	CreateLivenessSession(context.Context, string) ([]byte, error)
	SubmitLivenessFrame(context.Context, string, string, []byte) ([]byte, error)
	GetLivenessSession(context.Context, string) ([]byte, error)
	CancelLivenessSession(context.Context, string) ([]byte, error)
}

func NewHandler(client BiometricsClient, maxImageBytes int64) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", health)
	mux.HandleFunc("GET /api/v1/biometrics/health", engineHealth(client))
	mux.HandleFunc("POST /api/v1/biometrics/detect", detect(client, maxImageBytes))
	mux.HandleFunc("POST /api/v1/biometrics/verify", verify(client, maxImageBytes))
	mux.HandleFunc("POST /api/v1/biometrics/documents/scan", scanDocument(client, maxImageBytes))
	mux.HandleFunc("POST /api/v1/biometrics/documents/verify", verifyDocument(client, maxImageBytes))
	mux.HandleFunc("GET /api/v1/biometrics/liveness/presets", livenessPresets(client))
	mux.HandleFunc("POST /api/v1/biometrics/liveness/sessions", createLivenessSession(client))
	mux.HandleFunc("POST /api/v1/biometrics/liveness/sessions/{sessionID}/frame", submitLivenessFrame(client, maxImageBytes))
	mux.HandleFunc("GET /api/v1/biometrics/liveness/sessions/{sessionID}", getLivenessSession(client))
	mux.HandleFunc("DELETE /api/v1/biometrics/liveness/sessions/{sessionID}", cancelLivenessSession(client))
	return mux
}

func health(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(healthResponse{Status: "ok"})
}

func engineHealth(engine EngineHealthChecker) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := engine.Check(r.Context()); err != nil {
			log.Printf("OpenBiometrics health check failed: %v", err)
			writeJSON(w, http.StatusServiceUnavailable, healthResponse{
				Status: "error",
				Engine: "unavailable",
			})
			return
		}

		writeJSON(w, http.StatusOK, healthResponse{
			Status: "ok",
			Engine: "available",
		})
	}
}

func writeJSON(w http.ResponseWriter, status int, body healthResponse) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}
