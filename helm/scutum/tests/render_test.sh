#!/usr/bin/env bash
# Helm render tests for the scutum chart.
# Requires: helm >= 3.0
# Usage: ./helm/scutum/tests/render_test.sh
set -euo pipefail

CHART="$(cd "$(dirname "$0")/.." && pwd)"
PASS=0
FAIL=0

# ── helpers ──────────────────────────────────────────────────────────────────

green() { printf '\033[32m✔ %s\033[0m\n' "$*"; }
red()   { printf '\033[31m✘ %s\033[0m\n' "$*"; }

render() {
  # render <release-name> [--set key=val ...]
  local release="$1"; shift
  helm template "$release" "$CHART" "$@" 2>&1
}

assert_contains() {
  local label="$1" needle="$2" haystack="$3"
  if echo "$haystack" | grep -qF "$needle"; then
    green "$label"
    (( PASS++ )) || true
  else
    red "$label"
    echo "  expected to find: $needle"
    (( FAIL++ )) || true
  fi
}

assert_not_contains() {
  local label="$1" needle="$2" haystack="$3"
  if ! echo "$haystack" | grep -qF "$needle"; then
    green "$label"
    (( PASS++ )) || true
  else
    red "$label"
    echo "  expected NOT to find: $needle"
    (( FAIL++ )) || true
  fi
}

# ── tests ─────────────────────────────────────────────────────────────────────

echo "── Default values ───────────────────────────────────────────────────────"
OUT=$(render scutum)

assert_contains     "StatefulSet rendered"                   "kind: StatefulSet"                         "$OUT"
assert_contains     "ServiceAccount rendered"                "kind: ServiceAccount"                      "$OUT"
assert_contains     "ClusterRole rendered"                   "kind: ClusterRole"                         "$OUT"
assert_contains     "ConfigMap rendered"                     "kind: ConfigMap"                           "$OUT"
assert_contains     "API Service rendered"                   "kind: Service"                             "$OUT"
assert_contains     "Default image tag"                      "ghcr.io/sovforge/scutum:latest"            "$OUT"
assert_contains     "Health liveness probe"                  "/api/health"                               "$OUT"
assert_contains     "NET_ADMIN capability"                   "NET_ADMIN"                                 "$OUT"
assert_contains     "WireGuard UDP port"                     "51820"                                     "$OUT"
assert_contains     "WireGuard service LoadBalancer"         "type: LoadBalancer"                        "$OUT"
assert_contains     "Data PVC claim template"                "name: data"                                "$OUT"
assert_contains     "Secrets PVC claim template"             "name: secrets"                             "$OUT"
assert_contains     "TLS init container"                     "tls-init"                                  "$OUT"
assert_contains     "Dev net tun volume"                     "/dev/net/tun"                              "$OUT"
assert_not_contains "No DB secret by default"                "kind: Secret"                              "$OUT"
assert_not_contains "No HTTPRoute by default"                "kind: HTTPRoute"                           "$OUT"
assert_not_contains "No Docker socket by default"            "docker.sock"                               "$OUT"

echo ""
echo "── Custom image tag ──────────────────────────────────────────────────────"
OUT=$(render scutum --set image.tag=v1.2.3)
assert_contains     "Custom image tag applied"               "ghcr.io/sovforge/scutum:v1.2.3"            "$OUT"

echo ""
echo "── External database URL ─────────────────────────────────────────────────"
OUT=$(render scutum --set database.url="postgres://user:pass@pg:5432/scutum")
assert_contains     "DB Secret rendered"                     "kind: Secret"                              "$OUT"
assert_contains     "DATABASE_URL in secret"                 "DATABASE_URL"                              "$OUT"
assert_contains     "SecretKeyRef in StatefulSet"            "secretKeyRef"                              "$OUT"

echo ""
echo "── Gateway API HTTPRoute ─────────────────────────────────────────────────"
OUT=$(render scutum --set gateway.enabled=true --set gateway.name=prod-gateway \
  --set "gateway.hostnames[0]=scutum.example.com")
assert_contains     "HTTPRoute rendered"                     "kind: HTTPRoute"                           "$OUT"
assert_contains     "Gateway parent ref"                     "name: prod-gateway"                        "$OUT"
assert_contains     "HTTPRoute hostname"                     "scutum.example.com"                        "$OUT"

echo ""
echo "── Docker socket mount ───────────────────────────────────────────────────"
OUT=$(render scutum --set docker.enabled=true)
assert_contains     "Docker socket volume"                   "docker.sock"                               "$OUT"
assert_contains     "Docker socket hostPath"                 "hostPath"                                  "$OUT"

echo ""
echo "── WireGuard disabled ────────────────────────────────────────────────────"
OUT=$(render scutum --set wireguard.enabled=false)
assert_not_contains "No WireGuard service"                   "name: scutum-wireguard"                    "$OUT"
assert_not_contains "No WireGuard port"                      "51820"                                     "$OUT"
assert_not_contains "No dev net tun"                         "/dev/net/tun"                              "$OUT"

echo ""
echo "── TLS auto-generate disabled ────────────────────────────────────────────"
OUT=$(render scutum --set tls.autoGenerate=false)
assert_not_contains "No TLS init container"                  "tls-init"                                  "$OUT"

echo ""
echo "── RBAC disabled ─────────────────────────────────────────────────────────"
OUT=$(render scutum --set rbac.create=false)
assert_not_contains "No ClusterRole when rbac disabled"      "kind: ClusterRole"                         "$OUT"
assert_not_contains "No ClusterRoleBinding when rbac disabled" "kind: ClusterRoleBinding"                "$OUT"

echo ""
echo "── ServiceAccount disabled ───────────────────────────────────────────────"
OUT=$(render scutum --set serviceAccount.create=false --set rbac.create=false)
assert_not_contains "No SA when disabled"                    "kind: ServiceAccount"                      "$OUT"

echo ""
echo "── NodePort WireGuard service ────────────────────────────────────────────"
OUT=$(render scutum --set service.wireguard.type=NodePort --set service.wireguard.nodePort=31820)
assert_contains     "NodePort type"                          "type: NodePort"                            "$OUT"
assert_contains     "Explicit nodePort"                      "nodePort: 31820"                           "$OUT"

echo ""
echo "── Extra env vars ────────────────────────────────────────────────────────"
OUT=$(render scutum \
  --set "extraEnv[0].name=OTEL_EXPORTER_OTLP_ENDPOINT" \
  --set "extraEnv[0].value=http://otel:4317")
assert_contains     "Extra env var injected"                 "OTEL_EXPORTER_OTLP_ENDPOINT"               "$OUT"

echo ""
echo "── helm lint ─────────────────────────────────────────────────────────────"
if helm lint "$CHART" > /dev/null 2>&1; then
  green "helm lint passes"
  (( PASS++ )) || true
else
  red "helm lint failed"
  helm lint "$CHART"
  (( FAIL++ )) || true
fi

# ── summary ───────────────────────────────────────────────────────────────────
echo ""
echo "────────────────────────────────────────────────────────────────────────"
echo "Results: ${PASS} passed, ${FAIL} failed"
echo "────────────────────────────────────────────────────────────────────────"

[[ $FAIL -eq 0 ]]
