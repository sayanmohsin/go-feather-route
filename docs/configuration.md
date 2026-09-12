# Configuration

Configuration is loaded in this order:

1. `-config` and `-addr` command-line flags when supplied.
2. Environment variables.
3. YAML configuration file.
4. Safe defaults.

Environment access is centralized in the typed configuration loader. Runtime
packages do not read process environment variables directly.

Use `GOFEATHERROUTE_CONFIG_FILE` to select a YAML file. Provider configuration
references secret variable names with `api_key_env`; it never stores provider
keys in YAML.

The gateway supports these command-line overrides:

```bash
go-feather-route -config /etc/go-feather-route/config.yaml -addr :4000
```

The command-line values take precedence over environment variables. Other
settings are configured through environment variables or YAML.

The optional `server.diagnostics_address` YAML field is equivalent to
`GOFEATHERROUTE_PPROF_ADDR`. It must use a loopback host such as
`127.0.0.1:6060`; leave it empty to keep runtime profiling disabled.

Validate configuration with:

```bash
make config-check
make env-example-check
```

## Model routing

Provider model names are configuration data, not Go code. Use `model_list` to
publish the model identifiers that clients send and map them to an upstream
provider model:

```yaml
model_list:
  - model_name: deepseek-chat
    provider: deepseek
    upstream_model: deepseek-chat
    fallbacks: [ollama]
    input_cost_per_million_tokens: 0.14
    output_cost_per_million_tokens: 0.28

route_rules:
  - match: deepseek/*
    provider: deepseek
```

Exact `model_list` entries are advertised by `GET /v1/models`. A provider
qualified name such as `deepseek/new-model` is accepted by a matching
`route_rules` entry and is forwarded as `new-model`; adding a new provider or
model therefore requires configuration and credentials, not a code change.
Provider API keys are injected through the variable named by `api_key_env`.
In production, keep those variables in Doppler. For local development, the
same variables may be supplied through the shell or a local env file; the
gateway does not require Open Envault.

`fallbacks` is an ordered list of provider names. A retryable upstream response
(429 or 5xx) can advance to the next available provider; ambiguous transport
failures are never replayed. Providers that repeatedly fail enter a short
cooldown. Pricing values are USD per million tokens and are used only when the
upstream does not provide cost metadata.

Optional sanitized usage delivery is configured with `usage.endpoint`,
`usage.api_key_env`, and `usage.timeout`. Usage events contain request and
routing metadata, token counts, timing, status, and cost only; prompts,
responses, and credentials are never reported.
