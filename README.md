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

The binary also carries its own schema migrations, so the same image applies
them as an init container before the inserter starts. See
[Commands](#commands) and [Migrations](#migrations).

## Commands

| Command | What it does |
| --- | --- |
| *(none)* or `run` | Poll the monitors and write readings on the scrape interval. |
| `migrate up` | Apply every pending embedded migration. The init container command. |
| `migrate down` | Roll back the most recent migration. One step at a time, because the down migrations drop hypertables. |
| `migrate version` | Print the schema version. Exits non-zero if a failed migration left it dirty. |
| `migrate force <n>` | Mark the schema as version `n` without running anything, to recover from a dirty state after fixing the database by hand. |
| `version` | Print the build version, commit, and date stamped in by the image build. |

With no subcommand the inserter runs, so a bare `ENTRYPOINT` behaves as it
did before there were subcommands. The `migrate` commands need only
`TIMESCALE_CONN_STRING` and `LOG_LEVEL`; the inserter's other required
settings are not read.

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

## Migrations

The schema lives in `internal/schema/migrations` as
[golang-migrate](https://github.com/golang-migrate/migrate) `NNN_title.{up,down}.sql`
pairs, the same format as the central
[lfprocks/timescale-migrations](https://github.com/lfprocks/timescale-migrations)
repository. They are embedded in the binary at build time, so the image that
runs the inserter is the image that migrates its schema, and the two can never
drift apart.

The inserter writes to two hypertables in the `sensors` schema:

- `sensors.airgradient` — one row per scrape, per monitor.
- `sensors.airgradient_aqi` — one AQI row per scrape, per monitor, with the
  primary pollutant and category name.

The applied version is recorded in `public.airgradient_schema_migrations`,
not the default `schema_migrations`. Each service that migrates a shared
database keeps its own history that way, and this one cannot collide with the
central repository's table if they ever meet in the same database.

### As a CloudNativePG init container

Run `migrate up` as an init container on the inserter's pod, pointed at the
cluster's read-write service with the application user's secret. The init
container exits `0` when the schema is current (including when there was
nothing to do), so the inserter never starts against a stale schema.

```yaml
spec:
  initContainers:
    - name: migrate
      image: ghcr.io/michaelpeterswa/airgradient-timescaledb-inserter:1.0.0
      args: ["migrate", "up"]
      env:
        - name: TIMESCALE_CONN_STRING
          valueFrom:
            secretKeyRef:
              name: timescaledb-app  # the CNPG <cluster>-app secret
              key: uri
  containers:
    - name: inserter
      image: ghcr.io/michaelpeterswa/airgradient-timescaledb-inserter:1.0.0
      env:
        - name: TIMESCALE_CONN_STRING
          valueFrom:
            secretKeyRef:
              name: timescaledb-app
              key: uri
        - name: AIRGRADIENT_INSTANCES
          value: "10.0.1.250,10.0.0.34"
```

Two things the migrations deliberately do **not** do, because the CNPG
application user cannot:

- **Create the `timescaledb` extension.** It is not a trusted extension, so
  a superuser has to create it. Do that in the `Cluster` spec, with
  `postgresql.extensions` on recent operator versions or
  `bootstrap.initdb.postInitApplicationSQL: ["CREATE EXTENSION IF NOT EXISTS timescaledb;"]`
  on older ones. `migrate up` fails at `create_hypertable` if it is missing.
- **Create roles.** The `grafana` read-only role from the central repository is
  a cluster-wide object and belongs with the cluster, not with one service.
  Declare it through CNPG's `managed.roles` and grant it there.

The `sensors` schema is created without an explicit owner, so it belongs to
whichever role runs the migration. That is what lets the application user
own everything it needs without superuser rights.

### Recovering a dirty schema

If a migration fails part way, golang-migrate marks the version dirty and
refuses to run again. Fix the database by hand, then tell it which version
the database is actually at:

```sh
airgradient-timescaledb-inserter migrate version   # shows the dirty version
airgradient-timescaledb-inserter migrate force 2   # once the database matches version 2
airgradient-timescaledb-inserter migrate up
```

## Local development

`docker-compose.yml` brings up the inserter alongside TimescaleDB, Prometheus,
Tempo, Grafana, and pgAdmin. A `migrate` service built from the same
Dockerfile runs `migrate up` against the fresh database before the inserter
starts, the same way the init container does in the cluster. Set
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

The migration round-trip test (`up`, then `down` three times, then `up`
again) needs a live TimescaleDB and is skipped otherwise:

```sh
docker run -d --name ts -e POSTGRES_PASSWORD=example -p 55432:5432 timescale/timescaledb-ha:pg16
TEST_TIMESCALE_CONN_STRING="postgres://postgres:example@localhost:55432/postgres?sslmode=disable" \
  go test -run TestMigrateRoundTrip ./internal/schema/
```

### The image

The Dockerfile cross-compiles a static binary on the build platform for each
target architecture, so a multi-arch build needs no QEMU, and stamps the
version, commit, and date into `internal/version` with `-ldflags`. The
runtime image is `distroless/static:nonroot`. The release workflow passes
those build arguments; locally:

```sh
docker build --build-arg VERSION=dev --build-arg COMMIT=$(git rev-parse --short HEAD) -t airgradient-timescaledb-inserter .
docker run --rm airgradient-timescaledb-inserter version
```

Commits follow [Conventional Commits](https://www.conventionalcommits.org/);
semantic-release cuts a GitHub release from them on every push to `main`, and
the release publishes a multi-arch image to `ghcr.io`.
