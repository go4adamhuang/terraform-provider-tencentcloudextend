package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common"
	"github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common/profile"
	live "github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/live/v20180801"
)

var _ resource.Resource = &CssDomainVerifyResource{}

type CssDomainVerifyResource struct {
	client *live.Client
}

type CssDomainVerifyModel struct {
	Domain        types.String `tfsdk:"domain"`
	MainDomain    types.String `tfsdk:"main_domain"`
	VerifyContent types.String `tfsdk:"verify_content"`
}

func NewCssDomainVerifyResource() resource.Resource {
	return &CssDomainVerifyResource{}
}

func (r *CssDomainVerifyResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_css_domain_verify"
}

func (r *CssDomainVerifyResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Fetches the DNS TXT record value required to verify CSS domain ownership via dnsCheck.\n\n" +
			"Use the computed `verify_content` to create a `cssauth.<main_domain>` TXT record " +
			"(e.g. via `tencentcloud_dnspod_record`), then add `tencentcloudextend_css_domain` " +
			"with `depends_on` pointing to the DNS record resource.",
		Attributes: map[string]schema.Attribute{
			"domain": schema.StringAttribute{
				Required:    true,
				Description: "Any CSS domain under the main domain you want to verify (e.g. pull-global.hzjnf.com or hzjnf.com). Changing this forces a new resource.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"main_domain": schema.StringAttribute{
				Computed:    true,
				Description: "The root domain that needs to be verified (e.g. hzjnf.com). Returned by TencentCloud.",
			},
			"verify_content": schema.StringAttribute{
				Computed:    true,
				Description: "The value to set on the cssauth.<main_domain> TXT record for dnsCheck verification.",
			},
		},
	}
}

func (r *CssDomainVerifyResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	cfg, ok := req.ProviderData.(*ClientConfig)
	if !ok {
		resp.Diagnostics.AddError("Unexpected provider data type", fmt.Sprintf("expected *ClientConfig, got %T", req.ProviderData))
		return
	}

	client, err := live.NewClient(common.NewCredential(cfg.SecretID, cfg.SecretKey), cfg.Region, profile.NewClientProfile())
	if err != nil {
		resp.Diagnostics.AddError("Failed to create CSS/Live client", err.Error())
		return
	}
	r.client = client
}

func (r *CssDomainVerifyResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan CssDomainVerifyModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	authReq := live.NewAuthenticateDomainOwnerRequest()
	authReq.DomainName = strPtr(plan.Domain.ValueString())
	authReq.VerifyType = strPtr("dnsCheck")

	authResp, err := r.client.AuthenticateDomainOwner(authReq)
	if err != nil || authResp.Response == nil || authResp.Response.Content == nil || authResp.Response.MainDomain == nil {
		resp.Diagnostics.AddError("Failed to get domain ownership verification info", fmt.Sprintf("%v", err))
		return
	}

	plan.MainDomain = types.StringValue(*authResp.Response.MainDomain)
	plan.VerifyContent = types.StringValue(*authResp.Response.Content)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *CssDomainVerifyResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state CssDomainVerifyModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	authReq := live.NewAuthenticateDomainOwnerRequest()
	authReq.DomainName = strPtr(state.Domain.ValueString())
	authReq.VerifyType = strPtr("dnsCheck")

	authResp, err := r.client.AuthenticateDomainOwner(authReq)
	if err != nil || authResp.Response == nil {
		// Non-fatal: keep existing state if API is temporarily unavailable.
		return
	}

	if authResp.Response.MainDomain != nil {
		state.MainDomain = types.StringValue(*authResp.Response.MainDomain)
	}
	if authResp.Response.Content != nil {
		state.VerifyContent = types.StringValue(*authResp.Response.Content)
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

// Update is not applicable — all attributes are ForceNew.
func (r *CssDomainVerifyResource) Update(_ context.Context, _ resource.UpdateRequest, _ *resource.UpdateResponse) {
}

// Delete removes the resource from state only.
func (r *CssDomainVerifyResource) Delete(_ context.Context, _ resource.DeleteRequest, _ *resource.DeleteResponse) {
}
