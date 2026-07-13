#!/usr/bin/env sh
set -eu

demo_dir=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
repo_root=$(CDPATH= cd -- "$demo_dir/../../.." && pwd)
server_bin="$demo_dir/.demo-server"
client_bin="$demo_dir/.demo-client"
data_dir="$demo_dir/.demo-data"

cd "$repo_root"
go build -o "$server_bin" ./cmd/server/main.go
go build -o "$client_bin" ./examples/config/nacos/cmd/listener
"$server_bin" -http-addr :8858 -data-dir "$data_dir" -mode standalone -consistency ap -grpc off -quic off -raft off &
server_pid=$!
trap 'kill "$server_pid" 2>/dev/null || true' EXIT INT TERM

attempt=0
until curl --silent --fail http://127.0.0.1:8858/health >/dev/null 2>&1; do
  attempt=$((attempt + 1))
  if [ "$attempt" -ge 60 ]; then
    echo "Eden server did not become healthy" >&2
    exit 1
  fi
  sleep 1
done

"$client_bin" -server 127.0.0.1:8858
