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

var _ resource.Resource = &NodeResource{}

type NodeResource struct{ client *client.Client }

func NewNodeResource() resource.Resource { return &NodeResource{} }

type nodeModel struct {
	ID        types.String `tfsdk:"id"`
	Name      types.String `tfsdk:"name"`
	Type      types.String `tfsdk:"type"`
	Address   types.String `tfsdk:"address"`
	PublicKey types.String `tfsdk:"public_key"`
}

func (r *NodeResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_node"
}

func (r *NodeResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Register a Scutum node (hub or remote).",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:    true,
				Description: "Node UUID assigned by Scutum.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"name": schema.StringAttribute{Required: true, Description: "Unique node name."},
			"type": schema.StringAttribute{Required: true, Description: "Node type: hub or remote."},
			"address": schema.StringAttribute{Required: true, Description: "WireGuard mesh IP or CIDR for this node."},
			"public_key": schema.StringAttribute{Required: true, Description: "WireGuard public key (base64)."},
		},
	}
}

func (r *NodeResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	c, ok := req.ProviderData.(*client.Client)
	if !ok {
		resp.Diagnostics.AddError("Unexpected provider data type", "Expected *client.Client")
		return
	}
	r.client = c
}

func (r *NodeResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var m nodeModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}
	n, err := r.client.CreateNode(ctx, client.Node{
		Name:      m.Name.ValueString(),
		Type:      m.Type.ValueString(),
		Address:   m.Address.ValueString(),
		PublicKey: m.PublicKey.ValueString(),
	})
	if err != nil {
		resp.Diagnostics.AddError("Create node failed", err.Error())
		return
	}
	m.ID = types.StringValue(n.ID)
	resp.Diagnostics.Append(resp.State.Set(ctx, &m)...)
}

func (r *NodeResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var m nodeModel
	resp.Diagnostics.Append(req.State.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}
	n, err := r.client.GetNode(ctx, m.ID.ValueString())
	if err != nil {
		resp.State.RemoveResource(ctx)
		return
	}
	m.Name = types.StringValue(n.Name)
	m.Type = types.StringValue(n.Type)
	m.Address = types.StringValue(n.Address)
	m.PublicKey = types.StringValue(n.PublicKey)
	resp.Diagnostics.Append(resp.State.Set(ctx, &m)...)
}

func (r *NodeResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	// Nodes are immutable — changes force replacement via plan modifiers if needed.
	resp.Diagnostics.AddError("Node update not supported", "Delete and recreate the node to change its properties.")
}

func (r *NodeResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var m nodeModel
	resp.Diagnostics.Append(req.State.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.DeleteNode(ctx, m.ID.ValueString()); err != nil {
		resp.Diagnostics.AddError("Delete node failed", err.Error())
	}
}