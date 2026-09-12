# Security

Provider keys and the gateway key are server-side secrets. Do not put them in
browser builds, YAML committed to Git, Docker build arguments, logs, or error
responses.

Use HTTPS between the gateway and remote providers, restrict network exposure,
set explicit request and concurrency limits, and rotate credentials through the
secret manager. Runtime profiles can contain stack traces and operational
metadata, so keep `GOFEATHERROUTE_PPROF_ADDR` empty in production and never
publish the loopback diagnostics port.
