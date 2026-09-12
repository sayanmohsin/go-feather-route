# Troubleshooting

If the service exits during startup, run it with the same configuration file
and environment used by the deployment and inspect the redacted configuration
error.

If `/health/liveliness` works but a chat request fails, check the model route,
provider key injection, provider base URL, gateway bearer token, and upstream
timeout. Do not paste provider keys or authorization headers into issue
reports.

If a response stalls, first inspect `/status` and `/metrics` for active streams,
aborts, cancellations, and upstream timing. For deeper local investigation,
enable the loopback-only diagnostics listener and inspect the `goroutine` and
`goroutineleak` profiles described in [testing](testing.md). Disable it again
after the investigation.
