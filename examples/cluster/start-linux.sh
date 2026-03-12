#!/bin/bash
echo "Starting Eden Go Registry AP Cluster (3 Nodes)..."

WORKDIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$WORKDIR"

echo "Starting Node 1 (Port 8500)..."
go run ./cmd/server/main.go -config configs/node1.yaml > /tmp/eden-node1.log 2>&1 &
NODE1_PID=$!

sleep 2

echo "Starting Node 2 (Port 8501)..."
go run ./cmd/server/main.go -config configs/node2.yaml > /tmp/eden-node2.log 2>&1 &
NODE2_PID=$!

sleep 2

echo "Starting Node 3 (Port 8502)..."
go run ./cmd/server/main.go -config configs/node3.yaml > /tmp/eden-node3.log 2>&1 &
NODE3_PID=$!

echo "AP Cluster started successfully!"
echo "Node 1: http://localhost:8500 (PID: $NODE1_PID)"
echo "Node 2: http://localhost:8501 (PID: $NODE2_PID)"
echo "Node 3: http://localhost:8502 (PID: $NODE3_PID)"
echo ""
echo "Press Ctrl+C to stop the cluster..."

function cleanup() {
    echo ""
    echo "Stopping nodes..."
    kill $NODE1_PID
    kill $NODE2_PID
    kill $NODE3_PID
    echo "Cluster stopped."
}

trap cleanup EXIT
wait
