#!/bin/bash

# --- Configuration ---
HUB_URL="http://localhost:8080"
EDGE_URL="http://localhost:8081"
INTERNAL_HUB_IP="172.20.0.2"
INTERNAL_EDGE_IP="172.20.0.3"
COMPOSE_FILE="docker-compose.test.yml"
CHAOS_MESH_FILE="chaos-mesh-templates.yaml"

# --- Cleanup Function ---
cleanup() {
    local exit_code=$?
    echo -e "\n\n🧹 Cleaning up test environment..."

    # Kill any background pumba jobs still running (stages 2 & 3 launch with &)
    # shellcheck disable=SC2046
    kill $(jobs -p) 2>/dev/null || true

    # Stop and remove test containers + locally built images
    docker compose -f "$COMPOSE_FILE" down --rmi local --remove-orphans > /dev/null 2>&1 || true

    # Remove test data dirs — Docker may have written root-owned files inside the
    # bind-mounted volumes, so fall back to sudo only if a plain rm fails.
    for dir in ./scutum_data ./scutum_secrets ./edge_data ./edge_secrets; do
        rm -rf "$dir" 2>/dev/null || sudo rm -rf "$dir" 2>/dev/null || true
    done

    # Remove generated files
    rm -f "$COMPOSE_FILE" "$CHAOS_MESH_FILE" ./master.key ./scutum.db ./edge.db 2>/dev/null || true

    echo "✨ Environment wiped."
    exit "$exit_code"
}

# Intercept Ctrl-C with a clear message; EXIT trap handles the actual cleanup.
trap 'echo -e "\n🛑 Interrupted — running cleanup..."; exit 130' INT TERM
trap cleanup EXIT

# --- Step 0: Compose Definition ---
cat <<EOF > $COMPOSE_FILE
services:
  scutum:
    build: 
      context: .
      dockerfile: Dockerfile
    container_name: scutum
    cap_add:
      - NET_ADMIN
    networks:
      scutum_mesh:
        ipv4_address: $INTERNAL_HUB_IP
    ports:
      - "8080:8080"
    volumes:
      - /var/run/docker.sock:/var/run/docker.sock
      - ./scutum_data:/data
      - ./scutum_secrets:/secrets
    environment:
      - HEALER_INTERVAL=5s
    healthcheck:
      test: ["CMD", "curl", "-f", "http://localhost:8080/api/health"]
      interval: 5s
      timeout: 2s
      retries: 3

  edge-node:
    build:
      context: .
      dockerfile: Dockerfile
    container_name: edge-node
    cap_add:
      - NET_ADMIN
    networks:
      scutum_mesh:
        ipv4_address: $INTERNAL_EDGE_IP
    volumes:
      - ./edge_data:/data
      - ./edge_secrets:/secrets
    ports:
      - "8081:8080"
    restart: always

networks:
  scutum_mesh:
    ipam:
      config:
        - subnet: 172.20.0.0/24
EOF

# --- Step 1: Initialization ---
# Tear down any leftover resources from a previous run (including stale networks
# that would block subnet allocation) before bringing everything up fresh.
echo "🧹 Pre-flight teardown of any leftover test resources..."
docker compose -f "$COMPOSE_FILE" down --remove-orphans > /dev/null 2>&1 || true
# Also purge any legacy-named networks that share the same subnet.
docker network rm orcistrator_scutum_mesh orcistrator_sovran_mesh > /dev/null 2>&1 || true

echo "🏗️  Building and starting containers..."
docker compose -f $COMPOSE_FILE up -d --build

echo "⏳ Waiting for health checks..."
MAX_RETRIES=30
COUNT=0
until curl -s -f "$HUB_URL/api/health" > /dev/null && curl -s -f "$EDGE_URL/api/health" > /dev/null; do 
    printf "."
    sleep 1
    COUNT=$((COUNT+1))
    if [ $COUNT -gt $MAX_RETRIES ]; then
        echo -e "\n❌ Startup Timeout!"
        docker ps
        docker logs scutum
        docker logs edge-node
        exit 1
    fi
done
echo -e "\n✅ Services are Healthy."

generate_setup_json() {
  local type=$1; local wg_addr=$2; local hub_pub=$3
  cat <<EOF
{
  "install_type": "$type",
  "kms": { "provider": "local", "local": { "key_file": "/secrets/master.key" } },
  "wireguard": { 
    "listen_port": 51820, "address": "$wg_addr", "mtu": 1420, 
    "hub_endpoint": "$INTERNAL_HUB_IP:51820", "hub_public_key": "$hub_pub", "hub_allowed_ips": "10.0.0.0/24" 
  },
  "admin": { "username": "admin", "password": "Admin@123456!" }
}
EOF
}

# --- Step 2: Provisioning ---
echo "🛠️  Provisioning Mesh..."
HUB_RES=$(curl -s -X POST "$HUB_URL/api/setup" -H "Content-Type: application/json" -d "$(generate_setup_json "hub" "10.0.0.1/24" "")")
HUB_PUB_KEY=$(echo "$HUB_RES" | jq -r '.wireguard.public_key // empty' 2>/dev/null | tr -d '\n')
if [ -z "$HUB_PUB_KEY" ]; then
    echo "❌ Hub setup failed. Response: $HUB_RES"
    exit 1
