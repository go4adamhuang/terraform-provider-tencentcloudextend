BINARY      = terraform-provider-tencentcloudextend
NAMESPACE   = go4adamhuang
PROVIDER    = tencentcloudextend
VERSION     = $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
OS_ARCH     = $(shell go env GOOS)_$(shell go env GOARCH)

INSTALL_DIR = ~/.terraform.d/plugins/registry.terraform.io/$(NAMESPACE)/$(PROVIDER)/$(VERSION)/$(OS_ARCH)

.PHONY: build install test testacc lint generate fmt clean

## Build the provider binary
build:
	go build -ldflags="-X main.version=$(VERSION)" -o $(BINARY) .

## Install provider into local Terraform plugin cache for manual testing
install: build
	mkdir -p $(INSTALL_DIR)
	cp $(BINARY) $(INSTALL_DIR)/

## Run unit tests
test:
	go test ./... -v -count=1

## Run acceptance tests (requires real credentials; sets TF_ACC=1)
testacc:
	TF_ACC=1 go test ./... -v -count=1 -timeout 120m

## Run a single acceptance test by name, e.g.: make testacc-one TEST=TestAccMyResource
testacc-one:
	TF_ACC=1 go test ./internal/provider/ -v -run $(TEST) -timeout 120m

## Lint using golangci-lint
lint:
	golangci-lint run ./...

## Re-generate provider from OpenAPI specs via Speakeasy CLI
generate:
	speakeasy run

## Format Go code
fmt:
	go fmt ./...
	goimports -w .

## Tidy Go modules
tidy:
	go mod tidy

## Generate provider documentation into docs/
docs:
	tfplugindocs generate --provider-name $(PROVIDER)

clean:
	rm -f $(BINARY)
