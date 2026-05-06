#!/bin/bash
set -e

echo "======================================================="
echo " Official Nacos SDK Service Discovery Demo"
echo "======================================================="
echo ""

: "${NACOS_ADDR:=127.0.0.1:8500}"

WORKDIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)"
cd "$WORKDIR"

echo "Focalors address: $NACOS_ADDR"
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
NACOS_ADDR="$NACOS_ADDR" SERVICE_PORT=23002 SERVICE_ID=nacos-auth-center-1 go run ./examples/service-discovery/nacos/cmd/auth-center &
PIDS+=($!)
NACOS_ADDR="$NACOS_ADDR" SERVICE_PORT=23012 SERVICE_ID=nacos-auth-center-2 go run ./examples/service-discovery/nacos/cmd/auth-center &
PIDS+=($!)
NACOS_ADDR="$NACOS_ADDR" SERVICE_PORT=23022 SERVICE_ID=nacos-auth-center-3 go run ./examples/service-discovery/nacos/cmd/auth-center &
PIDS+=($!)
sleep 2

echo "[2/3] Starting user-center instances..."
NACOS_ADDR="$NACOS_ADDR" SERVICE_PORT=23001 SERVICE_ID=nacos-user-center-1 go run ./examples/service-discovery/nacos/cmd/user-center &
PIDS+=($!)
NACOS_ADDR="$NACOS_ADDR" SERVICE_PORT=23011 SERVICE_ID=nacos-user-center-2 go run ./examples/service-discovery/nacos/cmd/user-center &
PIDS+=($!)
sleep 2

echo "[3/3] Starting order-center instances..."
NACOS_ADDR="$NACOS_ADDR" SERVICE_PORT=23003 SERVICE_ID=nacos-order-center-1 go run ./examples/service-discovery/nacos/cmd/order-center &
PIDS+=($!)
NACOS_ADDR="$NACOS_ADDR" SERVICE_PORT=23013 SERVICE_ID=nacos-order-center-2 go run ./examples/service-discovery/nacos/cmd/order-center &
PIDS+=($!)
sleep 2

echo ""
echo "Test URLs:"
echo "  http://127.0.0.1:23002/api/auth/token?user_id=1"
echo "  http://127.0.0.1:23001/api/users/1/profile"
echo "  http://127.0.0.1:23003/api/orders/create?user_id=1"
echo "  http://127.0.0.1:23003/api/orders/demo?user_id=1"
echo ""
echo "Press Ctrl+C to stop all demo processes..."

wait
