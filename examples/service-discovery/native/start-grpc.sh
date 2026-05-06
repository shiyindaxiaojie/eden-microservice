#!/bin/bash
set -e

echo "======================================================="
echo " pkg/sdk Service Discovery Demo - gRPC"
echo "======================================================="
echo ""

: "${EDEN_GRPC_ADDRS:=127.0.0.1:9000}"

WORKDIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)"
cd "$WORKDIR"

echo "Focalors gRPC addresses: $EDEN_GRPC_ADDRS"
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
EDEN_GRPC_ADDRS="$EDEN_GRPC_ADDRS" SERVICE_PORT=21002 SERVICE_ID=native-grpc-auth-center-1 go run ./examples/service-discovery/native/cmd/grpc/auth-center &
PIDS+=($!)
EDEN_GRPC_ADDRS="$EDEN_GRPC_ADDRS" SERVICE_PORT=21012 SERVICE_ID=native-grpc-auth-center-2 go run ./examples/service-discovery/native/cmd/grpc/auth-center &
PIDS+=($!)
EDEN_GRPC_ADDRS="$EDEN_GRPC_ADDRS" SERVICE_PORT=21022 SERVICE_ID=native-grpc-auth-center-3 go run ./examples/service-discovery/native/cmd/grpc/auth-center &
PIDS+=($!)
sleep 2

echo "[2/3] Starting user-center instances..."
EDEN_GRPC_ADDRS="$EDEN_GRPC_ADDRS" SERVICE_PORT=21001 SERVICE_ID=native-grpc-user-center-1 go run ./examples/service-discovery/native/cmd/grpc/user-center &
PIDS+=($!)
EDEN_GRPC_ADDRS="$EDEN_GRPC_ADDRS" SERVICE_PORT=21011 SERVICE_ID=native-grpc-user-center-2 go run ./examples/service-discovery/native/cmd/grpc/user-center &
PIDS+=($!)
sleep 2

echo "[3/3] Starting order-center instances..."
EDEN_GRPC_ADDRS="$EDEN_GRPC_ADDRS" SERVICE_PORT=21003 SERVICE_ID=native-grpc-order-center-1 go run ./examples/service-discovery/native/cmd/grpc/order-center &
PIDS+=($!)
EDEN_GRPC_ADDRS="$EDEN_GRPC_ADDRS" SERVICE_PORT=21013 SERVICE_ID=native-grpc-order-center-2 go run ./examples/service-discovery/native/cmd/grpc/order-center &
PIDS+=($!)
sleep 2

echo ""
echo "Test URLs:"
echo "  http://127.0.0.1:21002/api/auth/token?user_id=1"
echo "  http://127.0.0.1:21001/api/users/1/profile"
echo "  http://127.0.0.1:21003/api/orders/create?user_id=1"
echo "  http://127.0.0.1:21003/api/orders/demo?user_id=1"
echo ""
echo "Press Ctrl+C to stop all gRPC demo processes..."

wait
