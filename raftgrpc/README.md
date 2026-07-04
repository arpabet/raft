# raftgrpc

gRPC management surface for the Raft layer: implements the `raftpb.RaftService`
(`Bootstrap` / `Join` / `GetConfiguration` / `ApplyCommand` / `Recover`) and the
`raft` CLI command (`config` / `join` / `bootstrap`).

Companion to [`raftmod`](../raftmod), which provides the node-to-node consensus
transport and the FSM/log/snapshot stores. `raftmod` wires the cluster; `raftgrpc`
exposes the leader-aware control API over the application's gRPC server.

## Beans

- `raftgrpc.RaftGrpcServer()` — server bean (`raftapi.RaftGrpcServer`); registers
  the `raftpb.RaftService` on the injected `*grpc.Server`. Add to the API
  `ServerRole` alongside `raftmod.RaftServices`.
- `raftgrpc.Commands()` — the `raft` cligo command group (`config` / `join` /
  `bootstrap`); talks to a running node's control endpoint configured by the
  `raft-grpc-client.address` property.

History: ported from `github.com/openraft/raftgrpc` (formerly
`github.com/codeallergy/raftgrpc`) into the arpabet namespace.
