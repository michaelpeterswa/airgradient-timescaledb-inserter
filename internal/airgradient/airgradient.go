// Package airgradient polls the local HTTP API of an AirGradient monitor.
package airgradient

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// Client fetches readings from AirGradient monitors over their local API.
type Client struct {
	client *http.Client
}

// NewClient wraps an HTTP client. Set the client's Timeout to bound a scrape.
func NewClient(client *http.Client) *Client {
	return &Client{
		client: client,
	}
}

// MeasuresCurrentResponse is the body of GET /measures/current on the monitor.
// The temperature fields are Celsius, as the firmware reports them.
type MeasuresCurrentResponse struct {
	Wifi            int     `json:"wifi"`
	Serialno        string  `json:"serialno"`
	Rco2            float64 `json:"rco2"`
	Pm01            float64 `json:"pm01"`
	Pm02            float64 `json:"pm02"`
	Pm10            float64 `json:"pm10"`
	Pm003Count      float64 `json:"pm003Count"`
	Atmp            float64 `json:"atmp"`
	Rhum            float64 `json:"rhum"`
	AtmpCompensated float64 `json:"atmpCompensated"`
	RhumCompensated float64 `json:"rhumCompensated"`
	TvocIndex       float64 `json:"tvocIndex"`
	TvocRaw         float64 `json:"tvocRaw"`
	NoxIndex        float64 `json:"noxIndex"`
	NoxRaw          float64 `json:"noxRaw"`
	Boot            int     `json:"boot"`
	BootCount       int     `json:"bootCount"`
	Firmware        string  `json:"firmware"`
	Model           string  `json:"model"`
}

// GetCurrentMeasures fetches the current readings from one monitor. The host is
// an IP or hostname with an optional port, without a scheme.
func (c *Client) GetCurrentMeasures(ctx context.Context, host string) (*MeasuresCurrentResponse, error) {
	reqURL := fmt.Sprintf("http://%s/measures/current", host)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("build request for %s: %w", host, err)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("get measures from %s: %w", host, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		// Drain a little of the body so the connection can be reused.
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
		return nil, fmt.Errorf("get measures from %s: unexpected status %s", host, resp.Status)
	}

	var measures MeasuresCurrentResponse
	if err := json.NewDecoder(resp.Body).Decode(&measures); err != nil {
		return nil, fmt.Errorf("decode response body from %s: %w", host, err)
	}

	return &measures, nil
}
