terraform {
  required_providers {
    scutum = {
      source  = "Sovforge/scutum"
      version = "~> 1.0"
    }
  }
}

# Configure the provider.
# Credentials can also be provided via SCUTUM_ENDPOINT and SCUTUM_API_KEY env vars.
provider "scutum" {
  endpoint = "https://scutum.example.com"
  api_key  = var.scutum_api_key
}

variable "scutum_api_key" {
  description = "Scutum API key with admin privileges"
  sensitive   = true
}

# ── Nodes ─────────────────────────────────────────────────────────────────────

resource "scutum_node" "edge_eu_west" {
  name       = "edge-eu-west-1"
  type       = "remote"
  address    = "10.100.0.10/32"
  public_key = "Jk5Hf3zXqTvP2LmN9sKdWbYcEuRoAiQeGhVwMnBxOp0="
}

resource "scutum_node" "edge_us_east" {
  name       = "edge-us-east-1"
  type       = "remote"
  address    = "10.100.0.11/32"
  public_key = "Rp7Ld2yZwUqA4MnJ8vNcGbXhEiToSfKrOeVmBuCxPj1="
}

# ── Node group ────────────────────────────────────────────────────────────────

resource "scutum_group" "production" {
  name        = "production"
  description = "All production edge nodes"
  node_ids = [
    scutum_node.edge_eu_west.id,
    scutum_node.edge_us_east.id,
  ]
}

# ── Webhook ───────────────────────────────────────────────────────────────────

resource "scutum_webhook" "pagerduty" {
  name   = "PagerDuty alerts"
  url    = "https://events.pagerduty.com/integration/abc123/enqueue"
  secret = var.webhook_secret
  events = [
    "alert.fired",
    "alert.resolved",
    "node.offline",
  ]
}

variable "webhook_secret" {
  description = "HMAC signing secret for the PagerDuty webhook"
  sensitive   = true
}

# ── Federation peer ───────────────────────────────────────────────────────────

resource "scutum_federation_peer" "dr_site" {
  name         = "dr-site"
  hub_url      = "https://scutum-dr.example.com"
  wg_endpoint  = "203.0.113.50:51820"
  wg_public_key = "DrHubPubKey+AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA="
  mesh_cidr    = "10.200.0.0/24"
}

# ── Outputs ───────────────────────────────────────────────────────────────────

output "production_group_id" {
  value = scutum_group.production.id
}

output "eu_west_node_id" {
  value = scutum_node.edge_eu_west.id
}