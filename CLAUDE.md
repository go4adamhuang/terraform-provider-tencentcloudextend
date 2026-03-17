# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

A **supplemental** Terraform provider for Tencent Cloud — adding resources/data sources absent from the official provider, and rewriting official resource logic for internal use. Built on [terraform-plugin-framework](https://github.com/hashicorp/terraform-plugin-framework) (not the older plugin-sdk).

Uses **Speakeasy** for OpenAPI-spec-driven code generation where TencentCloud API specs are available, with hand-written resources alongside.

Provider name: `tencentcloud_extend` (source: `go4adamhuang/tencentcloud-extend`)
Module path: `github.com/go4adamhuang/terraform-provider-tencentcloud`

## Commands

```bash
# Build provider binary
make build

# Install into local Terraform plugin cache (~/.terraform.d/plugins/...)
make install

# Unit tests
make test

# Acceptance tests (requires TENCENTCLOUD_SECRET_ID / TENCENTCLOUD_SECRET_KEY / TENCENTCLOUD_REGION)
make testacc

# Single acceptance test
make testacc-one TEST=TestAccMyResource

# Lint
make lint

# Re-generate from OpenAPI specs via Speakeasy
make generate

# Tidy modules
make tidy
```

## Architecture

### Two types of resources

1. **Speakeasy-generated** (future): Driven by OpenAPI specs in `openapi/`. Run `speakeasy run` (or `make generate`) to regenerate. Output goes to `generated/`.
2. **Hand-written**: Live in `internal/provider/`. Use `terraform-plugin-framework` types/schemas directly. Register them in `internal/provider/provider.go` in the `Resources()` and `DataSources()` slices.

### Key files

| File | Purpose |
|------|---------|
| `main.go` | Entry point; wires provider address and debug flag |
| `internal/provider/provider.go` | Provider schema (credentials), `Configure()` that builds `ClientConfig`, resource/datasource registration |
| `.speakeasy/workflow.yaml` | Speakeasy sources (OpenAPI specs) and targets |
| `.speakeasy/gen.yaml` | Speakeasy generation config; `additionalResources` / `additionalDataSources` for hand-written entries |
| `openapi/` | Place TencentCloud OpenAPI YAML specs here |

### Adding a hand-written resource

1. Create `internal/provider/resource_<name>.go` implementing `resource.Resource`.
2. Register it in `provider.go` → `Resources()`.
3. Add acceptance test in `internal/provider/resource_<name>_test.go`.

### Adding a Speakeasy-generated resource

1. Add the TencentCloud OpenAPI spec to `openapi/<service>.yaml`.
2. Add the source + target to `.speakeasy/workflow.yaml`.
3. Run `make generate`.

### Credentials (ClientConfig)

`Configure()` in `provider.go` resolves credentials from provider block or env vars and passes `*ClientConfig` to every resource/data source via `req.ProviderData`. Resources cast it:

```go
func (r *MyResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
    if req.ProviderData == nil {
        return
    }
    cfg, ok := req.ProviderData.(*provider.ClientConfig)
    // use cfg.SecretID, cfg.SecretKey, cfg.Region
}
```

### TencentCloud SDK

Import individual service modules as needed:

```go
import (
    "github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common"
    "github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common/credentials"
    cvm "github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/cvm/v20170312"
)
```

Add each service to `go.mod` and run `go mod tidy`.
