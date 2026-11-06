package biometrics

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

const maxHealthResponseBytes = 64 * 1024

type healthResponse struct {
	Status       string `json:"status"`
	ModelsLoaded bool   `json:"models_loaded"`
}

type Client struct {
	baseURL    string
	httpClient *http.Client
}

func NewClient(baseURL string, httpClient *http.Client) (*Client, error) {
	parsedURL, err := url.ParseRequestURI(baseURL)
	if err != nil {
		return nil, fmt.Errorf("parse OpenBiometrics base URL: %w", err)
	}
	if (parsedURL.Scheme != "http" && parsedURL.Scheme != "https") || parsedURL.Host == "" {
		return nil, fmt.Errorf("OpenBiometrics base URL must use http or https and include a host")
	}
	if parsedURL.User != nil || parsedURL.RawQuery != "" || parsedURL.Fragment != "" {
		return nil, fmt.Errorf("OpenBiometrics base URL must not include credentials, query, or fragment")
	}
	if httpClient == nil {
		return nil, fmt.Errorf("HTTP client is required")
	}

	return &Client{
		baseURL:    strings.TrimRight(baseURL, "/"),
		httpClient: httpClient,
	}, nil
}

func (c *Client) Check(ctx context.Context) error {
	request, err := http.NewRequestWithContext(
		ctx,
		http.MethodGet,
		c.baseURL+"/api/v1/health",
		nil,
	)
	if err != nil {
		return fmt.Errorf("create OpenBiometrics health request: %w", err)
	}

	response, err := c.httpClient.Do(request)
	if err != nil {
		return fmt.Errorf("request OpenBiometrics health: %w", err)
	}
	defer response.Body.Close()

	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("OpenBiometrics health returned HTTP %d", response.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(response.Body, maxHealthResponseBytes+1))
	if err != nil {
		return fmt.Errorf("read OpenBiometrics health response: %w", err)
	}
	if len(body) > maxHealthResponseBytes {
		return fmt.Errorf("OpenBiometrics health response exceeds %d bytes", maxHealthResponseBytes)
	}

	var health healthResponse
	if err := json.Unmarshal(body, &health); err != nil {
		return fmt.Errorf("decode OpenBiometrics health response: %w", err)
	}
	if health.Status != "ok" || !health.ModelsLoaded {
		return fmt.Errorf("OpenBiometrics reports an unhealthy face detector")
	}

	return nil
}
