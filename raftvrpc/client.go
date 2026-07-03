/*
 * Copyright (c) 2025-2026 Karagatan LLC.
 * SPDX-License-Identifier: BUSL-1.1
 */

package raftvrpc

import (
	"context"

	"go.arpabet.com/raft/raftpb"
	"go.arpabet.com/value-rpc/valueclient"
)

// Typed client helpers over a value-rpc client connected to a raft control
// service. A RaftClientPool's GetAPIConn returns a valueclient.Client that these
// call. They pair 1:1 with the server handlers registered by Register.

func CallBootstrap(ctx context.Context, cli valueclient.Client) (*raftpb.Status, error) {
	return valueclient.CallUnary(ctx, cli, FnBootstrap, struct{}{}, emptyCodec, statusCodec)
}

func CallJoin(ctx context.Context, cli valueclient.Client, node *raftpb.RaftNode) (*raftpb.Status, error) {
	return valueclient.CallUnary(ctx, cli, FnJoin, node, raftNodeCodec, statusCodec)
}

func CallGetConfiguration(ctx context.Context, cli valueclient.Client) (*raftpb.RaftConfiguration, error) {
	return valueclient.CallUnary(ctx, cli, FnGetConfiguration, struct{}{}, emptyCodec, raftConfigurationCodec)
}

func CallApplyCommand(ctx context.Context, cli valueclient.Client, cmd *raftpb.Command) (*raftpb.Status, error) {
	return valueclient.CallUnary(ctx, cli, FnApplyCommand, cmd, commandCodec, statusCodec)
}
