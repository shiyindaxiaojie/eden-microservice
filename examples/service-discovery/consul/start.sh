#!/bin/bash
set -e

echo "======================================================="
echo " Official Consul API Service Discovery Demo"
echo "======================================================="
echo ""

: "${CONSUL_ADDR:=127.0.0.1:8500}"

WORKDIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)"
cd "$WORKDIR"

echo "Focalors address: $CONSUL_ADDR"
echo ""

PIDS=()

cleanup() {
    echo ""
    echo "Shutting down..."
    for pid in "${PIDS[@]}"; do
        kill "$pid" 2>/dev/null || true
    done
    echo "Done."
}

trap cleanup EXIT

echo "[1/3] Starting auth-center instances..."
CONSUL_ADDR="$CONSUL_ADDR" SERVICE_PORT=22002 SERVICE_ID=consul-auth-center-1 go run ./examples/service-discovery/consul/cmd/auth-center &
PIDS+=($!)
CONSUL_ADDR="$CONSUL_ADDR" SERVICE_PORT=22012 SERVICE_ID=consul-auth-center-2 go run ./examples/service-discovery/consul/cmd/auth-center &
PIDS+=($!)
CONSUL_ADDR="$CONSUL_ADDR" SERVICE_PORT=22022 SERVICE_ID=consul-auth-center-3 go run ./examples/service-discovery/consul/cmd/auth-center &
PIDS+=($!)
sleep 2

echo "[2/3] Starting user-center instances..."
CONSUL_ADDR="$CONSUL_ADDR" SERVICE_PORT=22001 SERVICE_ID=consul-user-center-1 go run ./examples/service-discovery/consul/cmd/user-center &
PIDS+=($!)
CONSUL_ADDR="$CONSUL_ADDR" SERVICE_PORT=22011 SERVICE_ID=consul-user-center-2 go run ./examples/service-discovery/consul/cmd/user-center &
PIDS+=($!)
sleep 2

echo "[3/3] Starting order-center instances..."
CONSUL_ADDR="$CONSUL_ADDR" SERVICE_PORT=22003 SERVICE_ID=consul-order-center-1 go run ./examples/service-discovery/consul/cmd/order-center &
PIDS+=($!)
CONSUL_ADDR="$CONSUL_ADDR" SERVICE_PORT=22013 SERVICE_ID=consul-order-center-2 go run ./examples/service-discovery/consul/cmd/order-center &
PIDS+=($!)
sleep 2

echo ""
echo "Test URLs:"
echo "  http://127.0.0.1:22002/api/auth/token?user_id=1"
echo "  http://127.0.0.1:22001/api/users/1/profile"
echo "  http://127.0.0.1:22003/api/orders/create?user_id=1"
echo "  http://127.0.0.1:22003/api/orders/demo?user_id=1"
echo ""
echo "Press Ctrl+C to stop all demo processes..."

wait
