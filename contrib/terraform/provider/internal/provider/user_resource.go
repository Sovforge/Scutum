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

var _ resource.Resource = &UserResource{}

type UserResource struct{ client *client.Client }

func NewUserResource() resource.Resource { return &UserResource{} }

type userModel struct {
	ID       types.String `tfsdk:"id"`
	Username types.String `tfsdk:"username"`
	Email    types.String `tfsdk:"email"`
	Password types.String `tfsdk:"password"`
	RoleID   types.String `tfsdk:"role_id"`
}

func (r *UserResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_user"
}

func (r *UserResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manage a Scutum user account and role assignment.",
		Attributes: map[string]schema.Attribute{
			"id":       schema.StringAttribute{Computed: true, PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}},
			"username": schema.StringAttribute{Required: true},
			"email":    schema.StringAttribute{Optional: true, Computed: true},
			"password": schema.StringAttribute{Required: true, Sensitive: true, Description: "Initial password — changes after creation are ignored (use the UI to rotate)."},
			"role_id":  schema.StringAttribute{Optional: true, Computed: true, Description: "Role UUID to assign to this user."},
		},
	}
}

func (r *UserResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *UserResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var m userModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}
	u, err := r.client.CreateUser(ctx, client.User{
		Username: m.Username.ValueString(),
		Email:    m.Email.ValueString(),
		Password: m.Password.ValueString(),
		RoleID:   m.RoleID.ValueString(),
	})
	if err != nil {
		resp.Diagnostics.AddError("Create user failed", err.Error())
		return
	}
	m.ID = types.StringValue(u.ID)
	m.Email = types.StringValue(u.Email)
	resp.Diagnostics.Append(resp.State.Set(ctx, &m)...)
}

func (r *UserResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var m userModel
	resp.Diagnostics.Append(req.State.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}
	u, err := r.client.GetUser(ctx, m.ID.ValueString())
	if err != nil {
		resp.State.RemoveResource(ctx)
		return
	}
	m.Username = types.StringValue(u.Username)
	m.Email = types.StringValue(u.Email)
	// Password and role_id are write-only / not returned by the API; preserve state.
	resp.Diagnostics.Append(resp.State.Set(ctx, &m)...)
}

func (r *UserResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var m userModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}
	u, err := r.client.UpdateUser(ctx, m.ID.ValueString(), client.User{
		Username: m.Username.ValueString(),
		Email:    m.Email.ValueString(),
		RoleID:   m.RoleID.ValueString(),
	})
	if err != nil {
		resp.Diagnostics.AddError("Update user failed", err.Error())
		return
	}
	m.Email = types.StringValue(u.Email)
	resp.Diagnostics.Append(resp.State.Set(ctx, &m)...)
}

func (r *UserResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var m userModel
	resp.Diagnostics.Append(req.State.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.DeleteUser(ctx, m.ID.ValueString()); err != nil {
		resp.Diagnostics.AddError("Delete user failed", err.Error())
	}
}