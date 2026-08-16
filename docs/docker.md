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

## Release tags

Use `latest` for evaluation and a version tag for production deployments:

```bash
docker pull sayanmohsin/go-feather-route:0.1.0
```

The release workflow also publishes the `0.1` compatibility tag. Pinning a
full version makes rollbacks predictable.

## Configuration

The container listens on port `4000`. Set the gateway key and at least one
provider key at runtime:

```bash
docker run --rm --name go-feather-route \
  --publish 4000:4000 \
  --env GOFEATHERROUTE_API_KEY=replace-with-gateway-key \
  --env OPENAI_API_KEY=replace-with-provider-key \
  sayanmohsin/go-feather-route:0.1.0
```

Use `DEEPSEEK_API_KEY` with the `deepseek-chat` model, or configure both keys
to support both providers. For Doppler or another secret manager, inject the
environment at process start rather than baking secrets into the image.

## Verify the container

The image includes a healthcheck and exposes an unauthenticated liveliness
endpoint:

```bash
curl http://127.0.0.1:4000/health/liveliness
curl http://127.0.0.1:4000/v1/models \
  -H 'Authorization: Bearer replace-with-gateway-key'
```

## Compose

For local development, copy the repository's `docker-compose.yml`, provide
secrets through your shell or secret manager, and start the service:

```bash
export GOFEATHERROUTE_API_KEY=local-gateway-key
export OPENAI_API_KEY=your-key
docker compose up --build
```

Do not commit the exported values or a populated `.env` file.

## Image metadata

The image is published as
[`sayanmohsin/go-feather-route`](https://hub.docker.com/r/sayanmohsin/go-feather-route)
for `linux/amd64` and `linux/arm64`. GitHub Actions updates the Docker Hub
short description and long README automatically after a successful release.
