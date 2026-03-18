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

# 單一帳號：CSS 和 DNSPod 都用同一個 profile
provider "tencentcloudextend" {
  profile = "stream"
}

# 雙帳號：CSS 用 stream，DNSPod 用 default（不同帳號管 DNS）
# provider "tencentcloudextend" {
#   profile     = "stream"
#   dns_profile = "default"
# }
