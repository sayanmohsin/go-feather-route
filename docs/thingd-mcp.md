# Optional Thingd MCP connector

Thingd integration is intentionally not part of the router core. A later
connector may connect Go Feather Route to an authenticated Thingd MCP endpoint
and expose an allowlisted set of Thingd tools to models.

The connector will not access Thingd databases directly, will be disabled by
default, and will require tenant/workspace identity and explicit mutation
policies.
