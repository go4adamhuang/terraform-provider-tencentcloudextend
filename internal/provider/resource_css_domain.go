package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64default"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	tcerr "github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common/errors"
	"github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common"
	"github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common/profile"
	live "github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/live/v20180801"
)

const (
	cssDomainStatusEnabled       = uint64(1)
	errCodeDomainNeedVerifyOwner = "FailedOperation.DomainNeedVerifyOwner"
)

var _ resource.Resource = &CssDomainResource{}

type CssDomainResource struct {
	client *live.Client
}

type CssDomainModel struct {
	DomainName        types.String `tfsdk:"domain_name"`
	DomainType        types.Int64  `tfsdk:"domain_type"`
	PlayType          types.Int64  `tfsdk:"play_type"`
	IsDelayLive       types.Int64  `tfsdk:"is_delay_live"`
	IsMiniProgramLive types.Int64  `tfsdk:"is_mini_program_live"`
	VerifyOwnerType   types.String `tfsdk:"verify_owner_type"`
	Enable            types.Bool   `tfsdk:"enable"`
}

func NewCssDomainResource() resource.Resource {
	return &CssDomainResource{}
}

func (r *CssDomainResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_css_domain"
}

func (r *CssDomainResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages a Tencent Cloud CSS (Live Streaming) domain.\n\n" +
			"If the domain requires ownership verification, create a " +
			"`tencentcloudextend_css_domain_verify` resource first and use `depends_on`.",
		Attributes: map[string]schema.Attribute{
			"domain_name": schema.StringAttribute{
				Required:    true,
				Description: "Domain name. Changing this forces a new resource.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"domain_type": schema.Int64Attribute{
				Required:    true,
				Description: "Domain type: `0` push stream, `1` playback. Changing this forces a new resource.",
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.RequiresReplace(),
				},
			},
			"play_type": schema.Int64Attribute{
				Optional:    true,
				Computed:    true,
				Description: "Play type (only valid when domain_type=1): `1` Mainland China, `2` global, `3` outside Mainland China. Default: 1.",
				Default:     int64default.StaticInt64(1),
			},
			"is_delay_live": schema.Int64Attribute{
				Optional:    true,
				Computed:    true,
				Description: "Whether LCB: `0` LVB, `1` LCB. Default: 0. Changing this forces a new resource.",
				Default:     int64default.StaticInt64(0),
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.RequiresReplace(),
				},
			},
			"is_mini_program_live": schema.Int64Attribute{
				Optional:    true,
				Computed:    true,
				Description: "`0` LVB, `1` LVB on Mini Program. Default: 0. Changing this forces a new resource.",
				Default:     int64default.StaticInt64(0),
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.RequiresReplace(),
				},
			},
			"verify_owner_type": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Verification type passed to AddLiveDomain: `dbCheck` (default) or `dnsCheck`. Changing this forces a new resource.",
				Default:     stringdefault.StaticString("dbCheck"),
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"enable": schema.BoolAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Whether the domain is enabled. Default: true.",
				Default:     booldefault.StaticBool(true),
			},
		},
	}
}

