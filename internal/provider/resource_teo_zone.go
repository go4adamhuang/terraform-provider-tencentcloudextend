package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/mapplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common"
	"github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common/profile"
	teo "github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/teo/v20220901"
)

var _ resource.Resource = &TeoZoneResource{}
var _ resource.ResourceWithImportState = &TeoZoneResource{}

type TeoZoneResource struct {
	client *teo.Client
}

type TeoZoneModel struct {
	ZoneName             types.String `tfsdk:"zone_name"`
	Type                 types.String `tfsdk:"type"`
	Area                 types.String `tfsdk:"area"`
	PlanID               types.String `tfsdk:"plan_id"`
	AliasZoneName        types.String `tfsdk:"alias_zone_name"`
	Paused               types.Bool   `tfsdk:"paused"`
	Tags                 types.Map    `tfsdk:"tags"`
	WorkModeInfos        types.List   `tfsdk:"work_mode_infos"`
	ZoneID               types.String `tfsdk:"zone_id"`
	Status               types.String `tfsdk:"status"`
	NameServers          types.List   `tfsdk:"name_servers"`
	OwnershipVerification types.List  `tfsdk:"ownership_verification"`
}

func NewTeoZoneResource() resource.Resource {
	return &TeoZoneResource{}
}

func (r *TeoZoneResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_teo_zone"
}

func (r *TeoZoneResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Creates and manages a Tencent Cloud EdgeOne (TEO) zone. " +
			"Supports all parameters from the official tencentcloud_teo_zone resource plus the `dnsPodAccess` zone type " +
			"which is not supported by the official provider. " +
			"Also correctly handles the API constraint (ZoneHasHostsModifyConflict) by splitting " +
			"`area` changes into a separate API call from other field changes.",
		Attributes: map[string]schema.Attribute{
			"zone_name": schema.StringAttribute{
				Required:    true,
				Description: "Site name. For CNAME/NS/DNSPod access, pass the second-level domain (e.g. `example.com`). Leave empty for no-domain access.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"type": schema.StringAttribute{
				Required: true,
				Description: "Site access type. Valid values: " +
					"`partial` (CNAME access), `full` (NS access), `noDomainAccess` (no-domain access), " +
					"`dnsPodAccess` (DNSPod managed access; requires domain already hosted in DNSPod). " +
					"Changing this forces a new resource.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"area": schema.StringAttribute{
				Optional: true,
				Computed: true,
				Description: "Acceleration region for L7 domains. Valid values: `global`, `mainland`, `overseas`. " +
					"Applicable for `partial`, `full`, and `dnsPodAccess` types. Leave empty for `noDomainAccess`.",
			},
			"plan_id": schema.StringAttribute{
				Required:    true,
				Description: "Target plan ID to bind this zone to.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"alias_zone_name": schema.StringAttribute{
				Optional:    true,
				Description: "Alias site identifier. Alphanumeric, `-`, `_`, `.` characters, up to 200 chars.",
			},
			"paused": schema.BoolAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Whether the site is disabled.",
				PlanModifiers: []planmodifier.Bool{
					boolplanmodifier.UseStateForUnknown(),
				},
			},
			"tags": schema.MapAttribute{
				Optional:    true,
				ElementType: types.StringType,
				Description: "Tag key-value pairs. Note: tags can only be set at creation time; use the TencentCloud tag service to update tags after creation.",
				PlanModifiers: []planmodifier.Map{
					mapplanmodifier.RequiresReplace(),
				},
			},
			"work_mode_infos": schema.ListNestedAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Configuration group work mode settings. Supports version control or immediate effect per config group.",
				PlanModifiers: []planmodifier.List{
					listplanmodifier.UseStateForUnknown(),
				},
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"config_group_type": schema.StringAttribute{
							Required:    true,
							Description: "Configuration group type. Valid values: `l7_acceleration`, `edge_functions`.",
						},
						"work_mode": schema.StringAttribute{
							Required:    true,
							Description: "Work mode. Valid values: `immediate_effect`, `version_control`.",
						},
					},
				},
			},
			"zone_id": schema.StringAttribute{
				Computed:    true,
				Description: "Zone ID, e.g. `zone-2noz78a8ev6e`.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"status": schema.StringAttribute{
				Computed:    true,
				Description: "Zone status: `active` (NS switched), `pending` (NS not switched), `moved` (NS moved away), `deactivated` (blocked), `initializing` (pending plan binding).",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name_servers": schema.ListAttribute{
				Computed:    true,
				ElementType: types.StringType,
				Description: "NS list allocated by Tencent Cloud. Only populated for `full` (NS) zone type.",
				PlanModifiers: []planmodifier.List{
					listplanmodifier.UseStateForUnknown(),
				},
			},
			"ownership_verification": schema.ListNestedAttribute{
				Computed:    true,
				Description: "Ownership verification information. Only populated when verification is required (CNAME/partial zones).",
				PlanModifiers: []planmodifier.List{
					listplanmodifier.UseStateForUnknown(),
				},
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"dns_verification": schema.ListNestedAttribute{
							Computed:    true,
							Description: "DNS TXT record verification details.",
							NestedObject: schema.NestedAttributeObject{
								Attributes: map[string]schema.Attribute{
									"subdomain": schema.StringAttribute{
										Computed:    true,
										Description: "Host record (subdomain).",
									},
									"record_type": schema.StringAttribute{
										Computed:    true,
										Description: "DNS record type (e.g. TXT).",
									},
									"record_value": schema.StringAttribute{
										Computed:    true,
										Description: "DNS record value.",
									},
								},
							},
						},
					},
				},
			},
		},
	}
}

