package main

import (
	"log"
	"net/http"

	"openbiometrics-lab/backend/internal/biometrics"
	"openbiometrics-lab/backend/internal/config"
	"openbiometrics-lab/backend/internal/httpapi"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("load configuration: %v", err)
	}

	engineClient, err := biometrics.NewClient(cfg.OpenBiometricsBaseURL, &http.Client{
		Timeout: cfg.BiometricsTimeout,
	})
	if err != nil {
		log.Fatalf("configure OpenBiometrics client: %v", err)
	}

	server := &http.Server{
		Addr:    cfg.Addr,
		Handler: httpapi.NewHandler(engineClient, cfg.MaxImageBytes),
	}

	log.Printf("Go API listening on %s", server.Addr)
	log.Fatal(server.ListenAndServe())
}
