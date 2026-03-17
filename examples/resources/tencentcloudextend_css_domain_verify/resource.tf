# Verify domain ownership before adding subdomains to CSS.
# Once the main domain (e.g. hzjnf.com) is verified, all subdomains under it
# can be added to CSS without re-verification.

resource "tencentcloudextend_css_domain_verify" "hzjnf" {
  domain_name = "hzjnf.com"
}

resource "tencentcloudextend_css_domain" "pull_global" {
  domain_name = "pull-global.hzjnf.com"
  domain_type = 1
  play_type   = 2
  depends_on  = [tencentcloudextend_css_domain_verify.hzjnf]
}
