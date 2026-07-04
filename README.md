# raft

Raft consensus building blocks for the arpabet stack — a thin, glue-injectable
layer over [`hashicorp/raft`](https://github.com/hashicorp/raft) providing the
node runtime (transport, log/stable/snapshot stores, membership) and a pluggable
**control plane** (bootstrap / join / configuration / command-forwarding /
snapshot-recovery) available over more than one wire.

Originally extracted from `go.arpabet.com/sprint`; since v0.4.0 the library has
**no sprint dependency**. It targets applications built with
[`cligo`](https://go.arpabet.com/cligo) (CLI + DI bootstrap) and
[`servion`](https://go.arpabet.com/servion) (server runtime): the raft/serf
servers implement `servion.Server`, so `servion.RunCommand` runs them, and the
management CLIs are cligo command groups.

This is a **multi-module monorepo** (one `go.mod` per module, coordinated by
`go.work`), following the `go.arpabet.com/store` layout.

## Modules

| Module | Path | Role |
|--------|------|------|
| `raftapi`  | `go.arpabet.com/raft/raftapi`  | Interfaces — transport-neutral (no gRPC/vRPC types). `RaftServer`, `RaftService` (the `raft.FSM`), `RaftClientPool` (`GetAPIConn` returns `any`), `ServerLookup`, `SerfServer`, `NodeService` (node identity), `AuthorizationMiddleware` (optional ADMIN gate) |
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

## Using with cligo + servion

```go
cligo.Main(
    cligo.Name("myapp"), cligo.Version(version), cligo.Build(build),
    cligo.Beans(raftcmd.RaftCommands...),   // 'serf' command group
    cligo.Beans(raftgrpc.Commands()...),    // 'raft' command group (or raftvrpc.Commands())
    cligo.Beans(servion.RunCommand(append(raftmod.RaftServices,
        myFSM,             // raft.FSM bean
        myRaftStore,       // badger store.ManagedDataStore bean named "raft-store"
        myHCLog,           // hclog.Logger bean
        raftgrpc.RaftGrpcServer(), // control plane (or raftvrpc.RaftVrpcServer())
    )...)),
)
```

Node identity comes from the default `raftmod.NodeService()` bean (properties
`node.id` / `node.name` / `node.seq`, falling back to the cligo application
name); register your own `raftapi.NodeService` bean to override. The control
plane's ADMIN gate (`raftapi.AuthorizationMiddleware`) is optional — without it
the transport (e.g. mTLS) is expected to authenticate peers. See
[`raftmod/README.md`](raftmod/README.md) for the full property table.

### Migration from ≤ v0.3.x (sprint)

- `sprint.Application`, `sprint.NodeService`, `sprint.SystemEnvironmentPropertyResolver`
  injections are gone. Node identity is `raftapi.NodeService` (default bean
  included in `raftmod.RaftServices`); application name/version/build come from
  the cligo app bean or the `application.*` properties.
- `raftapi.RaftServer` / `raftapi.SerfServer` now embed `servion.Server` /
  `servion.Component`; their `Serve()` blocks until `Shutdown()` per the
  servion contract.
- The serf CLI (`raftcmd.RaftCommands`) is a cligo command group; the serf agent
  address property moved from `serf-server.rpc-address` to `serf.rpc-address`
  (same property the server binds).
- The snapshot encryption token is no longer prompted interactively: set the
  property named by `raft.snapshot-key-bean` or the matching environment
  variable (e.g. `RAFT_SNAPSHOT_KEY`).

## Build

```sh
# per-module, resolved via go.work
(cd raftmod && go build ./... && go test ./...)
```

## License

Business Source License 1.1 (BUSL-1.1) — matching `value-rpc`. Copyright (c)
2025-2026 Karagatan LLC. Change License MPL 2.0 after the Change Date. See
[LICENSE](LICENSE).
