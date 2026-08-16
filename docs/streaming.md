# Streaming

Set `stream` to `true` on a chat completion request. The gateway returns
Server-Sent Events and forwards provider chunks as they arrive.

```bash
curl --no-buffer http://127.0.0.1:4000/v1/chat/completions \
  -H 'Authorization: Bearer local-gateway-key' \
  -H 'Content-Type: application/json' \
  -d '{"model":"gpt-4o-mini","stream":true,"messages":[{"role":"user","content":"Hello"}]}'
```

The gateway does not retry after a stream has begun. Client cancellation is
propagated to the provider request.
