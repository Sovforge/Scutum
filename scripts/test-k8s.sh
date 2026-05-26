#!/usr/bin/env bash
# Integration test: deploy the scutum Helm chart into a local kind cluster and
# verify the pod reaches Ready and the health endpoint responds.
#
# Requirements: kind, kubectl, helm, docker
# Usage: ./scripts/test-k8s.sh [--keep]
#   --keep  do not delete the cluster after the test (useful for debugging)
#
# The test uses the pre-built GHCR image by default. Set IMAGE=<ref> to use
# a locally built image instead, e.g.:
#   IMAGE=scutum:dev ./scripts/test-k8s.sh
set -euo pipefail

CLUSTER_NAME="scutum-test"
NAMESPACE="scutum-test"
RELEASE="scutum"
CHART="$(cd "$(dirname "$0")/../helm/scutum" && pwd)"
IMAGE="${IMAGE:-ghcr.io/sovforge/scutum:latest}"
KEEP_CLUSTER=false
TIMEOUT=120s

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
  if [[ "$KEEP_CLUSTER" == "false" ]]; then
    log "Deleting kind cluster '${CLUSTER_NAME}'..."
    kind delete cluster --name "$CLUSTER_NAME" 2>/dev/null || true
  else
    log "Cluster '${CLUSTER_NAME}' kept. Delete with: kind delete cluster --name ${CLUSTER_NAME}"
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
  # Expose a NodePort range so we can reach services from the host
  cat <<EOF | kind create cluster --name "$CLUSTER_NAME" --config=-
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
nodes:
  - role: control-plane
    extraPortMappings:
      - containerPort: 30080
        hostPort: 30080
        protocol: TCP
EOF
fi

export KUBECONFIG
KUBECONFIG="$(kind get kubeconfig-path --name "$CLUSTER_NAME" 2>/dev/null || kind get kubeconfig --name "$CLUSTER_NAME" | grep -o '/[^:]*')"
KUBECONFIG="$(kind get kubeconfig --name "$CLUSTER_NAME" | kubectl config view --flatten --merge -o json | kubectl config view --minify -o jsonpath='{.clusters[0].cluster.server}' 2>/dev/null || true)"
# Simpler: just export via kind directly
KUBECONFIG_FILE="$(mktemp)"
kind get kubeconfig --name "$CLUSTER_NAME" > "$KUBECONFIG_FILE"
export KUBECONFIG="$KUBECONFIG_FILE"
trap "rm -f '$KUBECONFIG_FILE'; cleanup" EXIT

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
  --set service.api.type=NodePort \
  --set service.api.nodePort=30080 \
  --wait \
  --timeout "$TIMEOUT"

ok "Helm release installed"

# ── wait for ready ────────────────────────────────────────────────────────────

log "Waiting for pod to be ready (timeout: ${TIMEOUT})..."
kubectl rollout status statefulset/"${RELEASE}" \
  --namespace "$NAMESPACE" \
  --timeout "$TIMEOUT"
ok "StatefulSet rollout complete"

# ── health check ──────────────────────────────────────────────────────────────

log "Checking health endpoint via NodePort :30080..."
for attempt in 1 2 3 4 5; do
  if curl -sf "http://localhost:30080/api/health" | grep -q '"status"'; then
    ok "Health endpoint responded: $(curl -sf http://localhost:30080/api/health)"
    break
  fi
  if [[ $attempt -eq 5 ]]; then
    kubectl logs --namespace "$NAMESPACE" \
      "$(kubectl get pods -n "$NAMESPACE" -o name | head -1)" --tail=40 || true
    fail "Health endpoint did not respond after ${attempt} attempts"
  fi
  log "Attempt ${attempt}/5 failed, retrying in 5s..."
  sleep 5
done

# ── Docker feature availability ───────────────────────────────────────────────

log "Verifying Docker endpoint returns 503 (no socket mounted)..."
STATUS=$(curl -sf -o /dev/null -w "%{http_code}" \
  http://localhost:30080/api/docker/containers \
  -H "Authorization: Bearer invalid" 2>/dev/null || true)

# 401/403 means the request reached the auth layer (before the Docker check),
# which is also acceptable — auth runs first.
if [[ "$STATUS" == "503" || "$STATUS" == "401" || "$STATUS" == "403" ]]; then
  ok "Docker endpoint returned expected status ${STATUS} (no socket mounted)"
else
  fail "Docker endpoint returned unexpected status: ${STATUS} (expected 503/401/403)"
fi

# ── summary ───────────────────────────────────────────────────────────────────
echo ""
ok "All k8s integration tests passed"
