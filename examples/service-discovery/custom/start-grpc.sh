#!/bin/bash
set -e

echo "======================================================="
echo " Custom Service Discovery Demo - gRPC"
echo "======================================================="
echo ""

: "${CUSTOM_GRPC_ADDRS:=127.0.0.1:9000}"

WORKDIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)"
cd "$WORKDIR"

echo "Focalors gRPC addresses: $CUSTOM_GRPC_ADDRS"
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
CUSTOM_GRPC_ADDRS="$CUSTOM_GRPC_ADDRS" SERVICE_PORT=24002 SERVICE_ID=custom-grpc-auth-center-1 go run ./examples/service-discovery/custom/cmd/grpc/auth-center &
PIDS+=($!)
CUSTOM_GRPC_ADDRS="$CUSTOM_GRPC_ADDRS" SERVICE_PORT=24012 SERVICE_ID=custom-grpc-auth-center-2 go run ./examples/service-discovery/custom/cmd/grpc/auth-center &
PIDS+=($!)
CUSTOM_GRPC_ADDRS="$CUSTOM_GRPC_ADDRS" SERVICE_PORT=24022 SERVICE_ID=custom-grpc-auth-center-3 go run ./examples/service-discovery/custom/cmd/grpc/auth-center &
PIDS+=($!)
sleep 2

echo "[2/3] Starting user-center instances..."
CUSTOM_GRPC_ADDRS="$CUSTOM_GRPC_ADDRS" SERVICE_PORT=24001 SERVICE_ID=custom-grpc-user-center-1 go run ./examples/service-discovery/custom/cmd/grpc/user-center &
PIDS+=($!)
CUSTOM_GRPC_ADDRS="$CUSTOM_GRPC_ADDRS" SERVICE_PORT=24011 SERVICE_ID=custom-grpc-user-center-2 go run ./examples/service-discovery/custom/cmd/grpc/user-center &
PIDS+=($!)
sleep 2

echo "[3/3] Starting order-center instances..."
CUSTOM_GRPC_ADDRS="$CUSTOM_GRPC_ADDRS" SERVICE_PORT=24003 SERVICE_ID=custom-grpc-order-center-1 go run ./examples/service-discovery/custom/cmd/grpc/order-center &
PIDS+=($!)
CUSTOM_GRPC_ADDRS="$CUSTOM_GRPC_ADDRS" SERVICE_PORT=24013 SERVICE_ID=custom-grpc-order-center-2 go run ./examples/service-discovery/custom/cmd/grpc/order-center &
PIDS+=($!)
sleep 2

echo ""
echo "Test URLs:"
echo "  http://127.0.0.1:24002/api/auth/token?user_id=1"
echo "  http://127.0.0.1:24001/api/users/1/profile"
echo "  http://127.0.0.1:24003/api/orders/create?user_id=1"
echo "  http://127.0.0.1:24003/api/orders/demo?user_id=1"
echo ""
echo "Press Ctrl+C to stop all gRPC demo processes..."

wait
