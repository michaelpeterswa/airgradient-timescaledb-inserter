CREATE TABLE IF NOT EXISTS
    sensors.airgradient (
        time TIMESTAMPTZ NOT NULL,
        wifi INTEGER,
        serial_number TEXT NOT NULL,
        rco2 DOUBLE PRECISION,
        pm01 DOUBLE PRECISION,
        pm02 DOUBLE PRECISION,
        pm10 DOUBLE PRECISION,
        pm003_count DOUBLE PRECISION,
        atmp DOUBLE PRECISION,
        rhum DOUBLE PRECISION,
        atmp_compensated DOUBLE PRECISION,
        rhum_compensated DOUBLE PRECISION,
        tvoc_index DOUBLE PRECISION,
        tvoc_raw DOUBLE PRECISION,
        nox_index INTEGER,
        nox_raw DOUBLE PRECISION,
        boot INTEGER,
        boot_count INTEGER,
        firmware TEXT,
        model TEXT
    );

SELECT
    create_hypertable (
        'sensors.airgradient',
        'time',
        if_not_exists => TRUE
    );

CREATE TABLE IF NOT EXISTS
    sensors.airgradient_aqi (
        time TIMESTAMPTZ NOT NULL,
        serial_number TEXT NOT NULL,
        primary_pollutant TEXT,
        aqi DOUBLE PRECISION,
        level TEXT
    );

SELECT
    create_hypertable (
        'sensors.airgradient_aqi',
        'time',
        if_not_exists => TRUE
    );