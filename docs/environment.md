# Environment variables

Safe defaults belong in configuration files. Secrets belong in Doppler, a
secret manager, or injected CI/runtime environment variables.

| Variable | Purpose |
| --- | --- |
| `GOFEATHERROUTE_ADDR` | HTTP listen address |
| `GOFEATHERROUTE_API_KEY` | Incoming gateway bearer token |
| `GOFEATHERROUTE_REQUEST_TIMEOUT` | Upstream timeout |
| `GOFEATHERROUTE_MAX_BODY_BYTES` | Request size limit |
| `GOFEATHERROUTE_MAX_CONCURRENT_REQUESTS` | Concurrency limit |
| `GOFEATHERROUTE_ALLOW_INSECURE_HTTP` | Benchmark-only HTTP fake-provider opt-in; keep disabled in production |
| `OPENAI_API_KEY` | OpenAI provider secret |
| `DEEPSEEK_API_KEY` | DeepSeek provider secret |
| `OLLAMA_API_KEY` | Local Ollama placeholder key; no credential is required by Ollama |
| `OLLAMA_API_BASE` | Local Ollama OpenAI-compatible base URL override |
| `OLLAMA_CHAT_MODEL` | Upstream model for the `ollama-qwen3` alias |
| `OLLAMA_EMBEDDING_MODEL` | Upstream model for the `ollama-nomic-embed` alias |

The committed `.env.example` contains names and safe values only. It is
schema-checked against `config/env.schema.yaml` and is not a production secret
store.
