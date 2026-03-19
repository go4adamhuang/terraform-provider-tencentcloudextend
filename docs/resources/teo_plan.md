---
page_title: "tencentcloudextend_teo_plan Resource - tencentcloudextend"
subcategory: "EdgeOne (TEO)"
description: |-
  Creates a Tencent Cloud EdgeOne (TEO) billing plan.
---

# tencentcloudextend_teo_plan

Creates a Tencent Cloud **EdgeOne (TEO)** billing plan.

The official `tencentcloudstack/tencentcloud` provider does not provide a resource for creating TEO plans. After creating a plan with this resource, use `tencentcloud_teo_zone` from the official provider to create and bind zones to it.

## Example Usage

### Enterprise plan (postpaid)

```terraform
resource "tencentcloudextend_teo_plan" "enterprise" {
  plan_type = "enterprise"
}
```

### Standard plan (prepaid, 12 months)

```terraform
resource "tencentcloudextend_teo_plan" "standard" {
  plan_type  = "standard"
  period     = 12
  renew_flag = "on"
}
```

### Full example with zone binding

```terraform
resource "tencentcloudextend_teo_plan" "enterprise" {
  plan_type = "enterprise"
}

resource "tencentcloudextend_teo_zone" "example" {
  zone_name = "example.com"
  type      = "dnsPodAccess"
  area      = "global"
  plan_id   = tencentcloudextend_teo_plan.enterprise.plan_id
}
```

## Schema

### Required

- `plan_type` (String) Plan type. Changing this forces a new resource. Valid values:
  - `personal` — Personal edition, prepaid
  - `basic` — Basic edition, prepaid
  - `standard` — Standard edition, prepaid
  - `enterprise` — Enterprise edition, postpaid

### Optional

- `period` (Number) Subscription period in months, for prepaid plans only (`personal`, `basic`, `standard`). Valid values: `1`–`12`, `24`, `36`. Defaults to `1`. Changing this forces a new resource.
- `renew_flag` (String) Auto-renewal switch for prepaid plans. Valid values: `on`, `off`. Defaults to `off`.
- `auto_use_voucher` (Boolean) Whether to automatically apply vouchers at purchase time. Only applicable for prepaid plans. Defaults to `false`. Changing this forces a new resource.

### Read-Only

- `plan_id` (String) The plan ID, e.g. `edgeone-2unuvzjmmn2q`. Use this to bind zones via `tencentcloud_teo_zone`.
- `deal_name` (String) The order number returned when the plan was created.
- `status` (String) Current plan status: `normal`, `expiring-soon`, `expired`, `isolated`.
- `area` (String) Acceleration region: `mainland`, `overseas`, `global`.

## Import

TEO plans can be imported using the plan ID:

```shell
terraform import tencentcloudextend_teo_plan.example edgeone-2unuvzjmmn2q
```

-> **Note on deletion:** `DestroyPlan` requires the plan to be expired (enterprise plans are exempt) and all zones under the plan to be disabled or deleted. Terraform will return an error if these conditions are not met.
