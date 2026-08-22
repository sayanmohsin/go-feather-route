# Providers and routing

The MVP supports OpenAI-compatible OpenAI and DeepSeek endpoints. A model alias
maps to one configured provider in the YAML configuration.

```yaml
routes:
  gpt-4o-mini: openai
  deepseek-chat: deepseek
```

Provider keys are referenced by environment-variable name. The gateway keeps
keys server-side and forwards only the provider authorization header.

The gateway does not retry a request after an ambiguous transport failure. A
provider may have accepted the POST before the connection failed, so replaying
it could duplicate work or charges. Bounded retries are limited to explicit
retryable upstream responses such as rate limits and selected server errors.
