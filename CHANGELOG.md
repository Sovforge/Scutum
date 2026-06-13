# Changelog

All notable changes to Scutum are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).

---

## [Unreleased]

### Added
- **Enterprise licensing (open-core)**: Ed25519-signed JWT licenses gate enterprise features while leaving the open-core fully functional without any license. The public key is injected at build time so self-built binaries cannot verify licenses and enterprise features remain disabled. Set `SCUTUM_LICENSE=/path/to/license.jwt` to activate. Enterprise-only features: SSO/OIDC, SCIM 2.0 provisioning, hub federation, and the CRA compliance report. Unlicensed requests to these endpoints receive `402 Payment Required` with a JSON body identifying the required feature.
- **IronKey / USB share export**: Emergency Recovery Key shares can now be saved directly to a USB drive (e.g. IronKey) via the File System Access API (`showSaveFilePicker`). Each share gets its own `.erk` file. Falls back to `<a download>` for browsers without File System Access API support (Firefox, Safari). USB import (`showOpenFilePicker`) is also supported when loading shares for reissue or recovery. See `/settings/recovery`.
- **Recovery Keys settings page** (`/settings/recovery`): Admin page for generating a fresh set of ERK shares from the current master key and reissuing existing shares when a share holder leaves or a share is compromised. Per-share USB export and USB-load-from-file for input shares.
- **Node groups UI**: Nodes page now includes a Node Groups section. Groups can be created, deleted, and expanded inline to add or remove member nodes via a dropdown selector.
- **Settings sidebar for sub-pages**: Federation, Webhooks, SCIM, and Recovery Keys pages now render with the shared settings navigation sidebar (`SettingsShell` component), consistent with the rest of the settings area.
- **Hub federation**: Link two independent Scutum instances so their WireGuard meshes can route to each other. Adding a federation peer (`POST /api/federation/peers`) registers the remote hub's public key and mesh CIDR as a WireGuard peer with persistent keepalive; removing it tears the tunnel down. Status is tracked per peer (`pending` / `connected` / `error`).
- **Node groups and labels**: Tag nodes with arbitrary key=value labels (`PUT /api/nodes/{id}/labels`) and organise them into named groups (`GET|POST /api/groups`). Groups track membership explicitly; `GET /api/groups/{id}/nodes` lists all member nodes. Useful for targeting bulk operations by environment, region, or role.
- **CRA compliance report** (`GET /api/compliance/report`): Generates a structured report covering users, mesh topology, audit log summary, security incidents, encryption details, key management status, and audit retention policy — aligned with EU Cyber Resilience Act (CRA) 2024/2847. Available in `json` (default), `csv` (raw audit log), and `text` (human-readable) formats via the `?format=` query parameter.
- **Webhook notifications**: HTTP webhook delivery for mesh events (`node.enrolled`, `node.offline`, `node.online`, `healer.service_restart`, `audit.critical`, `user.created`, `auth.sso_login`). Each delivery is HMAC-SHA256 signed via `X-Scutum-Signature`. Includes a test endpoint (`POST /api/webhooks/{id}/test`) and one automatic retry on failure. Managed via admin-only CRUD at `GET|POST /api/webhooks` and `GET|PUT|DELETE /api/webhooks/{id}`.
- **SCIM 2.0 user provisioning**: Full RFC 7644 implementation at `/scim/v2/Users` supporting create, read, update, patch (including `active=false` deprovisioning), and delete. Uses dedicated long-lived bearer tokens (managed via `POST /api/scim/tokens`) separate from user JWTs. Compatible with Microsoft Entra ID, Okta, and any SCIM 2.0 IdP.
- **Audit log forwarding**: Background worker ships audit log entries every 30 seconds to configured external endpoints in JSON or CEF (ArcSight/QRadar) format. Managed via admin-only CRUD at `GET|POST /api/audit/forwarders` and `GET|PUT|DELETE /api/audit/forwarders/{id}`.
- **Single Sign-On (SSO)**: OIDC/OAuth2 login support for Microsoft Entra ID (Azure AD / Office 365), GitHub, Authentik, Keycloak, and any generic OIDC provider. Providers are enabled via environment variables (`SSO_<PROVIDER>_CLIENT_ID` / `CLIENT_SECRET` / `ISSUER_URL`) and only appear on the login page when configured. On first SSO login, the account is linked by email or created automatically. New endpoints: `GET /api/auth/sso/providers` (public — returns enabled providers), `GET /api/auth/sso/{provider}` (initiates login), `GET /api/auth/sso/{provider}/callback` (exchanges code and issues JWT).
- **Automatic TLS via ACME / Let's Encrypt**: Set `ACME_DOMAIN` and `ACME_EMAIL` to enable automatic certificate provisioning and renewal. Scutum starts an HTTP-01 challenge server on port 80 and redirects all plain-HTTP traffic to HTTPS. Set `ACME_STAGING=true` to use the Let's Encrypt staging environment. When unset, the existing manual cert (`CERT_FILE`/`KEY_FILE`) and plain-HTTP fallback behaviour is unchanged.
- **Kubernetes operator** (`operator/`): CRD-based cluster management with `ScutumHub` and `ScutumNode` custom resources. The operator reconciles StatefulSets, Services, RBAC, and ConfigMaps for hub and edge deployments; automatically enrolls edge nodes into the mesh via the hub enrollment API; and manages a bootstrap Secret per node containing WireGuard config and HMAC credentials. Requires `scutum.io/v1alpha1` CRDs installed via `operator/config/crd/`.
- **Operator bootstrap endpoint** (`GET /api/operator/bootstrap`): Admin-only endpoint that returns the hub's WireGuard public key, listen port, HMAC key (base64), and mesh CIDR. Used by the operator to configure edge nodes without requiring direct access to the hub's secrets volume.
- **Helm chart** (`helm/scutum`): Production-ready Helm chart deploying Scutum as a StatefulSet with a `ClusterIP` service for the API/UI and a `LoadBalancer` service for the WireGuard UDP port. Includes: TLS init container (self-signed cert generation on first start), Gateway API `HTTPRoute` template, `ClusterRole`/`ClusterRoleBinding` for Kubernetes in-cluster access, PVC-backed storage for `/data` and `/secrets`, and `NET_ADMIN` capability wiring for WireGuard.
- **NAT roaming via WireGuard keepalive**: The mesh now relies on WireGuard's native `persistent-keepalive = 25` to track endpoint changes transparently — no application-layer polling required. When an edge node changes networks, WireGuard's keepalive packets update the kernel's peer endpoint automatically. The healer's `FreshEndpoint` callback reads the live endpoint from `wg show endpoints` (with the database as a fallback) so re-adds always use the current address. On hub startup, `registerOwnEndpoint` performs a one-shot registration; the previous 2-minute periodic re-registration ticker has been removed.
- **Hub API address auto-derivation**: The hub API address for a remote node is now inferred from its `AllowedIPs` when the field is a single `/32` (or `/128`) host route, so operators no longer need to fill in a separate "Hub API address" field for standard mesh configurations. Explicit values still take priority; the WireGuard endpoint is used as a last resort.
- **`cmd/internal/roaming` package**: Extracted `NodeAPIBase`, `CallRegisterEndpoint`, and `HubMeshIP` from `package main` into an importable package so they can be unit-tested and reused outside the main binary.
- **`GetPeerEndpoint` utility**: New `utils.GetPeerEndpoint(iface, publicKey)` reads the live endpoint for a given peer directly from the WireGuard kernel state via `wg show <iface> endpoints`, and is exposed through the `wireguard.Service` interface.

