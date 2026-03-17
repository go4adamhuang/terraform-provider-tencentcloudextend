# 使用 tccli profile（推薦，credentials 不入 code）
provider "tencentcloudextend" {
  profile     = "stream"   # CSS 帳號
  dns_profile = "default"  # DNSPod 帳號（如與 CSS 帳號相同可省略）
}

# 先驗證主域名所有權（一次驗證，所有子域名共用）
resource "tencentcloudextend_css_domain_verify" "example" {
  domain_name = "example.com"
}

# 播放域名（全球）
resource "tencentcloudextend_css_domain" "pull_global" {
  domain_name = "pull-global.example.com"
  domain_type = 1
  play_type   = 2 # 1=中國大陸 2=全球 3=海外
  depends_on  = [tencentcloudextend_css_domain_verify.example]
}

# 推流域名
resource "tencentcloudextend_css_domain" "push" {
  domain_name = "push.example.com"
  domain_type = 0
  depends_on  = [tencentcloudextend_css_domain_verify.example]
}
