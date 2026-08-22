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

Validate configuration with:

```bash
make config-check
make env-example-check
```
