package provider

import (
	"context"

	"github.com/Sovforge/Scutum/contrib/terraform/provider/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ resource.Resource = &WebhookResource{}

type WebhookResource struct{ client *client.Client }

func NewWebhookResource() resource.Resource { return &WebhookResource{} }

type webhookModel struct {
	ID      types.String `tfsdk:"id"`
	Name    types.String `tfsdk:"name"`
	URL     types.String `tfsdk:"url"`
	Secret  types.String `tfsdk:"secret"`
	Events  types.List   `tfsdk:"events"`
	Enabled types.Bool   `tfsdk:"enabled"`
}

func (r *WebhookResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_webhook"
}

func (r *WebhookResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Configure a Scutum webhook endpoint.",
		Attributes: map[string]schema.Attribute{
			"id":     schema.StringAttribute{Computed: true, PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}},
			"name":   schema.StringAttribute{Required: true},
			"url":    schema.StringAttribute{Required: true, Description: "HTTPS URL to deliver events to."},
			"secret": schema.StringAttribute{Optional: true, Computed: true, Sensitive: true, Description: "HMAC-SHA256 signing secret."},
			"events": schema.ListAttribute{
				Required:    true,
				ElementType: types.StringType,
				Description: "Event types to subscribe to (e.g. node.enrolled, alert.fired).",
			},
			"enabled": schema.BoolAttribute{
				Optional: true,
				Computed: true,
				Default:  booldefault.StaticBool(true),
			},
		},
	}
}

func (r *WebhookResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *WebhookResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var m webhookModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}
	var events []string
	resp.Diagnostics.Append(m.Events.ElementsAs(ctx, &events, false)...)
	if resp.Diagnostics.HasError() {
		return
	}
	w, err := r.client.CreateWebhook(ctx, client.Webhook{
		Name:    m.Name.ValueString(),
		URL:     m.URL.ValueString(),
		Secret:  m.Secret.ValueString(),
		Events:  events,
		Enabled: m.Enabled.ValueBool(),
	})
	if err != nil {
		resp.Diagnostics.AddError("Create webhook failed", err.Error())
		return
	}
	m.ID = types.StringValue(w.ID)
	m.Secret = types.StringValue(w.Secret)
	resp.Diagnostics.Append(resp.State.Set(ctx, &m)...)
}

func (r *WebhookResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var m webhookModel
	resp.Diagnostics.Append(req.State.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}
	w, err := r.client.GetWebhook(ctx, m.ID.ValueString())
	if err != nil {
		resp.State.RemoveResource(ctx)
		return
	}
	m.Name = types.StringValue(w.Name)
	m.URL = types.StringValue(w.URL)
	m.Secret = types.StringValue(w.Secret)
	m.Enabled = types.BoolValue(w.Enabled)
	list, diag := types.ListValueFrom(ctx, types.StringType, w.Events)
	resp.Diagnostics.Append(diag...)
	m.Events = list
	resp.Diagnostics.Append(resp.State.Set(ctx, &m)...)
}

func (r *WebhookResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var m webhookModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}
	var events []string
	resp.Diagnostics.Append(m.Events.ElementsAs(ctx, &events, false)...)
	if resp.Diagnostics.HasError() {
		return
	}
	w, err := r.client.UpdateWebhook(ctx, m.ID.ValueString(), client.Webhook{
		Name:    m.Name.ValueString(),
		URL:     m.URL.ValueString(),
		Secret:  m.Secret.ValueString(),
		Events:  events,
		Enabled: m.Enabled.ValueBool(),
	})
	if err != nil {
		resp.Diagnostics.AddError("Update webhook failed", err.Error())
		return
	}
	m.Secret = types.StringValue(w.Secret)
	resp.Diagnostics.Append(resp.State.Set(ctx, &m)...)
}

func (r *WebhookResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var m webhookModel
	resp.Diagnostics.Append(req.State.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.DeleteWebhook(ctx, m.ID.ValueString()); err != nil {
		resp.Diagnostics.AddError("Delete webhook failed", err.Error())
	}
}