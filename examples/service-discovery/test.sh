#!/bin/bash
echo "Running Service Discovery Example..."

WORKDIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$WORKDIR/examples/service-discovery"

go run main.go

echo ""
echo "Done."
