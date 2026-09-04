package timescale

import (
	"context"
	_ "embed"
	"fmt"
	"time"

	"github.com/michaelpeterswa/goaqi"
)

//go:embed queries/get_pm02_past_day.pgsql
var getPM02PastDay string

//go:embed queries/get_pm10_past_day.pgsql
var getPM10PastDay string

//go:embed queries/insert_airgradient_aqi.pgsql
var insertAirgradientAQI string

// AQI is the US EPA Air Quality Index for one monitor, with the pollutant that
// set it and the category name.
type AQI struct {
	SerialNumber     string
	AQI              int64
	PrimaryPollutant string
	Designation      string
}

// GetPM02PastDay returns the trailing 24 hour mean PM2.5 for a monitor.
func (c *Client) GetPM02PastDay(ctx context.Context, serialNumber string) (float64, error) {
	var avg *float64
	if err := c.Pool.QueryRow(ctx, getPM02PastDay, serialNumber).Scan(&avg); err != nil {
		return 0, fmt.Errorf("query pm02 past day: %w", err)
	}
	if avg == nil {
		return 0, fmt.Errorf("pm02 past day: no readings for %s", serialNumber)
	}
	return *avg, nil
}

// GetPM10PastDay returns the trailing 24 hour mean PM10 for a monitor.
func (c *Client) GetPM10PastDay(ctx context.Context, serialNumber string) (float64, error) {
	var avg *float64
	if err := c.Pool.QueryRow(ctx, getPM10PastDay, serialNumber).Scan(&avg); err != nil {
		return 0, fmt.Errorf("query pm10 past day: %w", err)
	}
	if avg == nil {
		return 0, fmt.Errorf("pm10 past day: no readings for %s", serialNumber)
	}
	return *avg, nil
}

// CalculateAQI reads the trailing-day averages and computes the AQI.
func (c *Client) CalculateAQI(ctx context.Context, serialNumber string) (*AQI, error) {
	pm02, err := c.GetPM02PastDay(ctx, serialNumber)
	if err != nil {
		return nil, err
	}

	pm10, err := c.GetPM10PastDay(ctx, serialNumber)
	if err != nil {
		return nil, err
	}

	return aqiFromAverages(serialNumber, pm02, pm10)
}

// aqiFromAverages computes the AQI from the 24 hour mean concentrations. The
// index is the higher of the two pollutant sub-indices, and that pollutant is
// reported as primary. A tie goes to PM10, matching the original behaviour.
func aqiFromAverages(serialNumber string, pm02, pm10 float64) (*AQI, error) {
	aqiPM02, err := goaqi.AQIPM25(pm02)
	if err != nil {
		return nil, fmt.Errorf("calculate AQI PM02 from %v: %w", pm02, err)
	}

	aqiPM10, err := goaqi.AQIPM100(pm10)
	if err != nil {
		return nil, fmt.Errorf("calculate AQI PM10 from %v: %w", pm10, err)
	}

	primaryPollutant, aqi := "PM10.0", aqiPM10
	if aqiPM02 > aqiPM10 {
		primaryPollutant, aqi = "PM2.5", aqiPM02
	}

	designation, err := goaqi.AQIDesignationFromIndex(aqi)
	if err != nil {
		return nil, fmt.Errorf("get AQI designation for %d: %w", aqi, err)
	}

	return &AQI{
		SerialNumber:     serialNumber,
		AQI:              aqi,
		PrimaryPollutant: primaryPollutant,
		Designation:      designation,
	}, nil
}

// InsertAQI writes one AQI row, stamped with the current time.
func (c *Client) InsertAQI(ctx context.Context, aqi *AQI) error {
	_, err := c.Pool.Exec(ctx, insertAirgradientAQI,
		time.Now(),
		aqi.SerialNumber,
		aqi.PrimaryPollutant,
		aqi.AQI,
		aqi.Designation,
	)
	if err != nil {
		return fmt.Errorf("insert AQI: %w", err)
	}

	return nil
}
