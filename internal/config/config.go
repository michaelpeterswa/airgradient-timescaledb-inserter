package config

import (
	"fmt"
	"time"

	"github.com/caarlos0/env/v11"
)

type Config struct {
	LogLevel string `env:"LOG_LEVEL" envDefault:"error"`

	TimescaleConnString string `env:"TIMESCALE_CONN_STRING,required"`

	// AirgradientInstances is the comma-separated list of AirGradient hosts
	// (an IP or hostname, with an optional port) to poll for /measures/current.
	AirgradientInstances []string `env:"AIRGRADIENT_INSTANCES,required"`

	// ScrapeInterval is how often each instance is polled.
	ScrapeInterval time.Duration `env:"SCRAPE_INTERVAL" envDefault:"30s"`

	// ScrapeTimeout bounds one HTTP request to an instance.
	ScrapeTimeout time.Duration `env:"SCRAPE_TIMEOUT" envDefault:"10s"`

	// InsertTimeout bounds one database write.
	InsertTimeout time.Duration `env:"INSERT_TIMEOUT" envDefault:"30s"`

	MetricsEnabled bool `env:"METRICS_ENABLED" envDefault:"true"`
	MetricsPort    int  `env:"METRICS_PORT" envDefault:"8081"`

	Local bool `env:"LOCAL" envDefault:"false"`

	TracingEnabled    bool    `env:"TRACING_ENABLED" envDefault:"false"`
	TracingSampleRate float64 `env:"TRACING_SAMPLERATE" envDefault:"0.01"`
	TracingService    string  `env:"TRACING_SERVICE" envDefault:"airgradient-timescaledb-inserter"`
	TracingVersion    string  `env:"TRACING_VERSION"`
}

func NewConfig() (*Config, error) {
	var cfg Config

	err := env.Parse(&cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	return &cfg, nil
}
