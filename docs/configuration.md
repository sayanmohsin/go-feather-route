# Configuration

Configuration is loaded in this order:

1. Command-line flags when available.
2. Environment variables.
3. YAML configuration file.
4. Safe defaults.

Environment access is centralized in the typed configuration loader. Runtime
packages do not read process environment variables directly.

Use `GOFEATHERROUTE_CONFIG_FILE` to select a YAML file. Provider configuration
references secret variable names with `api_key_env`; it never stores provider
keys in YAML.

Validate configuration with:

```bash
make config-check
make env-example-check
```
