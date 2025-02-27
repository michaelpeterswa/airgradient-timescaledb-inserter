package airgradient

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

type AirgradientClient struct {
	client *http.Client
}

func NewAirgradientClient(client *http.Client) *AirgradientClient {
	return &AirgradientClient{
		client: client,
	}
}

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

func (ac *AirgradientClient) GetCurrentMeasures(url string) (*MeasuresCurrentResponse, error) {
	reqUrl := fmt.Sprintf("http://%s/measures/current", url)

	resp, err := ac.client.Get(reqUrl)
	if err != nil {
		return nil, fmt.Errorf("could not get measures from %s: %w", url, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("could not read response body from %s: %w", url, err)
	}

	var measuresCurrentResponse MeasuresCurrentResponse
	err = json.Unmarshal(body, &measuresCurrentResponse)
	if err != nil {
		return nil, fmt.Errorf("could not unmarshal response body from %s: %w", url, err)
	}

	return &measuresCurrentResponse, nil
}
