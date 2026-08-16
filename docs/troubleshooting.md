# Troubleshooting

If the service exits during startup, run it with the same configuration file
and environment used by the deployment and inspect the redacted configuration
error.

If `/health/liveliness` works but a chat request fails, check the model route,
provider key injection, provider base URL, gateway bearer token, and upstream
timeout. Do not paste provider keys or authorization headers into issue
reports.
