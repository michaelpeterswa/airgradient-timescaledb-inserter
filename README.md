# airgradient-timescaledb-inserter

Polls one or more [AirGradient](https://www.airgradient.com/) monitors over
their local HTTP API and writes the readings into TimescaleDB, along with a US
EPA Air Quality Index computed from the trailing 24 hours.

```
AirGradient /measures/current ──▶ sensors.airgradient ──(avg 24h pm02, pm10)──▶ goaqi ──▶ sensors.airgradient_aqi
```

Every scrape interval, each configured instance is fetched, the reading is
inserted, then the trailing-day mean PM2.5 and PM10 are read back from the
database and turned into an AQI row. One unreachable monitor does not stop the
others from being written.

## Configuration

| Variable | Default | Description |
| --- | --- | --- |
| `TIMESCALE_CONN_STRING` | *required* | PostgreSQL/TimescaleDB connection string. |
| `AIRGRADIENT_INSTANCES` | *required* | Comma-separated monitor hosts, each an IP or hostname with an optional port. No scheme. |
| `SCRAPE_INTERVAL` | `30s` | How often every instance is polled. |
| `SCRAPE_TIMEOUT` | `10s` | Timeout for one HTTP request to a monitor. |
| `INSERT_TIMEOUT` | `30s` | Timeout for one database write. |
| `LOG_LEVEL` | `error` | `debug`, `info`, `warn`, or `error`. |
| `METRICS_ENABLED` / `METRICS_PORT` | `true` / `8081` | Prometheus metrics endpoint. |
| `LOCAL` | `false` | Export metrics over OTLP gRPC instead of serving Prometheus, for a local collector. |
| `TRACING_ENABLED` / `TRACING_SAMPLERATE` | `false` / `0.01` | OpenTelemetry tracing. |
| `TRACING_SERVICE` / `TRACING_VERSION` | `airgradient-timescaledb-inserter` / | Service resource attributes on traces. |

Tracing and the OTLP metric exporter read the standard `OTEL_EXPORTER_OTLP_*`
environment variables for their endpoint.

### Units

The firmware reports temperature in Celsius. The inserter converts `atmp` and
`atmpCompensated` to Fahrenheit before writing, to match the table.

### Faulty sensor readings

Some firmware versions emit impossible values while a sensor is faulty, such as
a negative humidity. See
[airgradienthq/arduino#190](https://github.com/airgradienthq/arduino/issues/190).
A reading with a negative `rhum` is logged and skipped rather than written. The
AQI is still recomputed for that monitor from the history already stored.

## Metrics

Prometheus metrics on `METRICS_PORT`:

| Metric | Type | Labels | Description |
| --- | --- | --- | --- |
| `airgradient_scrapes` | counter | `host` | Scrapes attempted against a monitor. |
| `airgradient_scrape_errors` | counter | `host` | Scrapes that failed to return a reading. |
| `airgradient_write_errors` | counter | `serial_number` | Failed writes to the database. |
| `airgradient_pm02` | gauge | `serial_number` | Latest PM2.5 (µg/m³). |
| `airgradient_pm10` | gauge | `serial_number` | Latest PM10 (µg/m³). |
| `airgradient_rco2` | gauge | `serial_number` | Latest CO2 (ppm). |
| `airgradient_aqi` | gauge | `serial_number` | Latest AQI from the trailing 24 hours. |

The scrape counters are labelled by host because a failed scrape never learns
the serial number. Plus the standard Go runtime and host metrics from `ootel`.

## Database

The inserter writes to two hypertables in the `sensors` schema:

- `sensors.airgradient` — one row per scrape, per monitor.
- `sensors.airgradient_aqi` — one AQI row per scrape, per monitor, with the
  primary pollutant and category name.

The migrations under `docker/timescale/migrations` create both tables and a
read-only `grafana` role for the local stack.

## Local development

`docker-compose.yml` brings up the inserter alongside TimescaleDB (with the
migrations applied), Prometheus, Tempo, Grafana, and pgAdmin. Set
`AIRGRADIENT_INSTANCES` in the compose file to your monitors, then:

```sh
docker compose up --build
```

Grafana is on port 3000, Prometheus on 9090, and pgAdmin on 8082.

```sh
make            # install commitlint and wire up the git hooks
go test ./...   # unit tests
golangci-lint run
```

Commits follow [Conventional Commits](https://www.conventionalcommits.org/);
semantic-release cuts a GitHub release from them on every push to `main`, and
the release publishes a multi-arch image to `ghcr.io`.
