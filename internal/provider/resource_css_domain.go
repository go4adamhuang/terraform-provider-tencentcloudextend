package provider

import (
	"context"
	"fmt"
	"time"

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
	dnspod "github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/dnspod/v20210323"
	live "github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/live/v20180801"
)

const (
	cssDomainStatusEnabled        = uint64(1)
	errCodeDomainNeedVerifyOwner  = "FailedOperation.DomainNeedVerifyOwner"

	dnsVerifySubdomain = "cssauth"
	dnsVerifyRecordTTL = int64(600)
	verifyPollInterval = 5 * time.Second
	verifyPollMaxTries = 12 // 12 × 5s = 60s
)

var _ resource.Resource = &CssDomainResource{}

type CssDomainResource struct {
	cssClient *live.Client
	dnsClient *dnspod.Client
}

type CssDomainModel struct {
	DomainName        types.String `tfsdk:"domain_name"`
	DomainType        types.Int64  `tfsdk:"domain_type"`
	PlayType          types.Int64  `tfsdk:"play_type"`
	IsDelayLive       types.Int64  `tfsdk:"is_delay_live"`
	IsMiniProgramLive types.Int64  `tfsdk:"is_mini_program_live"`
	VerifyOwnerType   types.String `tfsdk:"verify_owner_type"`
	Enable            types.Bool   `tfsdk:"enable"`
	VerifyContent     types.String `tfsdk:"verify_content"`
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
			"Create flow:\n" +
			"  1. Attempt AddLiveDomain with dbCheck.\n" +
			"  2. If domain ownership verification is required, automatically:\n" +
			"     a. Calls AuthenticateDomainOwner (dnsCheck) to obtain the TXT record value.\n" +
			"     b. Creates/reuses the cssauth TXT record in DNSPod.\n" +
			"     c. Polls until TencentCloud confirms ownership (up to 60 s).\n" +
			"     d. Retries AddLiveDomain with dnsCheck.\n\n" +
			"DNS credentials can differ from CSS credentials — configure dns_profile or " +
			"dns_secret_id/dns_secret_key on the provider block.",
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
				Description: "Initial verification type for the first AddLiveDomain attempt: `dbCheck` (default) or `dnsCheck`. If the attempt fails with ownership error, the resource automatically falls back to the full dnsCheck+DNSPod automation flow regardless of this value.",
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
			"verify_content": schema.StringAttribute{
				Computed:    true,
				Description: "TXT record value used for dnsCheck domain ownership verification (cssauth.<main_domain>). Populated when the automatic dnsCheck flow is triggered.",
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

	cpf := profile.NewClientProfile()

	cssClient, err := live.NewClient(common.NewCredential(cfg.SecretID, cfg.SecretKey), cfg.Region, cpf)
	if err != nil {
		resp.Diagnostics.AddError("Failed to create CSS/Live client", err.Error())
		return
	}
	r.cssClient = cssClient

	dnsClient, err := dnspod.NewClient(common.NewCredential(cfg.DNSSecretID, cfg.DNSSecretKey), "", cpf)
	if err != nil {
		resp.Diagnostics.AddError("Failed to create DNSPod client", err.Error())
		return
	}
	r.dnsClient = dnsClient
}

func (r *CssDomainResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan CssDomainModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	domainName := plan.DomainName.ValueString()

	// ── Step 1: try AddLiveDomain with the configured verify type (default: dbCheck) ──
	addErr := r.addLiveDomain(plan, plan.VerifyOwnerType.ValueString())

	if addErr != nil {
		sdkErr, isSdkErr := addErr.(*tcerr.TencentCloudSDKError)

		if !isSdkErr || sdkErr.Code != errCodeDomainNeedVerifyOwner {
			// Unrelated error — fail immediately.
			resp.Diagnostics.AddError("Failed to add CSS domain", addErr.Error())
			return
		}

		// ── Step 2: ownership verification required — automate dnsCheck via DNSPod ──
		verifyContent, diags := r.autoVerifyDomain(ctx, domainName)
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return
		}
		plan.VerifyContent = verifyContent

		// ── Step 3: retry AddLiveDomain with dnsCheck ──
		if err := r.addLiveDomain(plan, "dnsCheck"); err != nil {
			resp.Diagnostics.AddError("Failed to add CSS domain after dnsCheck verification", err.Error())
			return
		}
	} else {
		plan.VerifyContent = types.StringValue("")
	}

	// Enable / disable.
	if err := r.setDomainEnabled(domainName, plan.Enable.ValueBool()); err != nil {
		resp.Diagnostics.AddError("Failed to set domain enable state", err.Error())
		return
	}

	// Refresh from API.
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
		if _, err := r.cssClient.ModifyLivePlayDomain(modReq); err != nil {
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

	// verify_content is immutable after create.
	plan.VerifyContent = state.VerifyContent

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

	if _, err := r.cssClient.DeleteLiveDomain(delReq); err != nil {
		resp.Diagnostics.AddError("Failed to delete CSS domain", err.Error())
	}
}

// addLiveDomain calls AddLiveDomain with the given verifyOwnerType.
func (r *CssDomainResource) addLiveDomain(plan CssDomainModel, verifyOwnerType string) error {
	req := live.NewAddLiveDomainRequest()
	req.DomainName = strPtr(plan.DomainName.ValueString())
	req.DomainType = uint64Ptr(uint64(plan.DomainType.ValueInt64()))
	req.PlayType = uint64Ptr(uint64(plan.PlayType.ValueInt64()))
	req.IsDelayLive = int64Ptr(plan.IsDelayLive.ValueInt64())
	req.IsMiniProgramLive = int64Ptr(plan.IsMiniProgramLive.ValueInt64())
	req.VerifyOwnerType = strPtr(verifyOwnerType)
	_, err := r.cssClient.AddLiveDomain(req)
	return err
}

// autoVerifyDomain implements the full dnsCheck automation:
//  1. Gets TXT record content from AuthenticateDomainOwner.
//  2. Creates/reuses cssauth TXT record in DNSPod.
//  3. Polls until TencentCloud confirms ownership (≤ 60 s).
func (r *CssDomainResource) autoVerifyDomain(ctx context.Context, domainName string) (types.String, diag.Diagnostics) {
	var diags diag.Diagnostics

	// 1. Get TXT record content.
	authReq := live.NewAuthenticateDomainOwnerRequest()
	authReq.DomainName = strPtr(domainName)
	authReq.VerifyType = strPtr("dnsCheck")

	authResp, err := r.cssClient.AuthenticateDomainOwner(authReq)
	if err != nil || authResp.Response == nil || authResp.Response.Content == nil || authResp.Response.MainDomain == nil {
		diags.AddError("Failed to get domain ownership verification info", fmt.Sprintf("%v", err))
		return types.StringValue(""), diags
	}
	txtValue := *authResp.Response.Content
	mainDomain := *authResp.Response.MainDomain

	// If already verified, skip DNS step.
	if authResp.Response.Status != nil && *authResp.Response.Status >= 0 {
		return types.StringValue(txtValue), diags
	}

	// 2. Ensure cssauth TXT record exists in DNSPod.
	if err := r.ensureDNSTXTRecord(mainDomain, txtValue); err != nil {
		diags.AddError("Failed to create DNS TXT record for domain verification", err.Error())
		return types.StringValue(txtValue), diags
	}

	// 3. Poll until verified.
	verified := false
	for i := 0; i < verifyPollMaxTries; i++ {
		select {
		case <-ctx.Done():
			diags.AddError("Context cancelled while waiting for domain verification", ctx.Err().Error())
			return types.StringValue(txtValue), diags
		case <-time.After(verifyPollInterval):
		}

		pollResp, err := r.cssClient.AuthenticateDomainOwner(authReq)
		if err == nil && pollResp.Response != nil && pollResp.Response.Status != nil && *pollResp.Response.Status >= 0 {
			verified = true
			break
		}
	}

	if !verified {
		diags.AddError(
			"Domain ownership verification timed out",
			fmt.Sprintf("Waited %s but TencentCloud did not confirm ownership of %s. "+
				"DNS TXT record cssauth.%s = %s has been created. "+
				"Please wait for DNS propagation and retry.",
				time.Duration(verifyPollMaxTries)*verifyPollInterval, domainName, mainDomain, txtValue),
		)
		return types.StringValue(txtValue), diags
	}

	return types.StringValue(txtValue), diags
}

// ensureDNSTXTRecord creates the cssauth TXT record in DNSPod if it doesn't already exist.
func (r *CssDomainResource) ensureDNSTXTRecord(mainDomain, txtValue string) error {
	// Check if record already exists.
	listReq := dnspod.NewDescribeRecordListRequest()
	listReq.Domain = strPtr(mainDomain)
	listReq.Subdomain = strPtr(dnsVerifySubdomain)
	listReq.RecordType = strPtr("TXT")

	listResp, err := r.dnsClient.DescribeRecordList(listReq)
	if err == nil && listResp.Response != nil && len(listResp.Response.RecordList) > 0 {
		// TXT record already exists — nothing to do.
		return nil
	}

	// Create the TXT record.
	createReq := dnspod.NewCreateRecordRequest()
	createReq.Domain = strPtr(mainDomain)
	createReq.SubDomain = strPtr(dnsVerifySubdomain)
	createReq.RecordType = strPtr("TXT")
	createReq.RecordLine = strPtr("默认")
	createReq.Value = strPtr(txtValue)
	createReq.TTL = uint64Ptr(uint64(dnsVerifyRecordTTL))

	if _, err := r.dnsClient.CreateRecord(createReq); err != nil {
		return fmt.Errorf("DNSPod CreateRecord failed: %w", err)
	}
	return nil
}

// describeDomain fetches a single domain by name. Returns nil if not found.
func (r *CssDomainResource) describeDomain(domainName string) (*live.DomainInfo, diag.Diagnostics) {
	var diags diag.Diagnostics

	req := live.NewDescribeLiveDomainRequest()
	req.DomainName = strPtr(domainName)

	result, err := r.cssClient.DescribeLiveDomain(req)
	if err != nil {
		diags.AddError("Failed to describe CSS domain", err.Error())
		return nil, diags
	}
	if result.Response == nil || result.Response.DomainInfo == nil {
		return nil, diags
	}
	return result.Response.DomainInfo, diags
}

// setDomainEnabled calls EnableLiveDomain or ForbidLiveDomain.
func (r *CssDomainResource) setDomainEnabled(domainName string, enable bool) error {
	if enable {
		req := live.NewEnableLiveDomainRequest()
		req.DomainName = strPtr(domainName)
		_, err := r.cssClient.EnableLiveDomain(req)
		return err
	}
	req := live.NewForbidLiveDomainRequest()
	req.DomainName = strPtr(domainName)
	_, err := r.cssClient.ForbidLiveDomain(req)
	return err
}

// populateCssDomainModel maps API fields onto the Terraform model.
// verify_content is intentionally excluded — it is only set on Create.
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
