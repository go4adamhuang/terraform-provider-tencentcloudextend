package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common"
	"github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common/profile"
	teo "github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/teo/v20220901"
)

var _ resource.Resource = &TeoPlanResource{}
var _ resource.ResourceWithImportState = &TeoPlanResource{}

type TeoPlanResource struct {
	client *teo.Client
}

type TeoPlanModel struct {
	PlanType       types.String `tfsdk:"plan_type"`
	Period         types.Int64  `tfsdk:"period"`
	RenewFlag      types.String `tfsdk:"renew_flag"`
	AutoUseVoucher types.Bool   `tfsdk:"auto_use_voucher"`
	PlanID         types.String `tfsdk:"plan_id"`
	DealName       types.String `tfsdk:"deal_name"`
	Status         types.String `tfsdk:"status"`
	Area           types.String `tfsdk:"area"`
}

func NewTeoPlanResource() resource.Resource {
	return &TeoPlanResource{}
}

func (r *TeoPlanResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_teo_plan"
}

func (r *TeoPlanResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Creates an EdgeOne (TEO) billing plan. After creating a plan, use the official " +
			"`tencentcloud_teo_zone` resource to create a zone and bind it to this plan.",
		Attributes: map[string]schema.Attribute{
			"plan_type": schema.StringAttribute{
				Required: true,
				Description: "Plan type. Changing this forces a new resource. Valid values: " +
					"`personal` (prepaid), `basic` (prepaid), `standard` (prepaid), `enterprise` (postpaid).",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"period": schema.Int64Attribute{
				Optional: true,
				Description: "Subscription period in months, for prepaid plans only (`personal`, `basic`, `standard`). " +
					"Valid values: 1–12, 24, 36. Defaults to 1. Changing this forces a new resource.",
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.RequiresReplace(),
				},
			},
			"renew_flag": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Auto-renewal switch for prepaid plans. Valid values: `on`, `off`. Defaults to `off`.",
			},
			"auto_use_voucher": schema.BoolAttribute{
				Optional: true,
				Description: "Whether to automatically apply vouchers at purchase time. " +
					"Only applicable for prepaid plans. Defaults to `false`. Changing this forces a new resource.",
				PlanModifiers: []planmodifier.Bool{
					boolplanmodifier.RequiresReplace(),
				},
			},
			"plan_id": schema.StringAttribute{
				Computed:    true,
				Description: "The plan ID, e.g. `edgeone-2unuvzjmmn2q`.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"deal_name": schema.StringAttribute{
				Computed:    true,
				Description: "The order number returned when the plan was created.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"status": schema.StringAttribute{
				Computed:    true,
				Description: "Current plan status: `normal`, `expiring-soon`, `expired`, `isolated`.",
			},
			"area": schema.StringAttribute{
				Computed:    true,
				Description: "Acceleration region: `mainland`, `overseas`, `global`.",
			},
		},
	}
}

func (r *TeoPlanResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	cfg, ok := req.ProviderData.(*ClientConfig)
	if !ok {
		resp.Diagnostics.AddError("Unexpected provider data type", fmt.Sprintf("expected *ClientConfig, got %T", req.ProviderData))
		return
	}
	client, err := teo.NewClient(common.NewCredential(cfg.SecretID, cfg.SecretKey), cfg.Region, profile.NewClientProfile())
	if err != nil {
		resp.Diagnostics.AddError("Failed to create TEO client", err.Error())
		return
	}
	r.client = client
}

