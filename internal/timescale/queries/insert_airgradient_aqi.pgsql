INSERT INTO
    sensors.airgradient_aqi (
        time,
        serial_number,
        primary_pollutant,
        aqi,
        level
    )
VALUES
    ($1, $2, $3, $4, $5)