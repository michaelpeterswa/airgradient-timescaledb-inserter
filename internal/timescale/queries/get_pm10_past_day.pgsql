SELECT
    avg(pm10) AS avg_pm10
FROM
    sensors.airgradient
WHERE
    serial_number = $1
    AND time > now() - interval '1 day';