func (r *TeoPlanResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan TeoPlanModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	createReq := teo.NewCreatePlanRequest()
	createReq.PlanType = strPtr(plan.PlanType.ValueString())

	if !plan.AutoUseVoucher.IsNull() && !plan.AutoUseVoucher.IsUnknown() {
		if plan.AutoUseVoucher.ValueBool() {
			createReq.AutoUseVoucher = strPtr("true")
		} else {
			createReq.AutoUseVoucher = strPtr("false")
		}
	}

	isPrepaid := plan.PlanType.ValueString() != "enterprise"
	if isPrepaid {
		prepaid := &teo.PrepaidPlanParam{}
		if !plan.Period.IsNull() && !plan.Period.IsUnknown() {
			prepaid.Period = int64Ptr(plan.Period.ValueInt64())
		}
		if !plan.RenewFlag.IsNull() && !plan.RenewFlag.IsUnknown() {
			prepaid.RenewFlag = strPtr(plan.RenewFlag.ValueString())
		}
		createReq.PrepaidPlanParam = prepaid
	}

	createResp, err := r.client.CreatePlan(createReq)
	if err != nil {
		resp.Diagnostics.AddError("Failed to create TEO plan", err.Error())
		return
	}
	if createResp.Response == nil || createResp.Response.PlanId == nil {
		resp.Diagnostics.AddError("Unexpected empty response from CreatePlan", "")
		return
	}

	plan.PlanID = types.StringValue(*createResp.Response.PlanId)
	if createResp.Response.DealName != nil {
		plan.DealName = types.StringValue(*createResp.Response.DealName)
	}

	r.refreshPlan(ctx, &plan, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *TeoPlanResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state TeoPlanModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	r.refreshPlan(ctx, &state, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *TeoPlanResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var state TeoPlanModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var desiredRenewFlag types.String
	resp.Diagnostics.Append(req.Plan.GetAttribute(ctx, path.Root("renew_flag"), &desiredRenewFlag)...)
	if resp.Diagnostics.HasError() {
		return
	}

	modifyReq := teo.NewModifyPlanRequest()
	modifyReq.PlanId = strPtr(state.PlanID.ValueString())
	modifyReq.RenewFlag = &teo.RenewFlag{
		Switch: strPtr(desiredRenewFlag.ValueString()),
	}

	_, err := r.client.ModifyPlan(modifyReq)
	if err != nil {
		resp.Diagnostics.AddError("Failed to update TEO plan", err.Error())
		return
	}

	state.RenewFlag = desiredRenewFlag
	r.refreshPlan(ctx, &state, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *TeoPlanResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state TeoPlanModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	destroyReq := teo.NewDestroyPlanRequest()
	destroyReq.PlanId = strPtr(state.PlanID.ValueString())

	_, err := r.client.DestroyPlan(destroyReq)
	if err != nil {
		resp.Diagnostics.AddError(
			"Failed to destroy TEO plan",
			fmt.Sprintf("%s\n\nNote: DestroyPlan requires the plan to be expired (except enterprise) "+
				"and all zones to be disabled or deleted.", err.Error()),
		)
	}
}

func (r *TeoPlanResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	state := TeoPlanModel{
		PlanID: types.StringValue(req.ID),
	}
	r.refreshPlan(ctx, &state, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *TeoPlanResource) refreshPlan(_ context.Context, m *TeoPlanModel, diags *diag.Diagnostics) {
	descReq := teo.NewDescribePlansRequest()
	descReq.Filters = []*teo.Filter{
		{Name: strPtr("plan-id"), Values: []*string{strPtr(m.PlanID.ValueString())}},
	}

	descResp, err := r.client.DescribePlans(descReq)
	if err != nil {
		diags.AddError("Failed to read TEO plan", err.Error())
		return
	}
	if descResp.Response == nil || len(descResp.Response.Plans) == 0 {
		diags.AddError("TEO plan not found", fmt.Sprintf("plan_id %s was not found", m.PlanID.ValueString()))
		return
	}

	p := descResp.Response.Plans[0]
	if p.Status != nil {
		m.Status = types.StringValue(*p.Status)
	}
	if p.Area != nil {
		m.Area = types.StringValue(*p.Area)
	}
	if p.AutoRenewal != nil {
		if *p.AutoRenewal {
			m.RenewFlag = types.StringValue("on")
		} else {
			m.RenewFlag = types.StringValue("off")
		}
	}
}