### Changed
- **Node type simplified**: The `combined` node type has been removed. Valid node types are now `hub` and `remote` only. Existing `peer`, `edge`, and `combined` rows are migrated to `remote` on first startup. The node enrollment modal, filters, stats strip, and default role in Node Defaults settings have all been updated accordingly.
- **Setup page — Hub API address is optional**: The "Hub API address" field is now labelled optional and its placeholder explains it is auto-derived from the Hub allowed IPs. The hint no longer mentions periodic re-registration.
- **Nodes page — API address auto-derived on enroll**: The "API address" field in the Enroll Peer modal is optional; when left blank the value is derived from the node's `AllowedIPs` (strips the `/32` prefix). The Enroll button no longer requires the field to be non-empty. `document.execCommand('copy')` replaced with the standard Clipboard API.
- **Node detail page — accurate WireGuard labels**: The "Endpoint" row is renamed "API address" to reflect that the displayed value is the mesh IP / API address, not the UDP WireGuard endpoint. The hardcoded "Listen port: 51820" is replaced with "—" since the port is not returned by the API.

### Fixed
- **SQLite schema**: `node_groups`, `node_group_members`, and `audit_forwarders` tables were truncated (missing closing `);` and missing `description`/`created_at` columns on `node_groups`). The same truncation existed across SQLite, MySQL, and PostgreSQL schemas and caused a fatal `near "CREATE": syntax error` on startup.
- **useApi.ts EOF**: `downloadComplianceReport` was missing its closing `}`, causing all subsequent function declarations (Webhooks, SCIM, SSO, audit forwarders) to be parsed inside that function body, producing an unexpected-EOF build failure.
- **Docker routes return 503 in Kubernetes**: All Docker API endpoints now check for the socket at `/var/run/docker.sock` before executing. When the socket is not mounted (the default in Kubernetes), the response is `503 Service Unavailable` with a clear JSON message instead of a raw dial-failure `500 Internal Server Error`. A `requireDocker` middleware in the router handles this uniformly across all 15 Docker endpoints.
- **Compose deploy error when Docker CLI absent**: `POST /docker/deploy-compose` now checks for the `docker` CLI binary via `exec.LookPath` and returns a `503` with a descriptive message if it is not found in `PATH`, rather than returning an opaque error from the failed `exec.Command` call.

