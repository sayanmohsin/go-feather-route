SHELL := /bin/sh

BIN := go-feather-route
TOOLS_BIN := $(CURDIR)/bin
NICE_CODE_VERSION ?= 0.3.2
NICE_CODE := npx --yes @sayanmohsin/nice-code@$(NICE_CODE_VERSION)

# Pin Go 1.27-compatible tool releases so local and CI quality checks agree.
STATICCHECK_VERSION := v0.8.1
GOIMPORTS_VERSION := v0.50.0
GOSEC_VERSION := v2.29.0
GOVULNCHECK_VERSION := v1.8.0
GOLANGCILINT_VERSION := v2.13.2

.PHONY: tools fmt fmt-check test race coverage lint security config-check env-example-check nice-code nice-code-all nice-code-skills bench benchmark-go benchmark-litellm benchmark-matrix-go benchmark-matrix-litellm benchmark-deepseek build docker profile-goroutines check

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

nice-code:
	$(NICE_CODE) --project . --changed --ci

nice-code-all:
	$(NICE_CODE) --project . --all --ci

nice-code-skills:
	$(NICE_CODE) skills list

bench:
	go test -bench=. -benchmem ./...

benchmark-go:
	./benchmarks/run.sh go

benchmark-litellm:
	./benchmarks/run.sh litellm

benchmark-matrix-go:
	./benchmarks/run-matrix.sh go

benchmark-matrix-litellm:
	./benchmarks/run-matrix.sh litellm

benchmark-deepseek:
	./scripts/benchmark-deepseek.sh

profile-goroutines:
	@echo "Start the gateway with GOFEATHERROUTE_PPROF_ADDR=127.0.0.1:6060, then run:"
	@echo "go tool pprof http://127.0.0.1:6060/debug/pprof/goroutine"
	@echo "curl 'http://127.0.0.1:6060/debug/pprof/goroutineleak?debug=1'"

build:
	CGO_ENABLED=0 go build -trimpath -ldflags='-s -w' -o $(BIN) ./cmd/go-feather-route

docker:
	docker buildx build --platform linux/amd64,linux/arm64 --tag go-feather-route:dev .

check: fmt-check test lint security config-check env-example-check
