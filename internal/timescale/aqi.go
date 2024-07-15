package timescale

import (
	"context"
	_ "embed"
	"fmt"
	"time"

	"676f.dev/goaqi"
	"github.com/jackc/pgx/v5"
)

//go:embed queries/get_pm02_past_day.pgsql
var getPM02PastDayQuery string

//go:embed queries/get_pm10_past_day.pgsql
var getPM10PastDayQuery string

//go:embed queries/insert_airgradient_aqi.pgsql
var insertAirgradientAQIQuery string

type PM02Row struct {
	//lint:ignore U1000 because it's needed for pgx.RowToStructByName
	Avg_pm02 float64
}

type PM10Row struct {
	//lint:ignore U1000 because it's needed for pgx.RowToStructByName
	Avg_pm10 float64
}

func (tc *TimescaleClient) GetPM02PastDay(ctx context.Context, serialNumber string) (float64, error) {
	rows, err := tc.Pool.Query(ctx, getPM02PastDayQuery, serialNumber)
	if err != nil {
		return 0, fmt.Errorf("could not query timescale: %w", err)
	}
	defer rows.Close()

	aqiRow, err := pgx.CollectExactlyOneRow[PM02Row](rows, pgx.RowToStructByName[PM02Row])
	if err != nil {
		return 0, fmt.Errorf("could not collect exactly one row: %w", err)
	}

	return aqiRow.Avg_pm02, nil
}

func (tc *TimescaleClient) GetPM10PastDay(ctx context.Context, serialNumber string) (float64, error) {
	rows, err := tc.Pool.Query(ctx, getPM10PastDayQuery, serialNumber)
	if err != nil {
		return 0, fmt.Errorf("could not query timescale: %w", err)
	}
	defer rows.Close()

	aqiRow, err := pgx.CollectExactlyOneRow[PM10Row](rows, pgx.RowToStructByName[PM10Row])
	if err != nil {
		return 0, fmt.Errorf("could not collect exactly one row: %w", err)
	}

	return aqiRow.Avg_pm10, nil
}

type AQI struct {
	SerialNumber     string
	AQI              int64
	PrimaryPollutant string
	Designation      string
}

func (tc *TimescaleClient) CalculateAQI(ctx context.Context, serialNumber string) (*AQI, error) {
	pm02, err := tc.GetPM02PastDay(ctx, serialNumber)
	if err != nil {
		return nil, fmt.Errorf("could not get PM02: %w", err)
	}

	pm10, err := tc.GetPM10PastDay(ctx, serialNumber)
	if err != nil {
		return nil, fmt.Errorf("could not get PM10: %w", err)
	}

	aqiPM02, err := goaqi.AQIPM25(pm02)
	if err != nil {
		return nil, fmt.Errorf("could not calculate AQI PM02: %w", err)
	}

	aqiPM10, err := goaqi.AQIPM100(pm10)
	if err != nil {
		return nil, fmt.Errorf("could not calculate AQI PM10: %w", err)
	}

	var primaryPollutant string
	var aqi int64
	if aqiPM02 > aqiPM10 {
		primaryPollutant = "PM2.5"
		aqi = aqiPM02
	} else {
		primaryPollutant = "PM10.0"
		aqi = aqiPM10
	}

	designation, err := goaqi.AQIDesignationFromIndex(aqi)
	if err != nil {
		return nil, fmt.Errorf("could not get AQI designation: %w", err)
	}

	return &AQI{
		SerialNumber:     serialNumber,
		AQI:              aqi,
		PrimaryPollutant: primaryPollutant,
		Designation:      designation,
	}, nil
}

func (tc *TimescaleClient) InsertAQI(ctx context.Context, aqi *AQI) error {
	_, err := tc.Pool.Exec(ctx, insertAirgradientAQIQuery,
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
