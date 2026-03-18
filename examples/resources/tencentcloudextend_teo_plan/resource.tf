# Enterprise plan (postpaid, no period/renew config needed)
resource "tencentcloudextend_teo_plan" "enterprise" {
  plan_type = "enterprise"
}

# Standard plan (prepaid, 12 months, auto-renewal on)
resource "tencentcloudextend_teo_plan" "standard" {
  plan_type  = "standard"
  period     = 12
  renew_flag = "on"
}

# Use plan_id to bind a zone via the official provider
resource "tencentcloud_teo_zone" "example" {
  zone_name = "example.com"
  plan_id   = tencentcloudextend_teo_plan.enterprise.plan_id
}
