# Security policy

Do not report vulnerabilities in public issues. Use GitHub's private security
advisory flow for this repository.

Provider keys and gateway credentials must be supplied through a secret
manager or environment injection. They must never be committed, baked into a
container image, returned by diagnostics, or written to logs.

The optional pprof listener is loopback-only and disabled by default. Treat
profiles as sensitive operational data because they may reveal stack traces,
request timing, and process behavior.
