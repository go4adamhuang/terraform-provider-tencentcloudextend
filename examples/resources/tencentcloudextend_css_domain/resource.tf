# 使用 tccli profile（推薦，credentials 不入 code）
provider "tencentcloudextend" {
  profile     = "stream"   # CSS 帳號
  dns_profile = "default"  # DNSPod 帳號（用於自動驗證域名所有權）
}

# 播放域名（全球）
# dbCheck 通過時直接建立；否則自動走 dnsCheck + DNSPod TXT record 全自動驗證
resource "tencentcloudextend_css_domain" "pull_global" {
  domain_name = "pull-global.example.com"
  domain_type = 1
  play_type   = 2 # 1=中國大陸 2=全球 3=海外
}

output "verify_content" {
  description = "DNS TXT record value used for domain ownership verification (cssauth.<main_domain>). Empty if dbCheck succeeded directly."
  value       = tencentcloudextend_css_domain.pull_global.verify_content
}

# 推流域名
resource "tencentcloudextend_css_domain" "push" {
  domain_name = "push.example.com"
  domain_type = 0
}
