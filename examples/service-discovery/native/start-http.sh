#!/bin/bash
set -e

echo "======================================================="
echo " pkg/sdk Service Discovery Demo - HTTP"
echo "======================================================="
echo ""

: "${EDEN_HTTP_ADDRS:=http://127.0.0.1:8500}"

WORKDIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)"
cd "$WORKDIR"

echo "Focalors HTTP addresses: $EDEN_HTTP_ADDRS"
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
EDEN_HTTP_ADDRS="$EDEN_HTTP_ADDRS" SERVICE_PORT=21102 SERVICE_ID=native-http-auth-center-1 go run ./examples/service-discovery/native/cmd/http/auth-center &
PIDS+=($!)
EDEN_HTTP_ADDRS="$EDEN_HTTP_ADDRS" SERVICE_PORT=21112 SERVICE_ID=native-http-auth-center-2 go run ./examples/service-discovery/native/cmd/http/auth-center &
PIDS+=($!)
EDEN_HTTP_ADDRS="$EDEN_HTTP_ADDRS" SERVICE_PORT=21122 SERVICE_ID=native-http-auth-center-3 go run ./examples/service-discovery/native/cmd/http/auth-center &
PIDS+=($!)
sleep 2

echo "[2/3] Starting user-center instances..."
EDEN_HTTP_ADDRS="$EDEN_HTTP_ADDRS" SERVICE_PORT=21101 SERVICE_ID=native-http-user-center-1 go run ./examples/service-discovery/native/cmd/http/user-center &
PIDS+=($!)
EDEN_HTTP_ADDRS="$EDEN_HTTP_ADDRS" SERVICE_PORT=21111 SERVICE_ID=native-http-user-center-2 go run ./examples/service-discovery/native/cmd/http/user-center &
PIDS+=($!)
sleep 2

echo "[3/3] Starting order-center instances..."
EDEN_HTTP_ADDRS="$EDEN_HTTP_ADDRS" SERVICE_PORT=21103 SERVICE_ID=native-http-order-center-1 go run ./examples/service-discovery/native/cmd/http/order-center &
PIDS+=($!)
EDEN_HTTP_ADDRS="$EDEN_HTTP_ADDRS" SERVICE_PORT=21113 SERVICE_ID=native-http-order-center-2 go run ./examples/service-discovery/native/cmd/http/order-center &
PIDS+=($!)
sleep 2

echo ""
echo "Test URLs:"
echo "  http://127.0.0.1:21102/api/auth/token?user_id=1"
echo "  http://127.0.0.1:21101/api/users/1/profile"
echo "  http://127.0.0.1:21103/api/orders/create?user_id=1"
echo "  http://127.0.0.1:21103/api/orders/demo?user_id=1"
echo ""
echo "Press Ctrl+C to stop all HTTP demo processes..."

wait
