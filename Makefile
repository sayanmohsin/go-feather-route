SHELL := /bin/sh

BIN := go-feather-route
TOOLS_BIN := $(CURDIR)/bin

STATICCHECK_VERSION := 2025.1.1
GOIMPORTS_VERSION := v0.36.0
GOSEC_VERSION := v2.22.8
GOVULNCHECK_VERSION := v1.1.4

.PHONY: tools fmt fmt-check test race coverage lint security config-check env-example-check bench build docker check

tools:
	mkdir -p $(TOOLS_BIN)
	GOBIN=$(TOOLS_BIN) go install honnef.co/go/tools/cmd/staticcheck@$(STATICCHECK_VERSION)
	GOBIN=$(TOOLS_BIN) go install golang.org/x/tools/cmd/goimports@$(GOIMPORTS_VERSION)
	GOBIN=$(TOOLS_BIN) go install github.com/securego/gosec/v2/cmd/gosec@$(GOSEC_VERSION)
	GOBIN=$(TOOLS_BIN) go install golang.org/x/vuln/cmd/govulncheck@$(GOVULNCHECK_VERSION)
	@command -v golangci-lint >/dev/null || echo "Install golangci-lint v2 separately: https://golangci-lint.run/docs/welcome/install/"

fmt:
	gofmt -w $$(find . -name '*.go' -not -path './vendor/*')
	$(TOOLS_BIN)/goimports -w $$(find . -name '*.go' -not -path './vendor/*')

fmt-check:
	test -z "$$(gofmt -l .)"

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

bench:
	go test -bench=. -benchmem ./...

build:
	CGO_ENABLED=0 go build -trimpath -ldflags='-s -w' -o $(BIN) ./cmd/go-feather-route

docker:
	docker buildx build --platform linux/amd64,linux/arm64 --tag go-feather-route:dev .

check: fmt-check test lint security config-check env-example-check
