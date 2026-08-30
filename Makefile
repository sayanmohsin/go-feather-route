SHELL := /bin/sh

BIN := go-feather-route
TOOLS_BIN := $(CURDIR)/bin

# Staticcheck 2025.1.1 cannot read Go 1.27 export data. Pin the first
# Go 1.27-compatible upstream revision until the next stable release.
STATICCHECK_VERSION := v0.7.0-0.dev.0.20260824195211-6cb65e58a558
GOIMPORTS_VERSION := v0.36.0
GOSEC_VERSION := v2.29.0
GOVULNCHECK_VERSION := v1.7.0
GOLANGCILINT_VERSION := v2.13.2

.PHONY: tools fmt fmt-check test race coverage lint security config-check env-example-check bench benchmark-go benchmark-litellm benchmark-deepseek build docker check

tools:
	mkdir -p $(TOOLS_BIN)
	GOBIN=$(TOOLS_BIN) go install honnef.co/go/tools/cmd/staticcheck@$(STATICCHECK_VERSION)
	GOBIN=$(TOOLS_BIN) go install golang.org/x/tools/cmd/goimports@$(GOIMPORTS_VERSION)
	GOBIN=$(TOOLS_BIN) go install github.com/securego/gosec/v2/cmd/gosec@$(GOSEC_VERSION)
	GOBIN=$(TOOLS_BIN) go install golang.org/x/vuln/cmd/govulncheck@$(GOVULNCHECK_VERSION)
	GOBIN=$(TOOLS_BIN) go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(GOLANGCILINT_VERSION)

fmt:
	gofmt -w $$(find . -name '*.go' -not -path './vendor/*')
	$(TOOLS_BIN)/goimports -w $$(find . -name '*.go' -not -path './vendor/*')

fmt-check:
	test -z "$$(gofmt -l .)"
	test -z "$$($(TOOLS_BIN)/goimports -l $$(find . -name '*.go' -not -path './vendor/*'))"

test:
	go test ./...

race:
	go test -race ./...

coverage:
	go test -coverprofile=coverage.out ./...

lint:
	go vet ./...
	$(TOOLS_BIN)/staticcheck ./...
	$(TOOLS_BIN)/golangci-lint run

security:
	$(TOOLS_BIN)/gosec ./...
	$(TOOLS_BIN)/govulncheck ./...
	go mod verify

config-check:
	go test ./internal/config -run TestLoad

env-example-check:
	go test ./internal/config -run TestEnvironmentExample
	./scripts/check-env-example.sh

bench:
	go test -bench=. -benchmem ./...

benchmark-go:
	./benchmarks/run.sh go

benchmark-litellm:
	./benchmarks/run.sh litellm

benchmark-deepseek:
	./scripts/benchmark-deepseek.sh

build:
	CGO_ENABLED=0 go build -trimpath -ldflags='-s -w' -o $(BIN) ./cmd/go-feather-route

docker:
	docker buildx build --platform linux/amd64,linux/arm64 --tag go-feather-route:dev .

check: fmt-check test lint security config-check env-example-check