func (r *CssDomainResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *CssDomainResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan CssDomainModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	addReq := live.NewAddLiveDomainRequest()
	addReq.DomainName = strPtr(plan.DomainName.ValueString())
	addReq.DomainType = uint64Ptr(uint64(plan.DomainType.ValueInt64()))
	addReq.PlayType = uint64Ptr(uint64(plan.PlayType.ValueInt64()))
	addReq.IsDelayLive = int64Ptr(plan.IsDelayLive.ValueInt64())
	addReq.IsMiniProgramLive = int64Ptr(plan.IsMiniProgramLive.ValueInt64())
	addReq.VerifyOwnerType = strPtr(plan.VerifyOwnerType.ValueString())

	if _, err := r.client.AddLiveDomain(addReq); err != nil {
		sdkErr, isSdkErr := err.(*tcerr.TencentCloudSDKError)
		if isSdkErr && sdkErr.Code == errCodeDomainNeedVerifyOwner {
			resp.Diagnostics.AddError(
				"Domain ownership not verified",
				fmt.Sprintf(
					"Domain %q requires ownership verification before it can be added to CSS.\n\n"+
						"Add the following resource and use depends_on:\n\n"+
						"  resource \"tencentcloudextend_css_domain_verify\" \"verify\" {\n"+
						"    domain_name = %q\n"+
						"  }",
					plan.DomainName.ValueString(),
					plan.DomainName.ValueString(),
				),
			)
			return
		}
		resp.Diagnostics.AddError("Failed to add CSS domain", err.Error())
		return
	}

	if err := r.setDomainEnabled(plan.DomainName.ValueString(), plan.Enable.ValueBool()); err != nil {
		resp.Diagnostics.AddError("Failed to set domain enable state", err.Error())
		return
	}

	domain, diags := r.describeDomain(plan.DomainName.ValueString())
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	if domain != nil {
		populateCssDomainModel(&plan, domain)
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *CssDomainResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state CssDomainModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	domain, diags := r.describeDomain(state.DomainName.ValueString())
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	if domain == nil {
		resp.State.RemoveResource(ctx)
		return
	}

	populateCssDomainModel(&state, domain)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *CssDomainResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state CssDomainModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	domainName := state.DomainName.ValueString()

	if !plan.PlayType.Equal(state.PlayType) {
		modReq := live.NewModifyLivePlayDomainRequest()
		modReq.DomainName = strPtr(domainName)
		modReq.PlayType = int64Ptr(plan.PlayType.ValueInt64())
		if _, err := r.client.ModifyLivePlayDomain(modReq); err != nil {
			resp.Diagnostics.AddError("Failed to update play_type", err.Error())
			return
		}
	}

	if !plan.Enable.Equal(state.Enable) {
		if err := r.setDomainEnabled(domainName, plan.Enable.ValueBool()); err != nil {
			resp.Diagnostics.AddError("Failed to set domain enable state", err.Error())
			return
		}
	}

	domain, diags := r.describeDomain(domainName)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	if domain != nil {
		populateCssDomainModel(&plan, domain)
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *CssDomainResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state CssDomainModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	delReq := live.NewDeleteLiveDomainRequest()
	delReq.DomainName = strPtr(state.DomainName.ValueString())
	delReq.DomainType = uint64Ptr(uint64(state.DomainType.ValueInt64()))

	if _, err := r.client.DeleteLiveDomain(delReq); err != nil {
		resp.Diagnostics.AddError("Failed to delete CSS domain", err.Error())
	}
}

func (r *CssDomainResource) describeDomain(domainName string) (*live.DomainInfo, diag.Diagnostics) {
	var diags diag.Diagnostics
	req := live.NewDescribeLiveDomainRequest()
	req.DomainName = strPtr(domainName)

	result, err := r.client.DescribeLiveDomain(req)
	if err != nil {
		diags.AddError("Failed to describe CSS domain", err.Error())
		return nil, diags
	}
	if result.Response == nil || result.Response.DomainInfo == nil {
		return nil, diags
	}
	return result.Response.DomainInfo, diags
}

func (r *CssDomainResource) setDomainEnabled(domainName string, enable bool) error {
	if enable {
		req := live.NewEnableLiveDomainRequest()
		req.DomainName = strPtr(domainName)
		_, err := r.client.EnableLiveDomain(req)
		return err
	}
	req := live.NewForbidLiveDomainRequest()
	req.DomainName = strPtr(domainName)
	_, err := r.client.ForbidLiveDomain(req)
	return err
}

func populateCssDomainModel(m *CssDomainModel, d *live.DomainInfo) {
	if d.Name != nil {
		m.DomainName = types.StringValue(*d.Name)
	}
	if d.Type != nil {
		m.DomainType = types.Int64Value(int64(*d.Type))
	}
	if d.PlayType != nil {
		m.PlayType = types.Int64Value(*d.PlayType)
	}
	if d.IsDelayLive != nil {
		m.IsDelayLive = types.Int64Value(*d.IsDelayLive)
	}
	if d.IsMiniProgramLive != nil {
		m.IsMiniProgramLive = types.Int64Value(*d.IsMiniProgramLive)
	}
	if d.Status != nil {
		m.Enable = types.BoolValue(*d.Status == cssDomainStatusEnabled)
	}
}
