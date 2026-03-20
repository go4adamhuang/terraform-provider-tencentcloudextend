# Terraform Provider: tencentcloudextend

A **supplemental** Terraform provider for Tencent Cloud — filling gaps in the [official provider](https://registry.terraform.io/providers/tencentcloudstack/tencentcloud/latest) by adding missing resources and rewriting certain resource logic for internal use.

Built on [`terraform-plugin-framework`](https://github.com/hashicorp/terraform-plugin-framework).

## Why This Exists

The official `tencentcloudstack/tencentcloud` provider has incomplete coverage for some services. This provider supplements it:

| Gap | This provider's solution |
|-----|--------------------------|
| EdgeOne (TEO) plan creation not supported | `tencentcloudextend_teo_plan` |
| TEO zone `area` update causes API conflict error | `tencentcloudextend_teo_zone` (handles sequencing) |
| CSS domain ownership verification missing | `tencentcloudextend_css_domain_verify` |

Use both providers together in the same Terraform configuration.

---

## Requirements

- [Terraform](https://www.terraform.io/downloads.html) >= 1.0
- [Go](https://golang.org/doc/install) >= 1.22 (for local development)

---

## Installation

Add to your Terraform configuration:

```hcl
terraform {
  required_providers {
    tencentcloud = {
      source = "tencentcloudstack/tencentcloud"
    }
    tencentcloudextend = {
      source  = "go4adamhuang/tencentcloudextend"
      version = "~> 1.0"
    }
  }
}
```

---

## Provider Configuration

Credentials are resolved in this order:

1. Explicit provider block arguments
2. [tccli](https://github.com/TencentCloud/tencentcloud-cli) profile file (`~/.tccli/<profile>.credential`)
3. Environment variables: `TENCENTCLOUD_SECRET_ID`, `TENCENTCLOUD_SECRET_KEY`, `TENCENTCLOUD_REGION`

### Single account

```hcl
provider "tencentcloudextend" {
  profile = "default"
}
```

### Explicit credentials

```hcl
provider "tencentcloudextend" {
  secret_id  = var.secret_id
  secret_key = var.secret_key
  region     = "ap-guangzhou"
}
```

### Dual-account (CSS and DNSPod on different accounts)

```hcl
provider "tencentcloudextend" {
  profile     = "stream"   # CSS account
  dns_profile = "default"  # DNSPod account
}
```

### Schema

| Argument | Type | Description |
|----------|------|-------------|
| `secret_id` | string | Tencent Cloud secret ID (sensitive) |
| `secret_key` | string | Tencent Cloud secret key (sensitive) |
| `region` | string | Default region (e.g. `ap-guangzhou`) |
| `profile` | string | tccli profile name for main account |

---

## Resources

### `tencentcloudextend_teo_plan`

Creates a Tencent Cloud EdgeOne (TEO) billing plan. The official provider does not support plan creation.

```hcl
# Enterprise plan (postpaid)
resource "tencentcloudextend_teo_plan" "enterprise" {
  plan_type = "enterprise"
}

# Standard plan (prepaid, 12 months, auto-renewal)
resource "tencentcloudextend_teo_plan" "standard" {
  plan_type  = "standard"
  period     = 12
  renew_flag = "on"
}

# Reference plan_id when creating zones via the official provider
resource "tencentcloud_teo_zone" "example" {
  zone_name = "example.com"
  plan_id   = tencentcloudextend_teo_plan.enterprise.plan_id
}
```

Full documentation: [docs/resources/teo_plan.md](docs/resources/teo_plan.md)

---

### `tencentcloudextend_teo_zone`

Manages a Tencent Cloud EdgeOne zone. Fixes the `ZoneHasHostsModifyConflict` API error when updating `area` by sequencing the API calls correctly.

```hcl
resource "tencentcloudextend_teo_zone" "example" {
  zone_name  = "example.com"
  plan_id    = tencentcloudextend_teo_plan.enterprise.plan_id
  type       = "full"
  area       = "overseas"
}
```

Full documentation: [docs/resources/teo_zone.md](docs/resources/teo_zone.md)

---

### `tencentcloudextend_css_domain_verify`

Retrieves the DNS TXT record value needed for CSS (Cloud Streaming Services) domain ownership verification. Verifying the main domain unlocks all its subdomains for CSS without re-verification.

```hcl
resource "tencentcloudextend_css_domain_verify" "example" {
  domain = "example.com"
}

# outputs:
#   main_domain    = "example.com"
#   verify_content = "<TXT record value>"

# Add a TXT record: cssauth.<main_domain> → verify_content
# Then manage CSS domains via the official provider:
resource "tencentcloud_css_domain" "pull" {
  domain_name = "pull.example.com"
  domain_type = 1
  play_type   = 2
  depends_on  = [tencentcloudextend_css_domain_verify.example]
}
```

Full documentation: [docs/resources/css_domain_verify.md](docs/resources/css_domain_verify.md)

---

## Development

### Build & Install

```bash
# Build provider binary
make build

# Install into local Terraform plugin cache
make install
```

### Testing

```bash
# Unit tests
make test

# Acceptance tests (requires real credentials)
export TENCENTCLOUD_SECRET_ID="..."
export TENCENTCLOUD_SECRET_KEY="..."
export TENCENTCLOUD_REGION="ap-guangzhou"
make testacc

# Single acceptance test
make testacc-one TEST=TestAccTeoPlanResource
```

### Other Commands

```bash
make lint      # Run golangci-lint
make fmt       # Format Go code
make tidy      # go mod tidy
make docs      # Regenerate docs from schema
make generate  # Regenerate from OpenAPI specs via Speakeasy
make clean     # Remove compiled binary
```

### Adding a New Resource

1. Create `internal/provider/resource_<name>.go` implementing `resource.Resource`.
2. Register it in `internal/provider/provider.go` → `Resources()`.
3. Add an acceptance test in `internal/provider/resource_<name>_test.go`.
4. Run `make docs` to regenerate documentation.

### Project Structure

```
internal/provider/
├── provider.go                    # Credentials, Configure(), resource registration
├── helpers.go                     # Shared pointer helpers
├── resource_teo_zone.go
├── resource_teo_plan.go
└── resource_css_domain_verify.go
examples/                          # Usage examples
docs/                              # Generated Terraform documentation
openapi/                           # OpenAPI specs for Speakeasy code generation
```

---

## License

[MIT](LICENSE)
