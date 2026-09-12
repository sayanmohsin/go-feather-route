# Providers and routing

The gateway supports OpenAI-compatible OpenAI, DeepSeek, and local Ollama
endpoints. A model alias maps to one configured provider in the YAML
configuration, and a provider can translate a stable alias to its upstream
model identifier.

```yaml
routes:
  gpt-4o-mini: openai
  deepseek-chat: deepseek
  ollama-qwen3: ollama
  ollama-nomic-embed: ollama

providers:
  ollama:
    base_url: "http://127.0.0.1:11434/v1"
    api_key_env: "OLLAMA_API_KEY"
    model_aliases:
      ollama-qwen3: "qwen3:4b"
      ollama-nomic-embed: "nomic-embed-text"
```

Ollama is expected to run outside Docker during local development. From a
Docker container, use `OLLAMA_API_BASE=http://host.docker.internal:11434/v1`.
The Ollama API key is a local placeholder; Ollama ignores it. For requests
with `reasoning_effort: "none"`, the gateway sends Ollama `think: false` so
thinking traces do not enter Cloud’s final response.

Provider keys are referenced by environment-variable name. The gateway keeps
keys server-side and forwards only the provider authorization header.

Provider `kind` selects provider-specific request behavior. The default
`openai-compatible` kind forwards the shared OpenAI-compatible contract;
`ollama` additionally translates stable aliases to local model names and maps
`reasoning_effort: "none"` to Ollama's `think: false`. These behaviors belong
to the provider adapter, not to a particular model.

The gateway does not retry a request after an ambiguous transport failure. A
provider may have accepted the POST before the connection failed, so replaying
it could duplicate work or charges. Bounded retries are limited to explicit
retryable upstream responses such as rate limits and selected server errors.
