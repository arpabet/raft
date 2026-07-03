# raft

Raft consensus building blocks for the arpabet stack — a thin, glue-injectable
layer over [`hashicorp/raft`](https://github.com/hashicorp/raft) providing the
node runtime (transport, log/stable/snapshot stores, membership) and a pluggable
**control plane** (bootstrap / join / configuration / command-forwarding /
snapshot-recovery) available over more than one wire.

Extracted from `go.arpabet.com/sprint` into its own repo so applications can pull
just the raft pieces without the sprint framework.

This is a **multi-module monorepo** (one `go.mod` per module, coordinated by
`go.work`), following the `go.arpabet.com/store` / `sprint` layout.

## Modules

| Module | Path | Role |
|--------|------|------|
| `raftapi`  | `go.arpabet.com/raft/raftapi`  | Interfaces — transport-neutral (no gRPC/vRPC types). `RaftServer`, `RaftService` (the `raft.FSM`), `RaftClientPool` (`GetAPIConn` returns `any`), `ServerLookup`, `SerfServer` |
| `raftpb`   | `go.arpabet.com/raft/raftpb`   | Protobuf messages + gRPC service definition (used by `raftgrpc`) |
| `raftmod`  | `go.arpabet.com/raft/raftmod`  | Raft node runtime: TCP consensus transport, log/stable/snapshot stores, serf membership, server lifecycle, gRPC client pool |
| `raftgrpc` | `go.arpabet.com/raft/raftgrpc` | Control service over **gRPC**: `Bootstrap`/`Join`/`GetConfiguration`/`ApplyCommand`/`Recover` + the `raft` CLI |
| `raftvrpc` | `go.arpabet.com/raft/raftvrpc` | Control service over **value-rpc** (`go.arpabet.com/value-rpc`) — same contract, schemaless Go-to-Go wire *(in progress)* |

The node-to-node **consensus** transport (`raftmod`, plain TCP + `raft.NetworkTransport`)
is independent of the control-plane wire: `raftgrpc` and `raftvrpc` are
interchangeable transports for the *management* API and share one `raftapi`.

## Control plane

`ApplyCommand` is the linearizable forward-to-leader primitive: a follower
receiving a write forwards it to the leader via the `RaftClientPool` connection
(gRPC or vRPC, selected by which pool is wired). `raftapi.RaftClientPool.GetAPIConn`
returns `any` so callers type-assert to their transport's client.

## Build

```sh
# per-module, resolved via go.work
(cd raftmod && go build ./... && go test ./...)
```

## License

Business Source License 1.1 (BUSL-1.1) — matching `value-rpc`. Copyright (c)
2025-2026 Karagatan LLC. Change License MPL 2.0 after the Change Date. See
[LICENSE](LICENSE).
