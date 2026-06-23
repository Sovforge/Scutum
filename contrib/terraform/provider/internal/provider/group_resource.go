package provider

import (
	"context"

	"github.com/Sovforge/Scutum/contrib/terraform/provider/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ resource.Resource = &GroupResource{}

type GroupResource struct{ client *client.Client }

func NewGroupResource() resource.Resource { return &GroupResource{} }

type groupModel struct {
	ID          types.String `tfsdk:"id"`
	Name        types.String `tfsdk:"name"`
	Description types.String `tfsdk:"description"`
	NodeIDs     types.List   `tfsdk:"node_ids"`
}

func (r *GroupResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_group"
}

func (r *GroupResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manage a Scutum node group and its membership.",
		Attributes: map[string]schema.Attribute{
			"id":          schema.StringAttribute{Computed: true, PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}},
			"name":        schema.StringAttribute{Required: true},
			"description": schema.StringAttribute{Optional: true, Computed: true},
			"node_ids": schema.ListAttribute{
				Optional:    true,
				Computed:    true,
				ElementType: types.StringType,
				Description: "List of node IDs that are members of this group.",
			},
		},
	}
}

func (r *GroupResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	c, ok := req.ProviderData.(*client.Client)
	if !ok {
		resp.Diagnostics.AddError("Unexpected provider data", "Expected *client.Client")
		return
	}
	r.client = c
}

func (r *GroupResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var m groupModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}
	g, err := r.client.CreateGroup(ctx, client.Group{
		Name:        m.Name.ValueString(),
		Description: m.Description.ValueString(),
	})
	if err != nil {
		resp.Diagnostics.AddError("Create group failed", err.Error())
		return
	}
	m.ID = types.StringValue(g.ID)
	if !m.NodeIDs.IsNull() {
		var ids []string
		resp.Diagnostics.Append(m.NodeIDs.ElementsAs(ctx, &ids, false)...)
		if !resp.Diagnostics.HasError() && len(ids) > 0 {
			if err := r.client.SetGroupMembers(ctx, g.ID, ids); err != nil {
				resp.Diagnostics.AddError("Set group members failed", err.Error())
				return
			}
		}
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &m)...)
}

func (r *GroupResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var m groupModel
	resp.Diagnostics.Append(req.State.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}
	g, err := r.client.GetGroup(ctx, m.ID.ValueString())
	if err != nil {
		resp.State.RemoveResource(ctx)
		return
	}
	m.Name = types.StringValue(g.Name)
	m.Description = types.StringValue(g.Description)

	nodes, err := r.client.GetGroupNodes(ctx, m.ID.ValueString())
	if err == nil {
		ids := make([]string, len(nodes))
		for i, n := range nodes {
			ids[i] = n.ID
		}
		list, diag := types.ListValueFrom(ctx, types.StringType, ids)
		resp.Diagnostics.Append(diag...)
		m.NodeIDs = list
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &m)...)
}

func (r *GroupResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var m groupModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}
	var ids []string
	if !m.NodeIDs.IsNull() {
		resp.Diagnostics.Append(m.NodeIDs.ElementsAs(ctx, &ids, false)...)
	}
	if err := r.client.SetGroupMembers(ctx, m.ID.ValueString(), ids); err != nil {
		resp.Diagnostics.AddError("Update group members failed", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &m)...)
}

func (r *GroupResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var m groupModel
	resp.Diagnostics.Append(req.State.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.DeleteGroup(ctx, m.ID.ValueString()); err != nil {
		resp.Diagnostics.AddError("Delete group failed", err.Error())
	}
}