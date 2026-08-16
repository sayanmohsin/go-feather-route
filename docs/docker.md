# Docker

The published image is:

```text
sayanmohsin/go-feather-route:latest
```

It is built as a static non-root image for amd64 and arm64. Provider keys are
injected at runtime and are never part of image layers.

```bash
docker run --rm -p 4000:4000 \
  -e GOFEATHERROUTE_API_KEY=local-gateway-key \
  -e OPENAI_API_KEY=your-key \
  sayanmohsin/go-feather-route:latest
```
