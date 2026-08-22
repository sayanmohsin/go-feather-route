# Authentication

Go Feather Route uses bearer authentication at the gateway boundary. The
runtime refuses to start unless `GOFEATHERROUTE_API_KEY` is set.

Send the key with every protected request:

```bash
curl http://localhost:4000/v1/models \
  -H 'Authorization: Bearer replace-me'
```

Health liveliness is intentionally unauthenticated so load balancers and
container orchestrators can verify that the process is alive. Model discovery
and chat completions require the gateway key.

Provider keys are separate credentials. `OPENAI_API_KEY` and
`DEEPSEEK_API_KEY` are sent only to their selected upstream provider and are
never returned by the API or written to structured logs.

For production, inject all secrets through Doppler, a cloud secret manager, or
the deployment platform. Keep `.env.example` empty of real credentials.
