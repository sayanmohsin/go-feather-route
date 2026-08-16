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
