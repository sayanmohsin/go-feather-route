# Providers and routing

The gateway supports any OpenAI-compatible endpoint, including OpenAI,
DeepSeek, and local Ollama. Public model names and upstream mappings are YAML
configuration data; the Go implementation does not contain a provider model
catalog.

```yaml
providers:
  ollama:
    base_url: "http://127.0.0.1:11434/v1"
    api_key_env: "OLLAMA_API_KEY"

model_list:
  - model_name: ollama-qwen3
    provider: ollama
    upstream_model: qwen3:4b

route_rules:
  - match: ollama/*
    provider: ollama
```

`model_list` entries are exact aliases advertised by `/v1/models`.
`route_rules` can accept provider-qualified model names that are not listed,
such as `ollama/qwen3:8b`; the gateway forwards only `qwen3:8b` upstream.

Ollama is expected to run outside Docker during local development. From a
Docker container, use `OLLAMA_API_BASE=http://host.docker.internal:11434/v1`.
The Ollama API key is a local placeholder; Ollama ignores it. For requests
with `reasoning_effort: "none"`, the gateway sends Ollama `think: false` so
thinking traces do not enter Cloud’s final response.

Provider keys are referenced by environment-variable name. The gateway keeps
keys server-side and forwards only the provider authorization header.

Provider `kind` selects provider-specific request behavior. The default
`openai-compatible` kind forwards the shared OpenAI-compatible contract;
`ollama` additionally maps configured aliases to local model names and maps
`reasoning_effort: "none"` to Ollama's `think: false`. These behaviors belong
to the provider adapter, not to a particular model.

The gateway does not retry a request after an ambiguous transport failure. A
provider may have accepted the POST before the connection failed, so replaying
it could duplicate work or charges. Bounded retries are limited to explicit
retryable upstream responses such as rate limits and selected server errors.
