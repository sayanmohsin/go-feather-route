# Health

`GET /health/liveliness` confirms that the HTTP process is running. It does not
perform a provider request, so provider outages do not make liveness fail.

Container orchestration should use this endpoint for liveness and use a
separate provider-aware check when declaring AI readiness.
