#!/usr/bin/env bash
# Integration test: deploy the scutum Helm chart into a local kind cluster and
# verify the pod reaches Ready and the health endpoint responds.
#
# Requirements: kind, kubectl, helm, docker
# Usage: ./scripts/test-k8s.sh [--keep]
#   --keep  do not delete the cluster after the test (useful for debugging)
#
# Set IMAGE=<ref> to use a locally built image instead of pulling from GHCR:
#   IMAGE=scutum:dev ./scripts/test-k8s.sh
set -euo pipefail

CLUSTER_NAME="scutum-test"
NAMESPACE="scutum-test"
RELEASE="scutum"
CHART="$(cd "$(dirname "$0")/../helm/scutum" && pwd)"
IMAGE="${IMAGE:-ghcr.io/sovforge/scutum:latest}"
KEEP_CLUSTER=false
TIMEOUT=120s
KUBECONFIG_FILE=""
PF_PID=""

for arg in "$@"; do
  [[ "$arg" == "--keep" ]] && KEEP_CLUSTER=true
done

# ── utilities ─────────────────────────────────────────────────────────────────

log()  { printf '\033[36m[k8s-test]\033[0m %s\n' "$*"; }
ok()   { printf '\033[32m[k8s-test] ✔ %s\033[0m\n' "$*"; }
fail() { printf '\033[31m[k8s-test] ✘ %s\033[0m\n' "$*"; exit 1; }

require_cmd() {
  command -v "$1" >/dev/null 2>&1 || fail "Required command not found: $1 — install it and re-run."
}

cleanup() {
  [[ -n "$PF_PID" ]] && kill "$PF_PID" 2>/dev/null || true
  [[ -n "$KUBECONFIG_FILE" ]] && rm -f "$KUBECONFIG_FILE"
  if [[ "$KEEP_CLUSTER" == "false" ]]; then
    log "Deleting kind cluster '${CLUSTER_NAME}'..."
    kind delete cluster --name "$CLUSTER_NAME" 2>/dev/null || true
  else
    log "Cluster kept. Delete with: kind delete cluster --name ${CLUSTER_NAME}"
  fi
}
trap cleanup EXIT

# ── preflight ─────────────────────────────────────────────────────────────────

require_cmd kind
require_cmd kubectl
require_cmd helm
require_cmd docker

log "Running Helm render tests first..."
bash "$(dirname "$0")/../helm/scutum/tests/render_test.sh"

# ── kind cluster ──────────────────────────────────────────────────────────────

if kind get clusters 2>/dev/null | grep -q "^${CLUSTER_NAME}$"; then
  log "Reusing existing kind cluster '${CLUSTER_NAME}'"
else
  log "Creating kind cluster '${CLUSTER_NAME}'..."
  kind create cluster --name "$CLUSTER_NAME"
fi

KUBECONFIG_FILE="$(mktemp)"
kind get kubeconfig --name "$CLUSTER_NAME" > "$KUBECONFIG_FILE"
export KUBECONFIG="$KUBECONFIG_FILE"

kubectl cluster-info --context "kind-${CLUSTER_NAME}" > /dev/null

# ── load image ────────────────────────────────────────────────────────────────

if [[ "$IMAGE" != ghcr.io/* ]]; then
  log "Loading local image '${IMAGE}' into kind cluster..."
  kind load docker-image "$IMAGE" --name "$CLUSTER_NAME"
fi

# ── deploy ────────────────────────────────────────────────────────────────────

log "Creating namespace '${NAMESPACE}'..."
kubectl create namespace "$NAMESPACE" --dry-run=client -o yaml | kubectl apply -f -

log "Installing Helm release '${RELEASE}'..."
helm upgrade --install "$RELEASE" "$CHART" \
  --namespace "$NAMESPACE" \
  --set image.repository="${IMAGE%%:*}" \
  --set "image.tag=${IMAGE##*:}" \
  --set image.pullPolicy=IfNotPresent \
  --set wireguard.enabled=false \
  --set tls.autoGenerate=false \
  --set livenessProbe.httpGet.scheme=HTTP \
  --set readinessProbe.httpGet.scheme=HTTP \
  --wait \
  --timeout "$TIMEOUT"

ok "Helm release installed"

# ── wait for ready ────────────────────────────────────────────────────────────

log "Waiting for StatefulSet rollout (timeout: ${TIMEOUT})..."
kubectl rollout status statefulset/"${RELEASE}" \
  --namespace "$NAMESPACE" \
  --timeout "$TIMEOUT"
ok "StatefulSet rollout complete"

# ── port-forward ──────────────────────────────────────────────────────────────

LOCAL_PORT=18080
log "Starting port-forward on localhost:${LOCAL_PORT}..."
kubectl port-forward \
  --namespace "$NAMESPACE" \
  "svc/${RELEASE}" \
  "${LOCAL_PORT}:8080" &
PF_PID=$!
sleep 2  # give port-forward time to establish

# ── health check ──────────────────────────────────────────────────────────────

log "Checking GET /api/health..."
for attempt in 1 2 3 4 5; do
  BODY=$(curl -sf "http://localhost:${LOCAL_PORT}/api/health" 2>/dev/null || true)
  if echo "$BODY" | grep -q '"status"'; then
    ok "Health endpoint responded: ${BODY}"
    break
  fi
  if [[ $attempt -eq 5 ]]; then
    log "Pod logs:"
    kubectl logs --namespace "$NAMESPACE" \
      "$(kubectl get pods -n "$NAMESPACE" -o name | head -1)" --tail=40 || true
    fail "Health endpoint did not respond after ${attempt} attempts"
  fi
  log "Attempt ${attempt}/5 — retrying in 3s..."
  sleep 3
done

# ── Docker feature availability ───────────────────────────────────────────────

log "Verifying Docker endpoint returns 503 when socket not mounted..."
HTTP_STATUS=$(curl -sf -o /dev/null -w "%{http_code}" \
  "http://localhost:${LOCAL_PORT}/api/docker/containers" \
  -H "Authorization: Bearer test-token" 2>/dev/null || true)

# Auth middleware runs before the Docker check, so 401/403 is also acceptable.
case "$HTTP_STATUS" in
  503) ok "Docker endpoint returned 503 (socket not mounted — correct)" ;;
  401|403) ok "Docker endpoint returned ${HTTP_STATUS} (auth layer reached before Docker check — acceptable)" ;;
  *) fail "Docker endpoint returned unexpected status ${HTTP_STATUS} (expected 503/401/403)" ;;
esac

# ── summary ───────────────────────────────────────────────────────────────────

echo ""
ok "All k8s integration tests passed"
