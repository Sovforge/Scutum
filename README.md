<div align="center">
  <img src="./frontend/public/favicon.png" width="128" height="128" alt="Scutum Logo">

# ⚡ **Scutum**
### *Your infrastructure. Your control. No permission required.*

**Secure. Peer-to-peer. Zero cloud dependencies.**  
*A single binary that shields your infrastructure from vendor lock-in and SaaS control planes.*
</div>

---

> **Scutum replaces fragmented infrastructure tools with a single peer-to-peer control plane for securely connecting and managing infrastructure—without cloud dependencies.**

### Use Scutum if you:

- **Manage fragmented tooling:** You want a single interface for networking, containers, and logs instead of juggling separate dashboards.
- **Operate behind NAT:** You need to connect nodes without exposing ports to the public internet.
- **Require full data sovereignty:** You want to deploy workloads without routing traffic or control data through a third-party SaaS plane.
- **Build at the edge:** You're running air-gapped or high-latency systems that need local autonomy and offline capability.

<details>
<summary><b>📖 Table of Contents</b> (click to expand)</summary>

- [⚡ Quick Start](#the-2-node-quick-start-under-60-seconds)
- [☸️ Kubernetes / Helm](#kubernetes--helm)
- [⚙️ Kubernetes Operator](#kubernetes-operator)
- [🌐 Overview](#overview)
- [🧩 Core Concepts](#core-concepts)
- [🏗️ Architecture](#architecture)
- [🎯 Who is Scutum for?](#who-is-scutum-for)
- [✨ Key Features](#key-features)
- [💎 Scutum vs. The World](#scutum-vs-the-world-why-choose-sovereignty)
- [🚀 Getting Started](#getting-started)
  - [Running Outside Docker (Advanced)](#running-outside-docker-advanced)
- [🛡️ The Scutum Advantage](#the-sovereign-advantage)
- [🔔 Webhook Notifications](#webhook-notifications)
- [🔗 SCIM 2.0 Provisioning](#scim-20-provisioning)
- [📤 Audit Log Forwarding](#audit-log-forwarding)
- [🔌 Plugin System](#plugin-system)
- [📡 API Quick Reference](#api-quick-reference)
- [🗺️ Roadmap](#roadmap)
- [🏢 Enterprise & Commercial Use](#enterprise--commercial-use)
- [🛡️ Supply Chain Security](#supply-chain-security-cra-compliance)
- [💬 Community & Support](#community--support)
- [📈 Node Scaling Philosophy](#node-scaling-philosophy)
- [📜 License](#license)

</details>

---
## ⚡ The 2-Node Quick Start (Under 60 Seconds)

Bridge a VPS and your local machine in under a minute.

### 0. Generate TLS Certificates

Scutum serves its UI and API over HTTPS. Before launching, generate a self-signed certificate for your hub:

```bash
mkdir -p secrets

# CA
openssl genrsa -out secrets/ca.key 4096
openssl req -new -x509 -days 3650 -key secrets/ca.key -out secrets/ca.crt -subj "/CN=scutum-ca"

# Server cert signed by the CA
openssl genrsa -out secrets/server.key 4096
openssl req -new -key secrets/server.key -out secrets/server.csr -subj "/CN=localhost"
openssl x509 -req -days 825 -in secrets/server.csr \
  -CA secrets/ca.crt -CAkey secrets/ca.key -CAcreateserial \
  -extfile <(printf "subjectAltName=DNS:localhost,IP:127.0.0.1") \
  -out secrets/server.crt
rm secrets/server.csr secrets/ca.srl
chmod 600 secrets/*.key
```

> **Tip:** Replace `IP:127.0.0.1` with your VPS's public IP so browsers trust the cert without a warning. For production, use automatic TLS below.

#### Automatic TLS (ACME / Let's Encrypt)

Skip the manual cert generation entirely by setting three environment variables:

```bash
ACME_DOMAIN=scutum.example.com   # your public hostname
ACME_EMAIL=admin@example.com     # used for expiry notifications
# ACME_STAGING=true              # uncomment to use LE staging while testing
```

Scutum will provision and auto-renew a trusted certificate from Let's Encrypt on first start. Port 80 must be reachable from the internet for the HTTP-01 challenge. All plain-HTTP requests are automatically redirected to HTTPS.

---

### 1. Launch the Hub (Public VPS)

```bash
docker run -d \
  --name scutum \
  --network host \
  --cap-add NET_ADMIN \
  --device /dev/net/tun \
  -v scutum_data:/data \
  -v $(pwd)/secrets:/secrets \
  -v /var/run/docker.sock:/var/run/docker.sock:ro \
  --restart unless-stopped \
  -e DATA_DIR=/data \
  -e SECRETS_DIR=/secrets \
  -e PORT=8080 \
  -e CERT_FILE=/secrets/server.crt \
  -e KEY_FILE=/secrets/server.key \
  -e AUDIT_ENABLED=true \
  --health-cmd "curl -fk https://localhost:8080/api/health" \
  --health-interval 30s \
  --health-timeout 5s \
  --health-start-period 10s \
  --health-retries 3 \
  ghcr.io/Sovforge/scutum:latest
```

Open the UI in your browser (accept the self-signed cert warning):

```
https://<your-vps-ip>:8080
```

Complete the setup wizard on first launch.

---

### 2. Start a Remote Node (Your Laptop)

```bash
docker run -d \
  --name scutum-agent \
  --network host \
  --cap-add NET_ADMIN \
  --device /dev/net/tun \
  -v scutum_agent_data:/data \
  -v $(pwd)/secrets:/secrets \
  -v /var/run/docker.sock:/var/run/docker.sock:ro \
  --restart unless-stopped \
  -e DATA_DIR=/data \
  -e SECRETS_DIR=/secrets \
  -e PORT=8081 \
  -e CERT_FILE=/secrets/server.crt \
  -e KEY_FILE=/secrets/server.key \
  ghcr.io/Sovforge/scutum:latest
```

The agent will start. Note its **node address** (`host:port`) from the logs — you'll need it when registering the node in the hub UI.

---

### 3. Register & Approve the Node (UI)

* Open the **Scutum UI** on your VPS
* Navigate to **Nodes** → **Add Node**
* Enter the agent's address as `<laptop-ip>:8081` and approve it

---

### 4. Verify the Mesh

Within minutes, both nodes should appear connected in the UI and begin routing traffic through an encrypted WireGuard mesh.

---

### ✅ What you now have

* A private encrypted network between your machines — no exposed ports, no third-party control plane
* Docker and Kubernetes actions from the hub UI forwarded to any registered node via `X-Target-Node` header

---

---

## ☸️ Kubernetes / Helm

Scutum ships a production-ready Helm chart (`helm/scutum`) that deploys as a **StatefulSet** with two services: a `ClusterIP` for the API/UI and a `LoadBalancer` for the WireGuard UDP port (so remote nodes can reach the mesh hub).

### Prerequisites

- Kubernetes 1.27+
- Helm 3.10+
- A LoadBalancer provider **or** use `service.wireguard.type: NodePort` for bare-metal

### Quickstart

```bash
# 1. Install (self-signed TLS cert generated automatically on first start)
helm install scutum ./helm/scutum \
  --namespace scutum --create-namespace

# 2. Get the API URL
kubectl get svc scutum -n scutum

# 3. Get the WireGuard endpoint (share with remote nodes)
kubectl get svc scutum-wireguard -n scutum
```

Open `https://<LoadBalancer-IP>:8080` and complete the setup wizard.

### Common value overrides

```yaml
# values-production.yaml

# Use an existing PostgreSQL database (required for replica count > 1)
database:
  url: "postgres://scutum:password@postgres:5432/scutum"

replicaCount: 3

# Attach to a Gateway API gateway instead of using direct LoadBalancer
gateway:
  enabled: true
  name: prod-gateway
  hostnames:
    - scutum.example.com

# Enable Docker socket mount on nodes that run Docker (not containerd)
docker:
  enabled: true

# Bring your own TLS certificate (e.g. from cert-manager)
tls:
  autoGenerate: false
  existingSecret: scutum-tls  # kubernetes.io/tls secret
```

```bash
helm upgrade --install scutum ./helm/scutum \
  --namespace scutum --create-namespace \
  -f values-production.yaml
```

### Notes

| Concern | Detail |
|---|---|
| **WireGuard** | Requires `NET_ADMIN` capability and the kernel WireGuard module (built-in since kernel 5.6). The chart adds these automatically. |
| **HA (replicas > 1)** | Requires an external database (`database.url`). SQLite is single-writer only. |
| **Docker features** | Disabled by default. Set `docker.enabled: true` only on nodes where Docker (not containerd) is the runtime — the socket is mounted as a `hostPath`. |
| **NAT roaming** | Edge nodes (e.g. laptops) re-register their WireGuard endpoint with the hub every 2 minutes, so the tunnel recovers automatically after a network change without a restart. |
| **Hub-and-spoke routing** | For two edge nodes behind different NATs to reach each other, configure their WireGuard `AllowedIPs` to include the full mesh CIDR — traffic is relayed through the hub. |

### Running the tests

```bash
# Helm render tests (no cluster required)
./helm/scutum/tests/render_test.sh

# Full integration test using kind
./scripts/test-k8s.sh

# Keep the cluster after the test for inspection
./scripts/test-k8s.sh --keep
```

---

## ⚙️ Kubernetes Operator

The Scutum operator manages hub and edge deployments as first-class Kubernetes resources using two CRDs: `ScutumHub` and `ScutumNode`. The hub controller reconciles StatefulSets, Services, and RBAC; the node controller auto-enrolls edges into the mesh via the hub API and writes a per-node `bootstrap` Secret containing WireGuard config and HMAC credentials.

### Install the CRDs and RBAC

```bash
kubectl apply -f operator/config/crd/
kubectl apply -f operator/config/rbac/
```

### Deploy a hub

```yaml
apiVersion: scutum.io/v1alpha1
kind: ScutumHub
metadata:
  name: hub
  namespace: scutum
spec:
  image:
    repository: ghcr.io/sovforge/scutum
    tag: latest
  adminSecret: scutum-admin-creds   # Secret with keys: username, password
  storage:
    size: 5Gi
  wireGuard:
    port: 51820
```

### Enroll an edge node

```yaml
apiVersion: scutum.io/v1alpha1
kind: ScutumNode
metadata:
  name: edge-london
  namespace: scutum
spec:
  hubRef:
    name: hub
    namespace: scutum
  nodeName: edge-london
  nodeType: remote
  image:
    repository: ghcr.io/sovforge/scutum
    tag: latest
```

The operator reads hub credentials from the `ScutumHub`'s `adminSecret`, calls `GET /api/operator/bootstrap` to fetch mesh parameters, creates the node via the enrollment API, and writes a `<name>-bootstrap` Secret the edge pod mounts at startup.

### Status fields

Once reconciled, both resources expose status:

```bash
kubectl get scutumhub hub -n scutum -o wide
kubectl get scutumnode edge-london -n scutum -o wide
# PHASE column shows: Pending → Provisioning → Ready
```

---

## 🌐 Overview

Scutum is a decentralized mesh controller designed to form a fully encrypted P2P mesh across your infrastructure. By eliminating SaaS control planes, relays, and telemetry, Scutum ensures that your network management remains private, secure, and entirely under your control.

It is a single statically-linked binary designed for engineers who want to manage their infrastructure, not the tools that manage it.

## 🧩 Core Concepts

- **Hub**: A node with a public IP that coordinates P2P introductions and NAT traversal. Any $5/mo VPS works — the Hub signals peers but does not relay traffic once the mesh is up.
- **Remote**: A node (behind NAT/Firewall) that establishes outbound-only connections to a Hub.
- **Mesh**: The WireGuard-based encrypted overlay connecting all nodes.
- **Store**: The encrypted state engine (SQLite/Postgres) residing within your perimeter.
- **Agent**: The background process on each node that reconciles workloads.
- **Enrollment**: Nodes are admitted by **manual approval only**. A Remote generates a node ID on first start; an admin approves it via the UI before any mesh keys are issued. No node is ever auto-trusted.

## 🏗️ Architecture

Scutum turns any set of nodes into an encrypted mesh. Unlike traditional VPNs, there is no central relay—data and control flow directly between peers.

**The Hub Requirement:** To facilitate P2P connections without relying on third-party STUN/TURN services, at least one node (the **Hub**) must have a **publicly accessible IP**. The Hub acts as the signaling point to allow peers behind CGNAT or strict firewalls to find each other. Once connected, traffic is strictly P2P.

*Multiple Hubs can be designated for High-Availability (HA) mesh coordination.*

```mermaid
graph TD
    subgraph internet["Internet"]
        HUB["Hub Node\n(Public IP - Signaling)"]
    end

    subgraph siteA["Site A (NAT/CGNAT)"]
        RA["Remote A"]
    end

    subgraph siteB["Site B (NAT/CGNAT)"]
        RB["Remote B"]
    end

    UI["Control UI"]

    UI -- "1. Enroll & approve" --> HUB
    RA -- "2. Outbound handshake" --> HUB
    RB -- "2. Outbound handshake" --> HUB
    HUB -. "3. Exchange WireGuard keys" .-> RA
    HUB -. "3. Exchange WireGuard keys" .-> RB
    RA == "4. Direct P2P - WireGuard - E2E Encrypted" ==> RB
```

> The Hub brokers the initial handshake only. Once peers have exchanged keys, all data flows directly between them — the Hub is not in the data path.

## 🎯 Who is Scutum for?

- **Security-Conscious Orgs:** Need to meet strict compliance (CRA, HIPAA) without third-party data transit.
- **Infrastructure Engineers:** Tired of SaaS vendor lock-in and "black box" control planes.
- **Edge & IoT Operators:** Need a mesh that functions in air-gapped or high-latency environments.
- **Consolidated Operators:** Looking for a **"Single Pane of Glass"** experience to manage networking, workloads, and logs without jumping between multiple vendor dashboards.

## ✨ Key Features

- **Multi-Runtime Orchestration**: Native support for managing **Docker** containers and **Kubernetes** pods across the mesh from a single interface.
- **Integrated Management UI**: No separate dashboard or web server to install; the management interface is built directly into the core binary.
- **Secure P2P Mesh**: Uses WireGuard to establish encrypted, high-performance tunnels between all nodes.
- **Zero-Trust Enrollment**: Security is enforced through **explicit manual enrollment**. Nodes are never auto-discovered or auto-trusted.
- **Enterprise Key Management (KMS)**: Secure master keys via HashiCorp Vault, AWS KMS, GCP, or Azure.
- **Emergency Recovery**: Uses **Shamir's Secret Sharing (SSS)** for local KMS to split master keys into multiple recovery shares.
- **Database Agnostic**: Supports encrypted **SQLite** for the edge, plus **PostgreSQL** and **MySQL** for HA clusters.
- **Forensic Observability**: Native OpenTelemetry integration for live logs, metrics, and distributed P2P traces.
- **Live Terminal**: Web-based `exec` access to both **Docker containers** and **Kubernetes pods**, routed directly through the mesh.

---

## 💎 Scutum vs. The World: Why Self-Host Your Control Plane?

Tailscale and Headscale are fantastic for secure network overlays. However, they are primarily network-focused. You still need separate tools for deploying applications, managing state, and collecting telemetry. **Scutum is an orchestration-first platform.** It integrates networking, workload deployment, state management, and observability into a true **"Single Pane of Glass"** experience, all within your control.

### vs. Nomad / Kubernetes (Orchestration)
Kubernetes is the industry standard for cloud-native orchestration but comes with significant operational overhead (ETCD, complex control plane networking, Ingress controllers). Nomad offers simplicity but still requires external components for networking and state. **Scutum is a single binary** that provides GitOps-driven OCI and Kubernetes deployment, making it ideal for edge, air-gapped, or sovereign infrastructure where simplicity and self-containment are paramount.

Manual VPN configurations are secure but quickly become an operational nightmare at scale. Managing peer keys, IP addresses, and routing tables for dozens or hundreds of nodes is time-consuming and error-prone. **Scutum automates the P2P overlay** with built-in key management and peer configuration, while strictly enforcing manual enrollment for security. This gives you the security of a manually configured setup with the agility and scalability of a modern mesh controller.

### vs. Cloud-Managed Orchestration (e.g., AWS ECS, Azure Container Apps)
Cloud-managed services offer convenience but introduce vendor lock-in, reliance on SaaS control planes, and often opaque telemetry. Your data and control plane reside on someone else's infrastructure. **Scutum brings the entire control plane within your perimeter.** It eliminates all cloud dependencies, ensuring full data sovereignty, air-gap capability, and complete control over your infrastructure's operational data.

## 🚀 Getting Started

### Prerequisites
- **Docker**: The only requirement for deployment.
- **Public IP**: At least one node (the Hub) requires a public-facing IP to coordinate the mesh. Any minimal VPS ($5/mo DigitalOcean, Hetzner CX11, etc.) is sufficient — the Hub signals peers but does not relay data traffic.
- **Kernel Module (Optional)**: For maximum performance, ensure `wireguard` is loaded on the host. If missing, the container automatically installs and configures `wireguard-go` as a fallback.

### Multi-Runtime Workload Deployment
Scutum supports deploying declarative stacks using a **GitOps workflow**. Simply point the control plane to your repository, and the agents will reconcile the state of your **Docker** (Compose-style) and **Kubernetes** (Pod/Manifest) workloads directly through the mesh.

---

### Running Outside Docker (Advanced)

> ⚠️ **This is not the recommended way to run Scutum.** Docker handles all system dependencies, privilege requirements, and networking automatically. Running the binary directly is intended for development or environments where Docker is not available. **Several features may not work** depending on your system configuration.

#### What may not work

| Feature | Requirement | Notes |
| :--- | :--- | :--- |
| **WireGuard mesh** | `wireguard` kernel module or `wireguard-go` installed + root / `CAP_NET_ADMIN` | Without elevated privileges the interface cannot be created |
| **iptables / routing** | Root or `CAP_NET_ADMIN` | Required for mesh traffic to route correctly |
| **Docker management** | `/var/run/docker.sock` accessible to the process | Run as root or add your user to the `docker` group |
| **Kubernetes management** | Valid `~/.kube/config` | Must point to a reachable cluster |
| **WireGuard auto-install** | Root + internet access | Falls back gracefully if it cannot install |

#### Prerequisites

- Go 1.25+ and Node 20+ (to build from source)
- `iproute2` and `wireguard-tools` installed on the host
- Root or `CAP_NET_ADMIN` capability if you want the mesh to function

#### Build and run

```bash
# 1. Build the frontend and embed it into the binary
cd frontend
npm install
npm run generate
cp -r .output/public/. ../cmd/api/dist/
cd ..

# 2. Build the Go binary (CGO_ENABLED=0 produces a fully static binary)
CGO_ENABLED=0 go build -ldflags="-s -w" -o scutum ./cmd/api

# 3. Run (root required for WireGuard)
sudo ./scutum
```

The UI will be available at `http://localhost:8080`. On first launch you will be taken through the setup wizard.

Environment variables:

| Variable | Default | Description |
| :--- | :--- | :--- |
| `PORT` | `8080` | HTTP listen port |
| `DATA_DIR` | `./data` | Directory for the SQLite database and persistent state |
| `SECRETS_DIR` | `./secrets` | Directory for TLS keys and KMS configuration |
| `DATABASE_URL` | *(unset — uses SQLite)* | Postgres (`postgres://...`) or MySQL (`mysql://...`) DSN |
| `LOG_LEVEL` | `info` | Log verbosity: `debug`, `info`, `warn`, or `error` |
| `CERT_FILE` | `$SECRETS_DIR/server.crt` | Path to the TLS certificate |
| `KEY_FILE` | `$SECRETS_DIR/server.key` | Path to the TLS private key |
| `CA_CERT_FILE` | *(unset)* | Path to a CA certificate to enable mTLS client verification |
| `ACME_DOMAIN` | *(unset)* | Enable Let's Encrypt auto-TLS for this domain (disables manual cert) |
| `ACME_EMAIL` | *(unset)* | Contact email for Let's Encrypt expiry notifications |
| `ACME_STAGING` | `false` | Use Let's Encrypt staging environment |
| `ACME_CACHE_DIR` | `$SECRETS_DIR/acme` | Directory for cached ACME certificates |
| `AUDIT_ENABLED` | `false` | Set to `true` to enable the security audit log |
| `AUDIT_RETENTION_DAYS` | `365` | Days to retain audit log entries (CRA recommends ≥ 1 year) |
| `HEALER_INTERVAL` | `30s` | How often the mesh healer reconciles peer state (Go duration string) |
| `PLUGINS_DIR` | `/app/plugins` | Directory where uploaded WASM plugins are stored |

---

## 🛡️ The Scutum Advantage

| Feature | SaaS / VPN Vendors | **Scutum** |
| :--- | :--- | :--- |
| **Control Plane** | Managed by Vendor (SaaS) | **Owned by You (Local)** |
| **Networking** | Public Relays / Exit Nodes | **Private P2P WireGuard Mesh** |
| **Observability** | Third-party Ingestion | **Local OpenTelemetry Streams** |
| **Dependency** | Internet Required | **Fully Air-Gap Capable** |

---

## 🔔 Webhook Notifications

Scutum can POST a signed JSON payload to any HTTP endpoint when key mesh events occur.

### Supported events

| Event | Trigger |
|---|---|
| `node.enrolled` | A new node is approved and added to the mesh |
| `node.offline` | The healer marks a node as unreachable |
| `node.online` | A previously offline node recovers |
| `healer.service_restart` | The healer restarts a failing service |
| `audit.critical` | A critical-severity audit event is logged |
| `user.created` | A new user account is created |
| `auth.sso_login` | A user authenticates via SSO |

### Payload format

```json
{
  "type": "node.enrolled",
  "timestamp": "2026-05-29T12:00:00Z",
  "payload": { "node_id": "...", "name": "edge-london" }
}
```

### Signature verification

Every delivery includes `X-Scutum-Signature: sha256=<hex>` computed as HMAC-SHA256 of the raw body using the webhook secret. Verify it on your receiver to confirm authenticity.

### Managing webhooks

```bash
# Create a webhook (subscribe to all node events)
curl -X POST /api/webhooks \
  -H "Authorization: Bearer <token>" \
  -d '{"name":"Slack","url":"https://hooks.slack.com/...","secret":"s3cr3t","events":["node.enrolled","node.offline"]}'

# Test delivery immediately
curl -X POST /api/webhooks/<id>/test -H "Authorization: Bearer <token>"
```

---

## 🔗 SCIM 2.0 Provisioning

Scutum implements SCIM 2.0 (RFC 7644) at `/scim/v2/`, enabling automatic user provisioning and deprovisioning from any compatible IdP (Microsoft Entra ID, Okta, JumpCloud).

### Generate a SCIM token

```bash
curl -X POST /api/scim/tokens \
  -H "Authorization: Bearer <admin-jwt>" \
  -d '{"description":"Entra ID provisioning"}'
# Returns: {"id":"...","token":"<raw-token>"}  ← copy the token, it won't be shown again
```

### Configure in Microsoft Entra ID

1. Enterprise Applications → your app → **Provisioning** → **Automatic**
2. **Tenant URL**: `https://scutum.example.com/scim/v2`
3. **Secret token**: the token returned above
4. Save and click **Test Connection**

### Supported operations

| Operation | Behaviour |
|---|---|
| Create user | New account created; random password set (SSO login recommended) |
| Update user | Username and email updated |
| Deactivate (`active: false`) | Account disabled — login rejected |
| Delete user | Account permanently removed |

---

## 📤 Audit Log Forwarding

Forward audit log entries to an external SIEM or log aggregator every 30 seconds.

### Supported formats

| Format | Use case |
|---|---|
| `json` | Generic — works with Elastic, Loki, Splunk HEC, any HTTP receiver |
| `cef` | ArcSight CEF — compatible with IBM QRadar, HP ArcSight |

### Configure a forwarder

```bash
curl -X POST /api/audit/forwarders \
  -H "Authorization: Bearer <admin-jwt>" \
  -d '{"name":"Elastic","url":"https://elastic.example.com/scutum-audit","format":"json"}'
```

Scutum will POST a batch of up to 500 recent audit entries to the URL every 30 seconds. Toggle forwarders on/off with `PUT /api/audit/forwarders/{id}` setting `"enabled": false`.

---

## 🔌 Plugin System

Scutum keeps the core focused and predictable. Features that are highly environment-specific or niche are handled through the plugin system—so the core stays simple, and you stay flexible.

Scutum includes a secure, capability-scoped **WebAssembly (WASM)** plugin sandbox. Extend the control plane with custom logic, HTTP routes, or automation using TinyGo, Rust, or AssemblyScript.
👉 [Read the Plugin Guide](PluginGuide.md)

---

## 📡 API Quick Reference

All endpoints are served under `/api/`. Requests to authenticated routes require a `Bearer` token or API key in the `Authorization` header.

| Method | Endpoint | Auth | Description |
|--------|----------|------|-------------|
| `GET` | `/health` | — | System heartbeat and uptime |
| `POST` | `/auth/login` | — | Obtain a JWT session token |
| `GET` | `/auth/me` | ✓ | Current user profile |
| `GET` | `/nodes` | ✓ | List registered mesh nodes |
| `POST` | `/nodes` | ✓ admin | Enroll a new node |
| `POST` | `/network/peer` | ✓ | Add a WireGuard peer to the mesh |
| `GET` | `/network/status` | ✓ | WireGuard interface and peer status |
| `GET` | `/network/mesh-summary` | ✓ | Topology summary for the UI graph |
| `GET` | `/docker/containers` | ✓ | List containers on the target node |
| `POST` | `/docker/deploy` | ✓ | Deploy a container image |
| `POST` | `/docker/deploy-compose` | ✓ | Deploy a Compose stack |
| `GET` | `/docker/containers/{id}/terminal` | ✓ | Open a WebSocket terminal into a container |
| `GET` | `/kubernetes/summary` | ✓ | Cluster resource summary |
| `GET` | `/kubernetes/pods` | ✓ | List pods across all namespaces |
| `GET` | `/k8s/{namespace}/{pod}/terminal` | ✓ | Open a WebSocket terminal into a pod |
| `GET` | `/observability/logs` | ✓ admin | Structured application logs |
| `GET` | `/audit/logs` | ✓ admin | CRA-compliant security audit trail |
| `GET` | `/audit/logs/export` | ✓ admin | Export audit logs as CSV/JSON |
| `POST` | `/plugins/upload` | ✓ admin | Upload and load a WASM plugin |
| `GET` | `/plugins` | ✓ | List loaded plugins |

> Routes that accept an `X-Target-Node` header will proxy the request to the specified mesh node.

---

## 🗺️ Roadmap

| Feature | Status |
| :--- | :--- |
| **Single Sign-On (OIDC / SAML)** — Keycloak, Okta, GitHub, Azure AD | 🔜 Planned |
| **Helm chart** — first-class Kubernetes deployment | ✅ Shipped |
| **Kubernetes operator** — CRD-based cluster management | ✅ Shipped |

Have a feature request? Open an issue or start a discussion.

---

## 🏢 Enterprise & Commercial Use

Scutum is designed to be a sustainable project. It is **free for personal use, academic institutions, and small businesses** that meet the following criteria:
- **Small Businesses**: Fewer than 100 total employees/contractors and less than $1,000,000 USD total revenue in the prior tax year.
- **Academic Institutions**: Accredited universities and non-profit research organizations are granted full usage rights for educational and research purposes.

**Enterprise Use:** If your organization exceeds these thresholds or requires an enterprise-grade SLA, please contact the maintainers to discuss a commercial license.

---

## 🛡️ Supply Chain Security (CRA Compliance)

In accordance with the **EU Cyber Resilience Act (CRA)**, Scutum provides full transparency into its software supply chain. Every release includes a cryptographically verifiable **Software Bill of Materials (SBOM)** to ensure components are auditable and secure.

### SBOM Validation
You can extract and audit the components within the Scutum image using Syft:

```bash
# Generate a human-readable list of dependencies
syft ghcr.io/Sovforge/scutum:latest

# Export SBOM in CycloneDX format for compliance tools
syft ghcr.io/Sovforge/scutum:latest -o cyclonedx-json > scutum-sbom.json
```

Official images are also scanned for vulnerabilities. You can perform a local scan using `grype`:
```bash
grype ghcr.io/Sovforge/scutum:latest
```

---

## 💬 Community & Support

Scutum is a solo-authored project — all core code is written and maintained by a single developer. There is no contributor team, and pull requests adding features to the core are not accepted at this time.

That said, your input genuinely shapes the roadmap. If there is something you want to see:

- **Feature requests** — Open a GitHub Issue with the `feature request` label. Describe the problem you are solving, not just the solution.
- **Bug reports** — Open a GitHub Issue with steps to reproduce and your environment details.
- **Questions** — Use GitHub Discussions for anything that is not a bug or feature request.

> Scutum is built for the long term. Keeping the core tightly controlled ensures it stays focused, auditable, and secure.

---

## 📈 Node Scaling Philosophy
Scutum does **not** impose artificial node limits—free or commercial. Real-world limits are determined by your hardware capacity and mesh topology complexity. We prefer honesty over marketing slogans.

<div align="center">

### ✨ *Your infrastructure, armored.*

</div>

## 📋 Changelog

For a full list of additions, changes, and fixes across all releases, see [CHANGELOG.md](CHANGELOG.md).

## 📜 License
This project is licensed under the PolyForm Small Business License 1.0.0.

*   **Small Business & Academic Use:** Free for qualifying small businesses and accredited academic institutions.
*   **Enterprise Use:** Requires a separate commercial agreement.

Please see the [LICENSE](LICENSE.md) file for the full legal text.

---
*Scutum — the shield between your infrastructure and everyone else's control plane.*