FROM golang:1.26-bookworm AS build

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY benchmarks/fake-provider ./benchmarks/fake-provider
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags='-s -w' -o /out/fake-provider ./benchmarks/fake-provider

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/fake-provider /fake-provider
USER nonroot:nonroot
EXPOSE 8080
ENTRYPOINT ["/fake-provider"]
