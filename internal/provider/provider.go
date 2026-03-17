package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/function"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ provider.Provider = &TencentCloudProvider{}
var _ provider.ProviderWithFunctions = &TencentCloudProvider{}

type TencentCloudProvider struct {
	version string
}

type TencentCloudProviderModel struct {
	SecretID  types.String `tfsdk:"secret_id"`
	SecretKey types.String `tfsdk:"secret_key"`
	Region    types.String `tfsdk:"region"`
	Profile   types.String `tfsdk:"profile"`

	// Separate credentials for DNSPod operations.
	// Falls back to the main credentials if not set.
	DNSSecretID  types.String `tfsdk:"dns_secret_id"`
	DNSSecretKey types.String `tfsdk:"dns_secret_key"`
	DNSProfile   types.String `tfsdk:"dns_profile"`
}

// ClientConfig holds resolved credentials passed to every resource and data source.
type ClientConfig struct {
	SecretID  string
	SecretKey string
	Region    string

	// DNSPod credentials (for DNS record management).
	// Falls back to SecretID/SecretKey if not explicitly set.
	DNSSecretID  string
	DNSSecretKey string
}

func New(version string) func() provider.Provider {
	return func() provider.Provider {
		return &TencentCloudProvider{version: version}
	}
}

func (p *TencentCloudProvider) Metadata(_ context.Context, _ provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "tencentcloudextend"
	resp.Version = p.version
}

func (p *TencentCloudProvider) Schema(_ context.Context, _ provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Extended Tencent Cloud provider (tencentcloudextend) for resources and data sources not covered by, or rewritten from, the official tencentcloudstack/tencentcloud provider.",
		Attributes: map[string]schema.Attribute{
			"secret_id": schema.StringAttribute{
				Optional:    true,
				Description: "Tencent Cloud secret ID (CSS/main account). Priority: explicit > profile > TENCENTCLOUD_SECRET_ID env var.",
			},
			"secret_key": schema.StringAttribute{
				Optional:    true,
				Sensitive:   true,
				Description: "Tencent Cloud secret key (CSS/main account). Priority: explicit > profile > TENCENTCLOUD_SECRET_KEY env var.",
			},
			"region": schema.StringAttribute{
				Optional:    true,
				Description: "Tencent Cloud region (e.g. ap-guangzhou). Priority: explicit > profile > TENCENTCLOUD_REGION env var.",
			},
			"profile": schema.StringAttribute{
				Optional:    true,
				Description: "tccli profile for CSS/main account. Loads credentials from ~/.tccli/<profile>.credential. Falls back to TENCENTCLOUD_PROFILE env var.",
			},
			"dns_secret_id": schema.StringAttribute{
				Optional:    true,
				Description: "Secret ID for DNSPod operations (may differ from the CSS account). Falls back to dns_profile, then to the main account credentials.",
			},
			"dns_secret_key": schema.StringAttribute{
				Optional:    true,
				Sensitive:   true,
				Description: "Secret key for DNSPod operations. Falls back to dns_profile, then to the main account credentials.",
			},
			"dns_profile": schema.StringAttribute{
				Optional:    true,
				Description: "tccli profile for DNSPod operations. Falls back to TENCENTCLOUD_DNS_PROFILE env var, then to the main account credentials.",
			},
		},
	}
}

func (p *TencentCloudProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	var config TencentCloudProviderModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// ── Main (CSS) credentials ──
	secretID := os.Getenv("TENCENTCLOUD_SECRET_ID")
	secretKey := os.Getenv("TENCENTCLOUD_SECRET_KEY")
	region := os.Getenv("TENCENTCLOUD_REGION")

	profileName := os.Getenv("TENCENTCLOUD_PROFILE")
	if !config.Profile.IsNull() && config.Profile.ValueString() != "" {
		profileName = config.Profile.ValueString()
	}
	if profileName != "" {
		pID, pKey, pRegion, err := loadTccliProfile(profileName)
		if err != nil {
			resp.Diagnostics.AddError(fmt.Sprintf("Failed to load tccli profile %q", profileName), err.Error())
			return
		}
		if pID != "" {
			secretID = pID
		}
		if pKey != "" {
			secretKey = pKey
		}
		if pRegion != "" {
			region = pRegion
		}
	}
	if !config.SecretID.IsNull() && config.SecretID.ValueString() != "" {
		secretID = config.SecretID.ValueString()
	}
	if !config.SecretKey.IsNull() && config.SecretKey.ValueString() != "" {
		secretKey = config.SecretKey.ValueString()
	}
	if !config.Region.IsNull() && config.Region.ValueString() != "" {
		region = config.Region.ValueString()
	}

	// ── DNS (DNSPod) credentials — fall back to main if not set ──
	dnsSecretID := secretID
	dnsSecretKey := secretKey

	dnsProfileName := os.Getenv("TENCENTCLOUD_DNS_PROFILE")
	if !config.DNSProfile.IsNull() && config.DNSProfile.ValueString() != "" {
		dnsProfileName = config.DNSProfile.ValueString()
	}
	if dnsProfileName != "" {
		pID, pKey, _, err := loadTccliProfile(dnsProfileName)
		if err != nil {
			resp.Diagnostics.AddError(fmt.Sprintf("Failed to load dns tccli profile %q", dnsProfileName), err.Error())
			return
		}
		if pID != "" {
			dnsSecretID = pID
		}
		if pKey != "" {
			dnsSecretKey = pKey
		}
	}
	if !config.DNSSecretID.IsNull() && config.DNSSecretID.ValueString() != "" {
		dnsSecretID = config.DNSSecretID.ValueString()
	}
	if !config.DNSSecretKey.IsNull() && config.DNSSecretKey.ValueString() != "" {
		dnsSecretKey = config.DNSSecretKey.ValueString()
	}

	clientCfg := &ClientConfig{
		SecretID:     secretID,
		SecretKey:    secretKey,
		Region:       region,
		DNSSecretID:  dnsSecretID,
		DNSSecretKey: dnsSecretKey,
	}

	resp.DataSourceData = clientCfg
	resp.ResourceData = clientCfg
}

func (p *TencentCloudProvider) Resources(_ context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		NewCssDomainResource,
	}
}

func (p *TencentCloudProvider) DataSources(_ context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{}
}

func (p *TencentCloudProvider) Functions(_ context.Context) []func() function.Function {
	return []func() function.Function{}
}

// loadTccliProfile reads credentials and region from tccli profile files.
func loadTccliProfile(profileName string) (secretID, secretKey, region string, err error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", "", "", fmt.Errorf("cannot determine home directory: %w", err)
	}
	base := filepath.Join(home, ".tccli")

	credPath := filepath.Join(base, profileName+".credential")
	credData, err := os.ReadFile(credPath)
	if err != nil {
		return "", "", "", fmt.Errorf("cannot read %s: %w", credPath, err)
	}
	var cred struct {
		SecretId  string `json:"secretId"`
		SecretKey string `json:"secretKey"`
	}
	if err := json.Unmarshal(credData, &cred); err != nil {
		return "", "", "", fmt.Errorf("cannot parse %s: %w", credPath, err)
	}
	secretID = cred.SecretId
	secretKey = cred.SecretKey

	cfgPath := filepath.Join(base, profileName+".configure")
	if cfgData, err := os.ReadFile(cfgPath); err == nil {
		var cfg struct {
			SysParam struct {
				Region string `json:"region"`
			} `json:"_sys_param"`
		}
		if json.Unmarshal(cfgData, &cfg) == nil {
			region = cfg.SysParam.Region
		}
	}

	return secretID, secretKey, region, nil
}