fi
echo "✅ Hub public key: $HUB_PUB_KEY"

EDGE_RES=$(curl -s -X POST "$EDGE_URL/api/setup" -H "Content-Type: application/json" -d "$(generate_setup_json "remote" "10.0.0.2/24" "$HUB_PUB_KEY")")
EDGE_PUB_KEY=$(echo "$EDGE_RES" | jq -r '.wireguard.public_key // empty' 2>/dev/null | tr -d '\n')
if [ -z "$EDGE_PUB_KEY" ]; then
    echo "❌ Edge setup failed. Response: $EDGE_RES"
    exit 1
fi
echo "✅ Edge public key: $EDGE_PUB_KEY"

TOKEN=$(curl -s -X POST "$HUB_URL/api/auth/login" -H "Content-Type: application/json" -d '{"username": "admin", "password": "Admin@123456!"}' | jq -r .token)
if [ -z "$TOKEN" ] || [ "$TOKEN" = "null" ]; then
    echo "❌ Login failed — check hub setup completed successfully"
    exit 1
fi
echo "✅ Authenticated."

# --- Step 3: Registration & Handshake Delay ---
echo "📡 Registering Peers..."
curl -s -X POST "$HUB_URL/api/network/peer" -H "Authorization: Bearer $TOKEN" -H "Content-Type: application/json" \
     -d "{\"public_key\": \"$EDGE_PUB_KEY\", \"endpoint\": \"$INTERNAL_EDGE_IP:51820\", \"allowed_ips\": \"10.0.0.2/32\"}" > /dev/null

echo "⏳ Allowing 10s for WireGuard handshake to stabilize..."
sleep 10

# --- Step 4: Verification & Chaos Stages ---
check_health() {
  local stage=$1
  echo -e "\n📊 Health Status [$stage]:"
  curl -s -X GET "$HUB_URL/api/network/mesh-summary" -H "Authorization: Bearer $TOKEN" | jq .
}

echo -e "\n⚡ Stage 1: Overlay Ping (Healthy Baseline)"
docker exec scutum ping -c 3 10.0.0.2 || exit 1
check_health "Baseline"

echo -e "\n🔥 Stage 2: Latency Chaos (300ms + 100ms jitter)"
docker run --rm -v /var/run/docker.sock:/var/run/docker.sock gaiaadm/pumba netem --duration 15s delay --time 300 --jitter 100 edge-node > /dev/null &
sleep 5
docker exec scutum ping -c 3 10.0.0.2
check_health "Latency"
sleep 15

echo -e "\n🔥 Stage 3: Packet Loss Chaos (20% Loss)"
docker run --rm -v /var/run/docker.sock:/var/run/docker.sock gaiaadm/pumba netem --duration 15s loss --percent 20 edge-node > /dev/null &
sleep 5
docker exec scutum ping -c 5 10.0.0.2
check_health "Packet Loss"
sleep 15

echo -e "\n🔥 Stage 4: Service Kill (Testing Healer/Pusher Resilience)"
docker kill --signal SIGKILL edge-node > /dev/null 2>&1
echo "💀 edge-node killed. Waiting for it to exit..."
docker wait edge-node > /dev/null 2>&1 || true
echo "▶️  Restarting edge-node..."
docker start edge-node > /dev/null

MAX_RETRIES=40
COUNT=0
until curl -s -f "$EDGE_URL/api/health" > /dev/null; do
    printf "."
    sleep 1
    COUNT=$((COUNT+1))
    if [ $((COUNT % 10)) -eq 0 ]; then
        echo -e "\n[Wait $COUNT/40] Status: $(docker inspect -f '{{.State.Status}}' edge-node)"
    fi
    if [ $COUNT -gt $MAX_RETRIES ]; then
        echo -e "\n❌ Timeout waiting for edge-node!"
        docker ps
        docker logs edge-node
        exit 1
    fi
done
echo -e "\n♻️  Service Recovered. Waiting for Healer to restore mesh (5s interval)..."
sleep 10
echo -e "\n⚡ Testing Overlay Ping after recovery..."
docker exec scutum ping -c 3 10.0.0.2
check_health "Recovery"

# --- Chaos Mesh Documentation ---
cat <<EOF > $CHAOS_MESH_FILE
# Chaos Mesh Templates for Kubernetes
# Apply these using: kubectl apply -f $CHAOS_MESH_FILE

apiVersion: chaos-mesh.org/v1alpha1
kind: NetworkChaos
metadata:
  name: edge-latency
spec:
  action: delay
  mode: one
  selector:
    labelSelectors:
      'app': 'edge-node'
  delay:
    latency: '300ms'
    correlation: '100'
    jitter: '50ms'
  duration: '30s'
---
apiVersion: chaos-mesh.org/v1alpha1
kind: PodChaos
metadata:
  name: edge-kill
spec:
  action: pod-kill
  mode: one
  selector:
    labelSelectors:
      'app': 'edge-node'
  duration: '10s'
EOF

echo -e "\n✅ Success! All chaos stages completed."
echo "📜 Chaos Mesh templates generated in $CHAOS_MESH_FILE"