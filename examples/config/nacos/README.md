# Nacos Config Compatibility Example

This example uses the official Nacos Go HTTP client to prove that Eden can act
as a Nacos Config server. It publishes a properties file, reads it back,
subscribes to it, publishes a changed value, receives the callback, reloads the
content, and deletes the demo resource.

## One-command run

Windows:

```bat
examples\config\nacos\start.bat
```

Linux or macOS:

```sh
./examples/config/nacos/start.sh
```

The launchers build a temporary server binary, start Eden on `127.0.0.1:8858`
with persistent data under `.demo-data`, run the client, and stop only the
server process they created. The generated binary and data directory are
ignored by Git.

To point the client at an already running server:

```sh
go run ./examples/config/nacos/cmd/listener \
  -server 127.0.0.1:8500 \
  -namespace default \
  -group DEFAULT_GROUP \
  -data-id demo.properties
```
