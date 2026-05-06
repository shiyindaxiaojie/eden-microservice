# Cluster Example

This directory provides a three-node cluster bootstrap example for local verification of cluster behavior.

## What It Starts

- node 1: HTTP `:8500`, frontend `:2019`
- node 2: HTTP `:8501`, frontend `:2020`
- node 3: HTTP `:8502`, frontend `:2021`

The example uses the configuration files under [`configs`](./configs) and currently targets `cluster + cp`.

## Files

| File | Purpose |
| --- | --- |
| [`start.bat`](./start.bat) | build the server, clean old data, start three backend nodes, register members, then start three frontend dev servers |
| [`restart.bat`](./restart.bat) | stop the existing example processes and start them again |
| [`start.sh`](./start.sh) | Linux or macOS shell script to start the three backend nodes and register members |
| [`restart.sh`](./restart.sh) | stop the existing example processes and start them again on Linux or macOS |
| [`configs/node1.yaml`](./configs/node1.yaml) | bootstrap node configuration |
| [`configs/node2.yaml`](./configs/node2.yaml) | follower node configuration |
| [`configs/node3.yaml`](./configs/node3.yaml) | follower node configuration |

## Start On Windows

```bat
examples\cluster\start.bat
```

What the script does:

1. stops old example windows
2. removes `data/node1`, `data/node2`, and `data/node3`
3. builds `registry-server.exe`
4. starts three backend nodes
5. logs into node 1 and registers node 2 and node 3 as cluster members
6. starts three frontend dev servers

## Start On Linux Or macOS

```bash
chmod +x ./examples/cluster/start.sh
./examples/cluster/start.sh
```

The shell script starts backend nodes only and writes logs to:

- `/tmp/registry-node1.log`
- `/tmp/registry-node2.log`
- `/tmp/registry-node3.log`

## Operational Notes

- `start.bat` deletes local cluster data before startup. Use it for a clean bootstrap, not for preserving state.
- `restart.bat` keeps the data directories by default and is intended for code-level restart verification.
- the frontend windows require local Node.js and project dependencies under `web`
- the cluster member registration step assumes node 1 is reachable at `http://127.0.0.1:8500`

## Related Reading

- [Deployment guide](../../docs/deployment.md)
- [Simplified Chinese version](./README-zh-CN.md)
