# Deployment

Use a secret manager such as Doppler for provider and gateway keys. Set
resource limits explicitly, keep the gateway private, and expose it through an
authenticated application boundary.

For small hosts, start with a conservative concurrency limit and measure RSS
under concurrent streaming requests before increasing it.