---

## [1.0.0] — 2026-05-23

### Added
- **Multi-stage Dockerfile**: Single-command build producing an Alpine-based binary image with an embedded CycloneDX SBOM generated by Syft at build time.
- **Hub-to-Node Proxying**: All Docker and Kubernetes API actions can now be forwarded to remote nodes by setting the `X-Target-Node: <node-id>` request header. Inter-node requests are authenticated with an HMAC-SHA256 signature (`X-Scutum-Hub-Sig` + `X-Scutum-Hub-Ts`) derived from the shared `sync_hmac_key`, so edge nodes never need to trust a browser JWT.
- **Leader Election**: Hub instances now use a DB-backed lease (`hub_leases` table) to elect a single active leader, preventing IP conflicts when multiple Hub containers share the same database.
- **Hub Interface Restoration**: On startup, if the system is already set up, Scutum automatically restores the `wg0` WireGuard interface from configuration saved in the database, enabling seamless container restarts and hot-standby failover.
- **Rate Limiting**: IP-based token-bucket rate limiter (2 req/s, burst 5) applied to all public authentication endpoints (`/auth/login`, `/auth/register`, `/setup`, `/auth/forgot-password`) to mitigate brute-force attacks. Includes automatic TTL-based eviction to prevent memory growth.
- **Graceful Shutdown**: The API server now catches `SIGINT`/`SIGTERM` and performs an ordered shutdown: HTTP server drain → Healer stop → Pusher stop → DB close.
- **Audit Log Retention**: Automated background pruning of `audit_logs`, `system_logs`, and `traces` tables. Defaults to 365 days (configurable via `AUDIT_RETENTION_DAYS`) to align with EU Cyber Resilience Act recommendations.
- **Admin Password Complexity**: Initial admin credentials must meet a 12-character minimum with uppercase, lowercase, number, and special-character requirements.
- **Docker Compose Healthcheck**: Added `healthcheck` directive to `docker-compose.yaml` via `GET /api/health`.
- **Backup Script**: Added `scripts/backup.sh` supporting SQLite (WAL checkpoint), PostgreSQL (`pg_dump`), and MySQL (`mysqldump`) with automatic retention purging (`BACKUP_RETAIN_DAYS`, default 90 days).
- **mTLS Support**: Enforces mutual TLS on incoming Hub connections when `CA_CERT_FILE` is configured; injects client certificates into outgoing edge sync (`HTTPEdgeSink`).
- **Multi-Database Support**: Automatic driver selection based on `DATABASE_URL` — PostgreSQL, MySQL, or SQLite.
- **Prometheus Metrics**: HTTP request counter (`scutum_http_requests_total`), healer health checks (`scutum_healer_checks_total`), and mesh sync latency (`scutum_mesh_sync_latency_seconds`) exposed at `/api/metrics`.
- **TOTP / MFA**: TOTP-based multi-factor authentication with recovery codes for all user accounts.
- **KMS Integration**: Local, AWS KMS, GCP KMS, and HashiCorp Vault key providers for envelope encryption of all secrets.
- **Role-Based Access Control**: Fine-grained permissions model with role assignment per user.
- **Plugin System**: Native Go plugin support with a route registrar for extending the API.
- **WireGuard Auto-Install**: Automatic installation of `wireguard-go` when the kernel module is absent.

