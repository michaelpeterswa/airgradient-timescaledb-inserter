SELECT
    avg(pm02) AS avg_pm02
FROM
    sensors.airgradient
WHERE
    serial_number = $1
    AND time > now() - interval '1 day';