func (r *TeoZoneResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *TeoZoneResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan TeoZoneModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	createReq := teo.NewCreateZoneRequest()
	createReq.ZoneName = strPtr(plan.ZoneName.ValueString())
	createReq.Type = strPtr(plan.Type.ValueString())
	createReq.PlanId = strPtr(plan.PlanID.ValueString())
	if !plan.Area.IsNull() && !plan.Area.IsUnknown() && plan.Area.ValueString() != "" {
		createReq.Area = strPtr(plan.Area.ValueString())
	}
	if !plan.AliasZoneName.IsNull() && !plan.AliasZoneName.IsUnknown() {
		createReq.AliasZoneName = strPtr(plan.AliasZoneName.ValueString())
	}
	if !plan.Tags.IsNull() && !plan.Tags.IsUnknown() {
		createReq.Tags = expandTags(ctx, plan.Tags, &resp.Diagnostics)
		if resp.Diagnostics.HasError() {
			return
		}
	}

	createResp, err := r.client.CreateZone(createReq)
	if err != nil {
		resp.Diagnostics.AddError("Failed to create TEO zone", err.Error())
		return
	}
	if createResp.Response == nil || createResp.Response.ZoneId == nil {
		resp.Diagnostics.AddError("Unexpected empty response from CreateZone", "")
		return
	}
	plan.ZoneID = types.StringValue(*createResp.Response.ZoneId)

	// paused: default is false (active); only call ModifyZoneStatus if explicitly paused
	if !plan.Paused.IsNull() && !plan.Paused.IsUnknown() && plan.Paused.ValueBool() {
		r.setPaused(plan.ZoneID.ValueString(), true, &resp.Diagnostics)
		if resp.Diagnostics.HasError() {
			return
		}
	}

	// work_mode_infos
	if !plan.WorkModeInfos.IsNull() && !plan.WorkModeInfos.IsUnknown() {
		r.setWorkMode(ctx, plan.ZoneID.ValueString(), plan.WorkModeInfos, &resp.Diagnostics)
		if resp.Diagnostics.HasError() {
			return
		}
	}

	r.refresh(ctx, &plan, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *TeoZoneResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state TeoZoneModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	r.refresh(ctx, &state, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *TeoZoneResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var state, plan TeoZoneModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	zoneID := state.ZoneID.ValueString()
	plan.ZoneID = state.ZoneID

	// area: must be changed in a separate API call (ZoneHasHostsModifyConflict if combined)
	if !plan.Area.Equal(state.Area) {
		modReq := teo.NewModifyZoneRequest()
		modReq.ZoneId = strPtr(zoneID)
		modReq.Area = strPtr(plan.Area.ValueString())
		if _, err := r.client.ModifyZone(modReq); err != nil {
			resp.Diagnostics.AddError("Failed to update TEO zone area", err.Error())
			return
		}
	}

	// alias_zone_name: separate ModifyZone call (avoids conflict with area change above)
	if !plan.AliasZoneName.Equal(state.AliasZoneName) {
		modReq := teo.NewModifyZoneRequest()
		modReq.ZoneId = strPtr(zoneID)
		modReq.AliasZoneName = strPtr(plan.AliasZoneName.ValueString())
		if _, err := r.client.ModifyZone(modReq); err != nil {
			resp.Diagnostics.AddError("Failed to update TEO zone alias_zone_name", err.Error())
			return
		}
	}

	// paused: separate API
	if !plan.Paused.Equal(state.Paused) {
		r.setPaused(zoneID, plan.Paused.ValueBool(), &resp.Diagnostics)
		if resp.Diagnostics.HasError() {
			return
		}
	}

	// work_mode_infos: separate API
	if !plan.WorkModeInfos.Equal(state.WorkModeInfos) {
		r.setWorkMode(ctx, zoneID, plan.WorkModeInfos, &resp.Diagnostics)
		if resp.Diagnostics.HasError() {
			return
		}
	}

	r.refresh(ctx, &plan, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *TeoZoneResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state TeoZoneModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	delReq := teo.NewDeleteZoneRequest()
	delReq.ZoneId = strPtr(state.ZoneID.ValueString())
	if _, err := r.client.DeleteZone(delReq); err != nil {
		resp.Diagnostics.AddError("Failed to delete TEO zone", err.Error())
	}
}

func (r *TeoZoneResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	state := TeoZoneModel{
		ZoneID: types.StringValue(req.ID),
	}
	r.refresh(ctx, &state, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

// refresh reads the zone from the API and populates the model.
func (r *TeoZoneResource) refresh(_ context.Context, m *TeoZoneModel, diags *diag.Diagnostics) {
	descReq := teo.NewDescribeZonesRequest()
	descReq.Filters = []*teo.AdvancedFilter{
		{Name: strPtr("zone-id"), Values: []*string{strPtr(m.ZoneID.ValueString())}},
	}
	descResp, err := r.client.DescribeZones(descReq)
	if err != nil {
		diags.AddError("Failed to read TEO zone", err.Error())
		return
	}
	if descResp.Response == nil || len(descResp.Response.Zones) == 0 {
		diags.AddError("TEO zone not found", fmt.Sprintf("zone_id %q was not found", m.ZoneID.ValueString()))
		return
	}

	z := descResp.Response.Zones[0]

	if z.ZoneName != nil {
		m.ZoneName = types.StringValue(*z.ZoneName)
	}
	if z.Type != nil {
		m.Type = types.StringValue(*z.Type)
	}
	if z.Area != nil {
		m.Area = types.StringValue(*z.Area)
	}
	if z.Status != nil {
		m.Status = types.StringValue(*z.Status)
	}
	if z.AliasZoneName != nil {
		m.AliasZoneName = types.StringValue(*z.AliasZoneName)
	} else {
		m.AliasZoneName = types.StringValue("")
	}
	if z.Paused != nil {
		m.Paused = types.BoolValue(*z.Paused)
	} else {
		m.Paused = types.BoolValue(false)
	}

	// plan_id from Resources[0]
	if len(z.Resources) > 0 && z.Resources[0].Id != nil {
		m.PlanID = types.StringValue(*z.Resources[0].Id)
	}

	// tags
	if len(z.Tags) > 0 {
		tagMap := make(map[string]attr.Value, len(z.Tags))
		for _, t := range z.Tags {
			if t.TagKey != nil && t.TagValue != nil {
				tagMap[*t.TagKey] = types.StringValue(*t.TagValue)
			}
		}
		m.Tags = types.MapValueMust(types.StringType, tagMap)
	} else {
		m.Tags = types.MapValueMust(types.StringType, map[string]attr.Value{})
	}

	// work_mode_infos
	m.WorkModeInfos = flattenWorkModeInfos(z.WorkModeInfos, diags)

	// name_servers: only for NS (full) zone type
	if z.NSDetail != nil && len(z.NSDetail.NameServers) > 0 {
		nsVals := make([]attr.Value, len(z.NSDetail.NameServers))
		for i, ns := range z.NSDetail.NameServers {
			nsVals[i] = types.StringValue(*ns)
		}
		m.NameServers = types.ListValueMust(types.StringType, nsVals)
	} else {
		m.NameServers = types.ListValueMust(types.StringType, []attr.Value{})
	}

	// ownership_verification: from CNAMEDetail or NSDetail
	m.OwnershipVerification = flattenOwnershipVerification(z, diags)
}

func (r *TeoZoneResource) setPaused(zoneID string, paused bool, diags *diag.Diagnostics) {
	req := teo.NewModifyZoneStatusRequest()
	req.ZoneId = strPtr(zoneID)
	req.Paused = &paused
	if _, err := r.client.ModifyZoneStatus(req); err != nil {
		diags.AddError("Failed to update TEO zone paused status", err.Error())
	}
}

func (r *TeoZoneResource) setWorkMode(ctx context.Context, zoneID string, workModeInfos types.List, diags *diag.Diagnostics) {
	if workModeInfos.IsNull() || workModeInfos.IsUnknown() {
		return
	}

	type workModeItem struct {
		ConfigGroupType types.String `tfsdk:"config_group_type"`
		WorkMode        types.String `tfsdk:"work_mode"`
	}
	var items []workModeItem
	diags.Append(workModeInfos.ElementsAs(ctx, &items, false)...)
	if diags.HasError() {
		return
	}

	sdkItems := make([]*teo.ConfigGroupWorkModeInfo, len(items))
	for i, item := range items {
		sdkItems[i] = &teo.ConfigGroupWorkModeInfo{
			ConfigGroupType: strPtr(item.ConfigGroupType.ValueString()),
			WorkMode:        strPtr(item.WorkMode.ValueString()),
		}
	}

	req := teo.NewModifyZoneWorkModeRequest()
	req.ZoneId = strPtr(zoneID)
	req.WorkModeInfos = sdkItems
	if _, err := r.client.ModifyZoneWorkMode(req); err != nil {
		diags.AddError("Failed to update TEO zone work mode", err.Error())
	}
}

// expandTags converts types.Map → []*teo.Tag
func expandTags(_ context.Context, tagsMap types.Map, diags *diag.Diagnostics) []*teo.Tag {
	if tagsMap.IsNull() || tagsMap.IsUnknown() {
		return nil
	}
	result := make([]*teo.Tag, 0, len(tagsMap.Elements()))
	for k, v := range tagsMap.Elements() {
		sv, ok := v.(types.String)
		if !ok {
			diags.AddError("Invalid tag value", fmt.Sprintf("tag %q has non-string value", k))
			return nil
		}
		key := k
		val := sv.ValueString()
		result = append(result, &teo.Tag{TagKey: &key, TagValue: &val})
	}
	return result
}

// workModeInfoAttrTypes defines the attr.Type for a work_mode_infos element.
var workModeInfoAttrTypes = map[string]attr.Type{
	"config_group_type": types.StringType,
	"work_mode":         types.StringType,
}

func flattenWorkModeInfos(infos []*teo.ConfigGroupWorkModeInfo, diags *diag.Diagnostics) types.List {
	elemType := types.ObjectType{AttrTypes: workModeInfoAttrTypes}
	if len(infos) == 0 {
		return types.ListValueMust(elemType, []attr.Value{})
	}
	elems := make([]attr.Value, 0, len(infos))
	for _, info := range infos {
		obj, d := types.ObjectValue(workModeInfoAttrTypes, map[string]attr.Value{
			"config_group_type": types.StringValue(strDeref(info.ConfigGroupType)),
			"work_mode":         types.StringValue(strDeref(info.WorkMode)),
		})
		diags.Append(d...)
		elems = append(elems, obj)
	}
	list, d := types.ListValue(elemType, elems)
	diags.Append(d...)
	return list
}

// ownershipVerification nested attr types
var dnsVerificationAttrTypes = map[string]attr.Type{
	"subdomain":    types.StringType,
	"record_type":  types.StringType,
	"record_value": types.StringType,
}
var ownershipVerificationAttrTypes = map[string]attr.Type{
	"dns_verification": types.ListType{ElemType: types.ObjectType{AttrTypes: dnsVerificationAttrTypes}},
}

func flattenOwnershipVerification(z *teo.Zone, diags *diag.Diagnostics) types.List {
	elemType := types.ObjectType{AttrTypes: ownershipVerificationAttrTypes}
	empty := types.ListValueMust(elemType, []attr.Value{})

	// Find ownership verification from zone type-specific detail
	var ov *teo.OwnershipVerification
	if z.CNAMEDetail != nil && z.CNAMEDetail.OwnershipVerification != nil {
		ov = z.CNAMEDetail.OwnershipVerification
	} else if z.NSDetail != nil && z.NSDetail.OwnershipVerification != nil {
		ov = z.NSDetail.OwnershipVerification
	}
	if ov == nil {
		return empty
	}

	// Build dns_verification list
	var dnsVerList types.List
	if ov.DnsVerification != nil {
		dv := ov.DnsVerification
		dnsObj, d := types.ObjectValue(dnsVerificationAttrTypes, map[string]attr.Value{
			"subdomain":    types.StringValue(strDeref(dv.Subdomain)),
			"record_type":  types.StringValue(strDeref(dv.RecordType)),
			"record_value": types.StringValue(strDeref(dv.RecordValue)),
		})
		diags.Append(d...)
		dnsVerList = types.ListValueMust(types.ObjectType{AttrTypes: dnsVerificationAttrTypes}, []attr.Value{dnsObj})
	} else {
		dnsVerList = types.ListValueMust(types.ObjectType{AttrTypes: dnsVerificationAttrTypes}, []attr.Value{})
	}

	ovObj, d := types.ObjectValue(ownershipVerificationAttrTypes, map[string]attr.Value{
		"dns_verification": dnsVerList,
	})
	diags.Append(d...)

	list, d := types.ListValue(elemType, []attr.Value{ovObj})
	diags.Append(d...)
	return list
}

func strDeref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
