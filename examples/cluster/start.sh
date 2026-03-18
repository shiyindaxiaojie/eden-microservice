#!/bin/bash
echo "======================================================="
echo " Eden Go Registry - 3 Node Cluster Startup"
echo "======================================================="
echo ""

WORKDIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$WORKDIR"

# 0. Cleanup old data
echo "[1/3] Cleaning old cluster data..."
rm -rf data/node1 data/node2 data/node3

# 1. Build
echo "[2/3] Building server binary..."
go build -o eden-server ./cmd/server/main.go
if [ $? -ne 0 ]; then
    echo "ERROR: Build failed!"
    exit 1
fi
echo "      Build successful."
echo ""

# 2. Start nodes
echo "[3/3] Starting cluster nodes..."

echo "  Node 1 (HTTP: 8500)..."
./eden-server -config examples/cluster/configs/node1.yaml > /tmp/eden-node1.log 2>&1 &
NODE1_PID=$!
sleep 3

echo "  Node 2 (HTTP: 8501)..."
./eden-server -config examples/cluster/configs/node2.yaml > /tmp/eden-node2.log 2>&1 &
NODE2_PID=$!
sleep 2

echo "  Node 3 (HTTP: 8502)..."
./eden-server -config examples/cluster/configs/node3.yaml > /tmp/eden-node3.log 2>&1 &
NODE3_PID=$!

echo ""
echo "======================================================="
echo " Cluster started successfully!"
echo "======================================================="
echo ""
echo "  Node 1: http://localhost:8500 (PID: $NODE1_PID)"
echo "  Node 2: http://localhost:8501 (PID: $NODE2_PID)"
echo "  Node 3: http://localhost:8502 (PID: $NODE3_PID)"
echo ""
echo "  Logs: /tmp/eden-node{1,2,3}.log"
echo ""
echo "Press Ctrl+C to stop the cluster..."

function cleanup() {
    echo ""
    echo "Shutting down..."
    kill $NODE1_PID $NODE2_PID $NODE3_PID 2>/dev/null
    rm -f eden-server
    echo "Done."
}

trap cleanup EXIT
wait
