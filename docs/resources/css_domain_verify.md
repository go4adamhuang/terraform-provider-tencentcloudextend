---
page_title: "tencentcloudextend_css_domain_verify Resource - tencentcloudextend"
subcategory: "Cloud Streaming Service (CSS)"
description: |-
  Retrieves the DNS TXT record value needed to verify CSS domain ownership via dnsCheck.
---

# tencentcloudextend_css_domain_verify

Retrieves the DNS TXT record value required to verify **CSS domain ownership** via `dnsCheck`.

Tencent Cloud CSS requires you to prove ownership of a domain before adding any subdomains under it. Once the root domain is verified, all subdomains can be added to CSS without re-verification.

## Verification workflow

1. Create this resource with your root domain (or any subdomain under it).
2. Read the computed `verify_content` value.
3. Create a DNS TXT record: `cssauth.<main_domain>` → `<verify_content>`.
4. Create `tencentcloud_css_domain` resources (from the [official provider](https://registry.terraform.io/providers/tencentcloudstack/tencentcloud/latest/docs/resources/css_domain)) with `depends_on` pointing to this resource.

-> **Note:** This resource calls the `AuthenticateDomainOwner` API to retrieve the verification content. It does **not** perform or confirm verification — that is determined by whether the DNS record is in place when Tencent Cloud checks it.

-> **Cross-account use case:** This resource is designed for scenarios where domain ownership verification is managed by a different Tencent Cloud account than the one managing CSS domains. Configure this provider with the account that owns the domain, and use the official `tencentcloud` provider for CSS domain management.

## Example Usage

```terraform
terraform {
  required_providers {
    tencentcloudextend = {
      source  = "go4adamhuang/tencentcloudextend"
      version = "~> 1.0"
    }
    tencentcloud = {
      source  = "tencentcloudstack/tencentcloud"
      version = "~> 1.81"
    }
  }
}

# This provider uses the account that owns the domain (for verification)
provider "tencentcloudextend" {
  profile = "dns-account"
}

# This provider uses the account that manages CSS
provider "tencentcloud" {
  profile = "css-account"
}

# Step 1: retrieve the DNS TXT verification value
resource "tencentcloudextend_css_domain_verify" "example" {
  domain = "example.com"
}

# Step 2: set cssauth.<main_domain> TXT record with verify_content
# (using your DNS provider's Terraform resource)

# Step 3: add CSS domains using the official provider, after verification
resource "tencentcloud_css_domain" "pull_global" {
  domain_name = "pull.example.com"
  domain_type = 1
  play_type   = 2

  depends_on = [tencentcloudextend_css_domain_verify.example]
}
```

## Schema

### Required

- `domain` (String) Any CSS domain under the root domain to verify (e.g. `pull-global.example.com` or `example.com`). The API will derive the root domain automatically. Changing this value forces a new resource.

### Read-Only

- `main_domain` (String) The root domain that requires verification (e.g. `example.com`).
- `verify_content` (String) The value to set on the `cssauth.<main_domain>` DNS TXT record for dnsCheck verification.
