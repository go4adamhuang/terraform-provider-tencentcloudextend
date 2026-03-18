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
4. Create [`tencentcloudextend_css_domain`](css_domain.md) resources with `depends_on` pointing to this resource.

-> **Note:** This resource calls the `AuthenticateDomainOwner` API to retrieve the verification content. It does **not** perform or confirm verification — that is determined by whether the DNS record is in place when Tencent Cloud checks it.

## Example Usage

```terraform
resource "tencentcloudextend_css_domain_verify" "example" {
  domain = "example.com"
}

# Set the DNS TXT record using the output value:
# cssauth.example.com → tencentcloudextend_css_domain_verify.example.verify_content
output "css_verify_content" {
  value = tencentcloudextend_css_domain_verify.example.verify_content
}

resource "tencentcloudextend_css_domain" "pull_global" {
  domain_name = "pull-global.example.com"
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