### Changed
- Module path renamed from `orcistrator` to `scutum`; database and storage directory paths updated accordingly.
- Frontend runs as a pure SPA (`ssr: false`) to eliminate hydration mismatches between server-rendered and client-rendered state.
- All frontend API calls use relative paths (`/api/...`) so the UI works correctly regardless of the host or port it is served from.
- WebSocket terminal URLs now derive from `window.location` with an `/api` prefix, fixing connections when the app is proxied or served on a non-default port.
- Prerender step skips `/api/**` routes to prevent build-time 502 errors when the backend is not available during `nuxt generate`.
- Mesh summary (`/api/network/mesh-summary`) counts the local node in `total` and `healthy`, giving an accurate view of the full mesh including the hub itself.
- `WGChecker` interface methods now accept a `context.Context`; a `ContextRunner` adapter bridges the existing context-free `utils.CommandRunner` to the new interface.

### Fixed
- `registerEdges` goroutine now retries up to 5 times with exponential back-off to handle transient startup failures.
- Rate limiter state is evicted every 5 minutes for IPs not seen recently, preventing unbounded memory growth.
- Healer: check timeout context is now correctly passed to `PeerHandshakeAge` instead of being silently discarded, preventing goroutine hangs when the `wg` CLI is slow.
- Healer: service restart backoff is now enforced — unhealthy services are no longer restarted on every tick regardless of the configured backoff window.
- Healer: `PeerHandshakeAge` no longer panics when a public key is shorter than 8 bytes.
- Healer: `Stop()` waits for the heal-loop goroutine to finish before returning.
- Pusher: `Stop()` resets the edge map to an empty map instead of `nil`, preventing a panic on any subsequent `Register()` call.
- Pusher: `HTTPEdgeSink.Send` uses `bytes.NewReader` instead of `strings.NewReader(string(body))`, eliminating a redundant allocation.
- Mesh test: removed host-side WireGuard UDP port mappings that caused bind failures on hosts already running a WireGuard interface; container-to-container traffic uses the Docker bridge directly.
- Mesh test: Stage 4 crash-recovery is now deterministic — `docker wait` ensures the container has fully exited before `docker start` is called, removing the dependency on the Docker restart policy.

---

## [0.1.0] — Initial Development Snapshot

- Initial commit: core WireGuard mesh orchestration API.
- SQLite-backed store with WireGuard peer management.
- Basic authentication with JWT and API keys.
- Embedded Nuxt frontend.
- Docker single-binary build.
