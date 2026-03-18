---
page_title: "Provider: TencentCloud Extend"
description: |-
  The TencentCloud Extend provider supplements the official TencentCloud provider with additional resources and data sources.
---

# TencentCloud Extend Provider

The **TencentCloud Extend** provider is a supplemental provider for [Tencent Cloud](https://cloud.tencent.com), adding resources absent from the [official provider](https://registry.terraform.io/providers/tencentcloudstack/tencentcloud/latest) and rewriting certain resource logic for internal use cases.

Built on [terraform-plugin-framework](https://github.com/hashicorp/terraform-plugin-framework).

## Authentication

Credentials can be supplied in three ways (in order of precedence):

1. **Explicit provider block** — `secret_id`, `secret_key`, and `region` attributes.
2. **tccli profile** — `profile` attribute reads from `~/.tccli/<profile>.credential`.
3. **Environment variables** — `TENCENTCLOUD_SECRET_ID`, `TENCENTCLOUD_SECRET_KEY`, `TENCENTCLOUD_REGION`.

Using a `profile` is recommended so credentials are never committed to code.

## Example Usage

```terraform
terraform {
  required_providers {
    tencentcloudextend = {
      source  = "go4adamhuang/tencentcloudextend"
      version = "~> 1.0"
    }
  }
}

# Single account: CSS and DNSPod both use the same profile
provider "tencentcloudextend" {
  profile = "stream"
}
```

```terraform
# Dual account: CSS on "stream", DNSPod on "default"
provider "tencentcloudextend" {
  profile     = "stream"
  dns_profile = "default"
}
```

```terraform
# Explicit credentials
provider "tencentcloudextend" {
  secret_id  = var.secret_id
  secret_key = var.secret_key
  region     = "ap-guangzhou"
}
```

## Schema

### Optional

- `secret_id` (String) Tencent Cloud Secret ID. Can also be set via the `TENCENTCLOUD_SECRET_ID` environment variable.
- `secret_key` (String, Sensitive) Tencent Cloud Secret Key. Can also be set via the `TENCENTCLOUD_SECRET_KEY` environment variable.
- `region` (String) Tencent Cloud region (e.g. `ap-guangzhou`). Can also be set via the `TENCENTCLOUD_REGION` environment variable.
- `profile` (String) tccli profile name for credentials. Reads from `~/.tccli/<profile>.credential`. Can also be set via the `TENCENTCLOUD_PROFILE` environment variable.
- `dns_profile` (String) tccli profile name for a separate DNSPod account. Defaults to the same account as `profile`.
