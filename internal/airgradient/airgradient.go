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
	Rco2            int     `json:"rco2"`
	Pm01            int     `json:"pm01"`
	Pm02            int     `json:"pm02"`
	Pm10            int     `json:"pm10"`
	Pm003Count      int     `json:"pm003Count"`
	Atmp            float64 `json:"atmp"`
	Rhum            int     `json:"rhum"`
	AtmpCompensated float64 `json:"atmpCompensated"`
	RhumCompensated int     `json:"rhumCompensated"`
	TvocIndex       int     `json:"tvocIndex"`
	TvocRaw         int     `json:"tvocRaw"`
	NoxIndex        int     `json:"noxIndex"`
	NoxRaw          int     `json:"noxRaw"`
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
