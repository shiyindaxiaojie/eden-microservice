#!/bin/bash
echo "======================================================="
echo " Focalors - 3 Node Cluster Startup"
echo "======================================================="
echo ""

WORKDIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$WORKDIR"

# 0. Cleanup old data
echo "[1/4] Cleaning old cluster data..."
rm -rf data/node1 data/node2 data/node3

# 1. Build
echo "[2/4] Building server binary..."
go build -o registry-server ./cmd/server/main.go
if [ $? -ne 0 ]; then
    echo "ERROR: Build failed!"
    exit 1
fi
echo "      Build successful."
echo ""

# 2. Start nodes
echo "[3/4] Starting cluster nodes..."

echo "  Node 1 (HTTP: 8500)..."
./registry-server -config examples/cluster/configs/node1.yaml > /tmp/registry-node1.log 2>&1 &
NODE1_PID=$!
sleep 3

echo "  Node 2 (HTTP: 8501)..."
./registry-server -config examples/cluster/configs/node2.yaml > /tmp/registry-node2.log 2>&1 &
NODE2_PID=$!
sleep 2

echo "  Node 3 (HTTP: 8502)..."
./registry-server -config examples/cluster/configs/node3.yaml > /tmp/registry-node3.log 2>&1 &
NODE3_PID=$!
sleep 2

# 3. Register members via API
echo "[4/4] Registering cluster members through API..."
LOGIN_RESP=$(curl -sS -X POST http://127.0.0.1:8500/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"8c6976e5b5410415bde908bd4dee15dfb167a9c873fc4bb8a81f6f2ab448a918"}')
TOKEN=$(printf '%s' "$LOGIN_RESP" | sed -n 's/.*"token":"\([^"]*\)".*/\1/p')
if [ -z "$TOKEN" ]; then
    echo "ERROR: Failed to login to node 1!"
    exit 1
fi
curl -sS -X POST http://127.0.0.1:8500/v1/cluster/member \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"addresses":["http://127.0.0.1:8501","http://127.0.0.1:8502"]}' > /dev/null
if [ $? -ne 0 ]; then
    echo "ERROR: Failed to register cluster members!"
    exit 1
fi
echo "      Cluster membership configured."

echo ""
echo "======================================================="
echo " Cluster started successfully!"
echo "======================================================="
echo ""
echo "  Node 1: http://localhost:8500 (PID: $NODE1_PID)"
echo "  Node 2: http://localhost:8501 (PID: $NODE2_PID)"
echo "  Node 3: http://localhost:8502 (PID: $NODE3_PID)"
echo ""
echo "  Logs: /tmp/registry-node{1,2,3}.log"
echo ""
echo "Press Ctrl+C to stop the cluster..."

function cleanup() {
    echo ""
    echo "Shutting down..."
    kill $NODE1_PID $NODE2_PID $NODE3_PID 2>/dev/null
    rm -f registry-server
    echo "Done."
}

trap cleanup EXIT
wait


