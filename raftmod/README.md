# raftmod

Raft node runtime for the arpabet stack: TCP consensus transport,
log/stable/snapshot stores (badger-backed), serf gossip membership and the
server lifecycle beans.

Framework-free by design: it plugs into applications built with
[`cligo`](https://go.arpabet.com/cligo) (CLI + DI bootstrap) and
[`servion`](https://go.arpabet.com/servion) (server runtime). The servers
implement `servion.Server`, so `servion.RunCommand` discovers and runs them
automatically.

## Beans

`raftmod.RaftServices` is the bean list to add to the server scope
(e.g. `servion.RunCommand(raftmod.RaftServices...)`):

- `NodeService()` — default `raftapi.NodeService`: node identity (id, name,
  sequence) resolved from properties, with the cligo application name as
  fallback. Replace with your own bean if the application manages identity.
- `RaftLogStoreFactory()` / `RaftStableStoreFactory()` — raft log and stable
  stores on top of a badger `store.ManagedDataStore` bean named `raft-store`
  (**must be provided by the application**).
- `RaftSnapshotFactory()` — file snapshot store, optionally encrypted (see
  `raft.snapshot-key-bean` below).
- `SerfConfigFactory()` — serf config with node identity tags
  (`id`, `role`, `version`, `build`, `port`, `raft-port`, `grpc-port`).
- `ServerLookup()` — raft `ServerAddressProvider` fed by serf membership.
- `SerfRPCServer()` — serf agent + its RPC endpoint (`servion.Server`).
- `RaftServer()` — the raft node itself (`servion.Server`).
- `RaftClientPool()` — gRPC connection pool to peer control endpoints.

The application must also provide beans for: `raft.FSM` (the state machine),
`hclog.Logger`, and the `raft-store` badger store.

## Properties

| Property | Default | Purpose |
|----------|---------|---------|
| `raft.bind-address` | — | raft consensus bind address (required to activate) |
| `serf.bind-address` | — | serf gossip bind address (required to activate) |
| `serf.rpc-address` | `:8700` | serf agent RPC endpoint (used by the `serf` CLI) |
| `serf.rpc-auth` | — | serf agent RPC auth key |
| `node.id` | derived | pin a stable node id (hex of this is the raft ServerID) |
| `node.name` | app name | base node name |
| `node.seq` | `0` | node sequence; all bind ports are shifted by it |
| `node.lan` | local name | override the serf gossip node name |
| `application.name` | cligo app name | cluster role name (serf `role` tag) |
| `application.version` / `application.build` | cligo app | serf `version`/`build` tags |
| `application.data.dir` | `<home>/db/<node>` | data directory override |
| `raft.snapshot-key-bean` | — | property holding the snapshot encryption token; falls back to the corresponding environment variable (e.g. `raft.snapshot-key` → `RAFT_SNAPSHOT_KEY`) |
| `raft-store.log-prefix` / `raft-store.stable-prefix` | `log` / `stable` | key prefixes in the badger store |

All bind ports are adjusted by `node.seq`, so several nodes can share a host.

## Serf CLI (`raftcmd`)

`raftcmd.RaftCommands` adds a `serf` command group to a cligo application
(`join`, `members`, `event`, `info`, `version`, `leave`, `monitor`,
`reachability`, `rtt`, `tags`), talking to the local serf agent over
`serf.rpc-address`:

```go
cligo.Main(
    cligo.Name("myapp"),
    cligo.Beans(raftcmd.RaftCommands...),
    cligo.Beans(servion.RunCommand(raftmod.RaftServices...)),
)
```

```sh
./myapp run
./myapp serf members --detailed
./myapp serf join 10.0.0.5:7946
```
