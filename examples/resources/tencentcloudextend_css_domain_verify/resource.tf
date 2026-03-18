# Retrieve the DNS TXT verification value for CSS domain ownership.
# Once the main domain (e.g. hzjnf.com) is verified, all subdomains under it
# can be added to CSS without re-verification.

resource "tencentcloudextend_css_domain_verify" "example" {
  domain = "example.com"
}

# Add cssauth.<main_domain> TXT record with the value from verify_content,
# then use the official tencentcloud provider to manage CSS domains:
#
# resource "tencentcloud_css_domain" "pull_global" {
#   domain_name = "pull.example.com"
#   domain_type = 1
#   play_type   = 2
#   depends_on  = [tencentcloudextend_css_domain_verify.example]
# }
