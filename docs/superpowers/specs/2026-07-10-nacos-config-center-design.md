# Nacos Config Center Design

## Goal

Deliver Eden Config as a durable control-plane domain: the console reads and
writes server-side data, and standard Nacos Config HTTP clients can publish,
retrieve, delete, and long-poll the same resources. Apollo compatibility,
config import/export, rollback, and clustered Config replication are outside
this increment.

## Architecture

`internal/configcenter` owns the Config resource model, validation, bbolt
repository, revision/history rules, and change notifications. It has no
dependency on HTTP, console code, or Nacos types. The HTTP handler and the
Nacos adapter both depend on its service interface, so a configuration written
through either API is immediately visible through the other.

The repository is always durable. It opens `${data_dir}/config/configs.db`,
creates `configs`, `config_history`, and `config_index` buckets, and closes on
server shutdown. No in-memory data store is used as a persistence fallback;
the only live memory is a bounded notification registry for outstanding
long-poll requests.

## Resource and version semantics

Every resource is normalized to `namespace/group/data_id`; empty namespace is
`default` and empty group is `DEFAULT_GROUP`. The service trims identity and
metadata fields, rejects empty or path-like `data_id` values, accepts empty
content, and calculates MD5 over the complete content.

The repository maintains one monotonic global revision sequence. A changed
publish writes a history entry for the previous state (when present), writes
the new current resource with the next revision, and records the publisher.
An unchanged content publish updates display metadata but does not add history,
advance revision, or notify listeners. Delete is logical: it removes the item
from the current bucket, appends a delete history entry with a new revision,
and wakes listeners. A supplied `expected_md5` that does not match the current
version produces a conflict.

## Native HTTP API

The management and runtime routes are served under the existing `/v1` API:

| Route | Behavior | Auth |
| --- | --- | --- |
| `GET /v1/config` | Fetch one active resource with metadata and content. | admin/developer |
| `GET /v1/configs` | Paginated list/search of active resources. | admin/developer |
| `POST /v1/config` | Create or publish a resource. | admin/developer |
| `PUT /v1/config` | Publish an existing resource, optionally with `expected_md5`. | admin/developer |
| `DELETE /v1/config` | Logically delete a resource. | admin/developer |
| `GET /v1/config/history` | List identity-scoped history, newest first. | admin/developer |
| `POST /v1/config/listener` | Exact-key long poll returning only changed metadata. | API key |

The listener returns promptly if the client MD5 is stale; otherwise it waits
for its configured timeout, capped by the server's maximum. A timeout returns
an empty change list. A client may only wait for a bounded number of keys; the
adapter returns a diagnostic HTTP error when that limit or the global waiter
limit is exceeded.

## Nacos compatibility

`internal/adapter/nacos` gains a Config adapter registered at both
`/nacos/v1/cs/*` and `/v1/cs/*`, matching the existing dual-prefix Naming
adapter. `tenant`, `group`, and `dataId` map respectively to Eden namespace,
group, and data ID. All input is form-compatible, as used by common Nacos
clients.

- `GET /configs` returns raw content with a text content type; absent active
  configurations return 404.
- `POST /configs` publishes content and returns literal `true`.
- `DELETE /configs` logically deletes and returns literal `true`; deleting a
  missing resource remains idempotently successful.
- `POST /configs/listener` accepts `Listening-Configs` or form-encoded
  listener data. It waits for matching changes and returns a Nacos
  change-key payload without configuration content.

Nacos mutation routes use the existing API-key middleware. Query and listener
routes are left client-accessible, consistent with the existing Naming
compatibility routes; deployments can enable API-key client authentication via
the service configuration.

## Console and executable example

The configuration page keeps its current UI but obtains list, history,
publish, and delete data from a new `web/src/api/config` module. It sends
access tokens through the shared Axios client and presents backend validation
or conflict errors. Mock configuration functions and browser local-storage
state are no longer referenced by this view.

`examples/config/nacos` contains a small Go Nacos Config client using the
existing Nacos SDK dependency. Its launcher starts Eden with an isolated data
directory, publishes a `.properties` configuration through the real Nacos
endpoint, starts a listener, publishes a changed value, and prints the
callback result. `start.sh` and `start.bat` provide the same one-command flow.

## Errors and verification

Domain tests cover normalization, persistence across reopen, MD5/CAS rules,
history, deletion, and listener wakeup/timeout. HTTP tests cover native JSON
contracts and Nacos raw-content/boolean/listener contracts. The example is
verified against a launched local server, and the web checks run i18n and the
production build.
