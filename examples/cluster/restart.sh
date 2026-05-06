#!/bin/bash
set -e

echo "======================================================="
echo " Focalors - Cluster Restarting"
echo "======================================================="
echo ""

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
WORKDIR="$(cd "$SCRIPT_DIR/../.." && pwd)"
cd "$WORKDIR"

echo "[1/3] Stopping all cluster nodes..."
pkill -f "registry-server -config examples/cluster/configs/node1.yaml" 2>/dev/null || true
pkill -f "registry-server -config examples/cluster/configs/node2.yaml" 2>/dev/null || true
pkill -f "registry-server -config examples/cluster/configs/node3.yaml" 2>/dev/null || true
sleep 2

echo "[2/3] Re-building server binary..."
go build -o registry-server ./cmd/server/main.go
echo "      Build successful."

echo "[3/3] Restarting cluster..."
exec "$SCRIPT_DIR/start.sh"
