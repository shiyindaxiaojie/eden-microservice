#!/bin/bash
set -e

echo "======================================================="
echo " Custom Service Discovery Demo - HTTP"
echo "======================================================="
echo ""

: "${CUSTOM_HTTP_ADDRS:=http://127.0.0.1:8500}"

WORKDIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)"
cd "$WORKDIR"

echo "Focalors HTTP addresses: $CUSTOM_HTTP_ADDRS"
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
CUSTOM_HTTP_ADDRS="$CUSTOM_HTTP_ADDRS" SERVICE_PORT=24102 SERVICE_ID=custom-http-auth-center-1 go run ./examples/service-discovery/custom/cmd/http/auth-center &
PIDS+=($!)
CUSTOM_HTTP_ADDRS="$CUSTOM_HTTP_ADDRS" SERVICE_PORT=24112 SERVICE_ID=custom-http-auth-center-2 go run ./examples/service-discovery/custom/cmd/http/auth-center &
PIDS+=($!)
CUSTOM_HTTP_ADDRS="$CUSTOM_HTTP_ADDRS" SERVICE_PORT=24122 SERVICE_ID=custom-http-auth-center-3 go run ./examples/service-discovery/custom/cmd/http/auth-center &
PIDS+=($!)
sleep 2

echo "[2/3] Starting user-center instances..."
CUSTOM_HTTP_ADDRS="$CUSTOM_HTTP_ADDRS" SERVICE_PORT=24101 SERVICE_ID=custom-http-user-center-1 go run ./examples/service-discovery/custom/cmd/http/user-center &
PIDS+=($!)
CUSTOM_HTTP_ADDRS="$CUSTOM_HTTP_ADDRS" SERVICE_PORT=24111 SERVICE_ID=custom-http-user-center-2 go run ./examples/service-discovery/custom/cmd/http/user-center &
PIDS+=($!)
sleep 2

echo "[3/3] Starting order-center instances..."
CUSTOM_HTTP_ADDRS="$CUSTOM_HTTP_ADDRS" SERVICE_PORT=24103 SERVICE_ID=custom-http-order-center-1 go run ./examples/service-discovery/custom/cmd/http/order-center &
PIDS+=($!)
CUSTOM_HTTP_ADDRS="$CUSTOM_HTTP_ADDRS" SERVICE_PORT=24113 SERVICE_ID=custom-http-order-center-2 go run ./examples/service-discovery/custom/cmd/http/order-center &
PIDS+=($!)
sleep 2

echo ""
echo "Test URLs:"
echo "  http://127.0.0.1:24102/api/auth/token?user_id=1"
echo "  http://127.0.0.1:24101/api/users/1/profile"
echo "  http://127.0.0.1:24103/api/orders/create?user_id=1"
echo "  http://127.0.0.1:24103/api/orders/demo?user_id=1"
echo ""
echo "Press Ctrl+C to stop all HTTP demo processes..."

wait
