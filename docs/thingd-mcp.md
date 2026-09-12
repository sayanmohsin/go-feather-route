# Planned Thingd MCP boundary

Thingd integration is intentionally not part of the router core. A planned,
separate connector may connect Go Feather Route to an authenticated Thingd MCP
endpoint and expose an allowlisted set of Thingd tools to models.

When implemented, the connector will not access Thingd databases directly, will
be disabled by default, and will require tenant/workspace identity and explicit
mutation policies.
