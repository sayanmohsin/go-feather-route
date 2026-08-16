FROM golang:1.26-bookworm AS build

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=${TARGETARCH} \
    go build -trimpath -ldflags='-s -w' -o /out/go-feather-route ./cmd/go-feather-route

FROM gcr.io/distroless/static-debian12:nonroot

ARG VERSION=dev
LABEL org.opencontainers.image.title="Go Feather Route" \
      org.opencontainers.image.description="A fast, featherweight OpenAI-compatible model-routing gateway written in Go" \
      org.opencontainers.image.source="https://github.com/sayanmohsin/go-feather-route" \
      org.opencontainers.image.version="$VERSION" \
      org.opencontainers.image.licenses="Apache-2.0"

COPY --from=build /out/go-feather-route /go-feather-route
COPY config/defaults.yaml /etc/go-feather-route/defaults.yaml
ENV GOFEATHERROUTE_CONFIG_FILE=/etc/go-feather-route/defaults.yaml
EXPOSE 4000
USER nonroot:nonroot
HEALTHCHECK --interval=10s --timeout=3s --start-period=2s --retries=3 \
  CMD ["/go-feather-route", "healthcheck"]
ENTRYPOINT ["/go-feather-route"]
