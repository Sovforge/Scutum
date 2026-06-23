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

var _ resource.Resource = &FederationPeerResource{}

type FederationPeerResource struct{ client *client.Client }

func NewFederationPeerResource() resource.Resource { return &FederationPeerResource{} }

type federationPeerModel struct {
	ID          types.String `tfsdk:"id"`
	Name        types.String `tfsdk:"name"`
	HubURL      types.String `tfsdk:"hub_url"`
	WGEndpoint  types.String `tfsdk:"wg_endpoint"`
	WGPublicKey types.String `tfsdk:"wg_public_key"`
	MeshCIDR    types.String `tfsdk:"mesh_cidr"`
	AllowedIPs  types.String `tfsdk:"allowed_ips"`
}

func (r *FederationPeerResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_federation_peer"
}

func (r *FederationPeerResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Link a federated Scutum hub so that the two WireGuard meshes can route to each other.",
		Attributes: map[string]schema.Attribute{
			"id":           schema.StringAttribute{Computed: true, PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}},
			"name":         schema.StringAttribute{Required: true, Description: "Human-readable peer name."},
			"hub_url":      schema.StringAttribute{Required: true, Description: "API URL of the remote Scutum hub."},
			"wg_endpoint":  schema.StringAttribute{Required: true, Description: "WireGuard UDP endpoint of the remote hub (host:port)."},
			"wg_public_key": schema.StringAttribute{Required: true, Description: "WireGuard public key of the remote hub."},
			"mesh_cidr":    schema.StringAttribute{Required: true, Description: "CIDR block of the remote hub's mesh."},
			"allowed_ips":  schema.StringAttribute{Optional: true, Computed: true, Description: "Extra CIDRs to route via this peer (comma-separated)."},
		},
	}
}

func (r *FederationPeerResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *FederationPeerResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var m federationPeerModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}
	p, err := r.client.CreateFederationPeer(ctx, client.FederationPeer{
		Name:        m.Name.ValueString(),
		HubURL:      m.HubURL.ValueString(),
		WGEndpoint:  m.WGEndpoint.ValueString(),
		WGPublicKey: m.WGPublicKey.ValueString(),
		MeshCIDR:    m.MeshCIDR.ValueString(),
		AllowedIPs:  m.AllowedIPs.ValueString(),
	})
	if err != nil {
		resp.Diagnostics.AddError("Create federation peer failed", err.Error())
		return
	}
	m.ID = types.StringValue(p.ID)
	m.AllowedIPs = types.StringValue(p.AllowedIPs)
	resp.Diagnostics.Append(resp.State.Set(ctx, &m)...)
}

func (r *FederationPeerResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var m federationPeerModel
	resp.Diagnostics.Append(req.State.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}
	p, err := r.client.GetFederationPeer(ctx, m.ID.ValueString())
	if err != nil {
		resp.State.RemoveResource(ctx)
		return
	}
	m.Name = types.StringValue(p.Name)
	m.HubURL = types.StringValue(p.HubURL)
	m.WGEndpoint = types.StringValue(p.WGEndpoint)
	m.WGPublicKey = types.StringValue(p.WGPublicKey)
	m.MeshCIDR = types.StringValue(p.MeshCIDR)
	m.AllowedIPs = types.StringValue(p.AllowedIPs)
	resp.Diagnostics.Append(resp.State.Set(ctx, &m)...)
}

func (r *FederationPeerResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	resp.Diagnostics.AddError("Federation peer update not supported", "Delete and recreate the peer to change its properties.")
}

func (r *FederationPeerResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var m federationPeerModel
	resp.Diagnostics.Append(req.State.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.DeleteFederationPeer(ctx, m.ID.ValueString()); err != nil {
		resp.Diagnostics.AddError("Delete federation peer failed", err.Error())
	}
}