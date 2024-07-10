CREATE TABLE IF NOT EXISTS
    sensors.airgradient (
        time TIMESTAMPTZ NOT NULL,
        wifi INTEGER,
        serial_number TEXT NOT NULL,
        rco2 INTEGER,
        pm01 INTEGER,
        pm02 INTEGER,
        pm10 INTEGER,
        pm003_count INTEGER,
        atmp DOUBLE PRECISION,
        rhum INTEGER,
        atmp_compensated DOUBLE PRECISION,
        rhum_compensated INTEGER,
        tvoc_index INTEGER,
        tvoc_raw INTEGER,
        nox_index INTEGER,
        nox_raw INTEGER,
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