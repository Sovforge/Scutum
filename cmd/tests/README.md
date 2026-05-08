# Scutum Test Suite

Unit, integration, and end-to-end tests for the Scutum project.

---

## Unit & Integration Tests

All Go tests live in `cmd/tests/` and run against the live package code.

### Test Files

| File | What it covers |
|------|----------------|
| `wireguard_test.go` | Key generation, interface config, peer management |
| `wireguard_handler_test.go` | WireGuard HTTP handlers, mesh summary (including local node count) |
| `docker_test.go` | Docker deployment validation, container config, port mappings |
| `handlers_test.go` | HTTP handlers, status codes, routing, path parameters |
| `handlers_auth_test.go` | Auth endpoints: register, login, API keys |
| `handlers_k8s_test.go` | Kubernetes HTTP handlers |
| `handlers_obs_test.go` | Observability/metrics handlers |
| `handlers_storage_test.go` | Storage handler endpoints |
| `healer_test.go` | WireGuard peer healing, service restart backoff enforcement |
| `pusher_test.go` | Payload signing/verification, edge fan-out, retry logic |
| `sync_handler_test.go` | Sync/push API handlers |
| `auth_internal_test.go` | JWT and API key internals |
| `auth_middleware_test.go` | Auth middleware, hub HMAC request verification |
| `auth_totp_test.go` | TOTP / MFA flows and recovery codes |
| `kubernetes_test.go` | Kubernetes deployments, pods, scaling, namespaces |
| `clients_test.go` | Docker/Kubernetes client setup and error handling |
| `clients_docker_test.go` | Docker client specifics |
| `clients_internal_test.go` | Internal client helpers |
| `websocket_test.go` | WebSocket handshake, frames, masking, opcodes |
| `integration_test.go` | Multi-step flows, concurrency, security, performance |
| `kms_test.go` | KMS provider envelope encryption |
| `kms_recovery_test.go` | Shamir secret sharing and key recovery |
| `recovery_handler_test.go` | Recovery API handlers |
| `nodes_handler_test.go` | Node registration and management handlers |
| `users_roles_handler_test.go` | RBAC: user and role management handlers |
| `plugin_test.go` | Plugin loading and route registration |
| `plugins_test.go` | Plugin lifecycle |
| `setup_handler_test.go` | First-run setup wizard handler |
| `store_test.go` | Core store operations |
| `store_mock_test.go` | Store mock helpers |
| `store_observability_test.go` | Audit log and trace store |
| `store_recovery_codes_test.go` | Recovery code store |
| `store_secret_test.go` | Secret store encryption |
| `store_setup_seed_test.go` | Store seeding on first run |
| `store_users_roles_nodes_test.go` | User, role, and node store |
| `s3_test.go` | S3 SigV4 signing, content hashing, region support |
| `git_test.go` | Git URL validation, credential injection, path handling |
| `export_handler_test.go` | Audit log export handler |
| `base_handler_test.go` | Base handler helpers |
| `api_main_test.go` | Top-level API wiring |
| `utils_test.go` | Utility functions |
| `utils_logger_test.go` | Structured logger |
| `utils_tracing_test.go` | OpenTelemetry tracing helpers |
| `utils_wireguard_test.go` | WireGuard utility functions |
| `version_handler_test.go` | Version endpoint |

### Running the Tests

```bash
# All tests
go test ./cmd/tests/...

# Verbose output
go test ./cmd/tests/... -v

# With race detector (recommended before merging)
go test ./cmd/tests/... -race

# Coverage report
go test ./cmd/tests/... -coverprofile=coverage.out
go tool cover -html=coverage.out

# Single test or pattern
go test ./cmd/tests/... -run TestHealer
go test ./cmd/tests/... -run TestHandleMeshSummary
```

---

## End-to-End Mesh Test

`mesh-test.sh` in the repo root builds and runs a two-node mesh (hub + edge) entirely in Docker, then exercises it through four chaos stages:

| Stage | What it tests |
|-------|---------------|
| **Baseline** | WireGuard overlay ping between hub and edge |
| **Latency** | 300 ms + 100 ms jitter via Pumba; mesh stays up |
| **Packet loss** | 20% loss via Pumba; mesh stays up |
| **Service kill** | SIGKILL edge-node, wait for full exit, restart, healer restores mesh |

### Prerequisites

- Docker with Compose V2
- `jq` (for parsing API responses)
- `pumba` image pulled automatically by the script

### Running

```bash
chmod +x mesh-test.sh
./mesh-test.sh
```

The script tears down any leftovers before starting and cleans up on exit (including Ctrl-C). On success it prints the health summary after each stage and exits 0. On failure it prints container logs and exits 1.

> **Note:** The script does not map WireGuard UDP ports to the host. Containers communicate over the internal `172.20.0.0/24` Docker bridge, so the test works even if port 51820 is already in use on the host.
