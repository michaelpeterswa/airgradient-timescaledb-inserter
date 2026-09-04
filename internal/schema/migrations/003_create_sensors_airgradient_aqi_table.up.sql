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
