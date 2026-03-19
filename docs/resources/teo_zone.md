---
page_title: "tencentcloudextend_teo_zone Resource - tencentcloudextend"
subcategory: "EdgeOne (TEO)"
description: |-
  Creates and manages a Tencent Cloud EdgeOne (TEO) zone.
---

# tencentcloudextend_teo_zone

Creates and manages a Tencent Cloud **EdgeOne (TEO)** zone.

This resource supports all parameters from the official `tencentcloud_teo_zone` resource and adds:

- **`dnsPodAccess` zone type** — DNSPod managed access, which is not supported by the official provider.
- **Safe `area` updates** — The TEO API raises `ZoneHasHostsModifyConflict` if `area` is changed at the same time as other fields when the zone has acceleration domains. This resource automatically separates the calls to avoid that error.

## Example Usage

### DNSPod managed access (primary use case)

```terraform
resource "tencentcloudextend_teo_plan" "default" {
  plan_type = "standard"
  period    = 1
}

resource "tencentcloudextend_teo_zone" "example" {
  zone_name = "example.com"
  type      = "dnsPodAccess"
  area      = "global"
  plan_id   = tencentcloudextend_teo_plan.default.plan_id
}
```

### CNAME access (partial)

```terraform
resource "tencentcloudextend_teo_zone" "cname" {
  zone_name = "example.com"
  type      = "partial"
  area      = "overseas"
  plan_id   = tencentcloudextend_teo_plan.default.plan_id
}
```

### NS access (full)

```terraform
resource "tencentcloudextend_teo_zone" "ns" {
  zone_name = "example.com"
  type      = "full"
  area      = "global"
  plan_id   = tencentcloudextend_teo_plan.default.plan_id
}

output "name_servers" {
  value = tencentcloudextend_teo_zone.ns.name_servers
}
```

### With tags and work mode

```terraform
resource "tencentcloudextend_teo_zone" "full" {
  zone_name      = "example.com"
  type           = "dnsPodAccess"
  area           = "global"
  plan_id        = tencentcloudextend_teo_plan.default.plan_id
  alias_zone_name = "my-alias"
  paused         = false

  tags = {
    env  = "prod"
    team = "platform"
  }

  work_mode_infos {
    config_group_type = "l7_acceleration"
    work_mode         = "immediate_effect"
  }
}
```

## Schema

### Required

- `zone_name` (String) Zone domain name, e.g. `example.com`. For `partial`, `full`, and `dnsPodAccess` types, pass the second-level domain. Leave empty for `noDomainAccess`. Changing this forces a new resource.
- `type` (String) Zone access type. Changing this forces a new resource. Valid values:
  - `partial` — CNAME access
  - `full` — NS access
  - `noDomainAccess` — No-domain access
  - `dnsPodAccess` — DNSPod managed access (requires the domain to already be hosted in DNSPod). **Not supported by the official provider.**
- `plan_id` (String) Target plan ID to bind this zone to. Changing this forces a new resource.

### Optional

- `area` (String) Acceleration region. Valid values: `global`, `mainland`, `overseas`. Applicable for `partial`, `full`, and `dnsPodAccess` types. Leave empty for `noDomainAccess`. Defaults to `overseas`.
- `alias_zone_name` (String) Alias site identifier. Alphanumeric, `-`, `_`, `.` characters, up to 200 chars.
- `paused` (Boolean) Whether the zone is disabled. Defaults to `false`.
- `tags` (Map of String) Tag key-value pairs. Changing tags forces a new resource (the TEO API does not provide a zone tag update endpoint; use the TencentCloud tag service to manage tags independently).
- `work_mode_infos` (Block List) Configuration group work mode settings. Each block contains:
  - `config_group_type` (String, Required) Configuration group type. Valid values: `l7_acceleration`, `edge_functions`.
  - `work_mode` (String, Required) Work mode. Valid values: `immediate_effect`, `version_control`.

### Read-Only

- `zone_id` (String) Zone ID, e.g. `zone-2noz78a8ev6e`.
- `status` (String) Zone status:
  - `active` — NS has been switched (NS zones)
  - `pending` — NS not yet switched
  - `moved` — NS moved away
  - `deactivated` — Zone is blocked
  - `initializing` — Pending plan binding
- `name_servers` (List of String) NS servers allocated by Tencent Cloud. Only populated for `full` (NS) zone type.
- `ownership_verification` (Block List) Ownership verification details. Only present when domain ownership verification is required (e.g. `partial`/CNAME zones). Contains:
  - `dns_verification` (Block List) DNS TXT record verification details:
    - `subdomain` (String) Host record to create.
    - `record_type` (String) DNS record type (e.g. `TXT`).
    - `record_value` (String) DNS record value.

## Import

Zones can be imported using the zone ID:

```shell
terraform import tencentcloudextend_teo_zone.example zone-2noz78a8ev6e
```

## Notes

- **`dnsPodAccess` vs `partial`**: Use `dnsPodAccess` when your domain is already managed by Tencent Cloud DNSPod. In this mode EdgeOne manages DNS records directly without requiring CNAME validation.
- **`area` update conflict**: The TEO API does not allow changing `area` and other fields simultaneously when the zone has acceleration domains (`ZoneHasHostsModifyConflict`). This resource handles that automatically by issuing separate API calls.
- **Tags**: Tag updates are not supported in-place due to a TEO API limitation. Changing `tags` will destroy and recreate the zone along with all its acceleration domains. To update tags without recreation, use the TencentCloud tag service directly.
