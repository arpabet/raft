/*
 * Copyright (c) 2025-2026 Karagatan LLC.
 * SPDX-License-Identifier: BUSL-1.1
 */

package raftgrpc

import (
	"context"
	"crypto/tls"
	"fmt"

	"go.arpabet.com/cligo"
	"go.arpabet.com/raft/raftpb"
	"go.uber.org/zap"
	"golang.org/x/xerrors"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/protobuf/types/known/emptypb"
)

// RaftGroup groups the gRPC raft control commands under "raft"
// (`app raft config|join|bootstrap`).
type RaftGroup struct{}

func (RaftGroup) Group() string { return "raft" }

func (RaftGroup) Help() (string, string) {
	return "raft cluster management over gRPC", ""
}

// Commands returns the cligo beans for the raft control CLI — the group plus its
// config/join/bootstrap commands. Add them to cligo.Main / cligo.Beans.
func Commands() []interface{} {
	return []interface{}{
		&RaftGroup{},
		&raftConfigCommand{},
		&raftJoinCommand{},
		&raftBootstrapCommand{},
	}
}

func dialControl(address string, log *zap.Logger, cb func(client raftpb.RaftServiceClient) error) error {
	if address == "" {
		return xerrors.New("empty property 'raft-grpc-client.address'")
	}
	conn, err := grpc.NewClient(address, grpc.WithTransportCredentials(
		credentials.NewTLS(&tls.Config{InsecureSkipVerify: true, NextProtos: []string{"h2"}})))
	if err != nil {
		return xerrors.Errorf("dial %s: %v", address, err)
	}
	defer conn.Close()
	return cb(raftpb.NewRaftServiceClient(conn))
}

type raftConfigCommand struct {
	Parent  cligo.CliGroup `cli:"group=raft"`
	Log     *zap.Logger    `inject:""`
	Address string         `value:"raft-grpc-client.address,default="`
}

func (t *raftConfigCommand) Command() string        { return "config" }
func (t *raftConfigCommand) Help() (string, string) { return "show the raft cluster configuration", "" }
func (t *raftConfigCommand) Run(ctx context.Context) error {
	return dialControl(t.Address, t.Log, func(client raftpb.RaftServiceClient) error {
		resp, err := client.GetConfiguration(ctx, &emptypb.Empty{})
		if err != nil {
			return err
		}
		fmt.Println(resp.String())
		return nil
	})
}

type raftJoinCommand struct {
	Parent   cligo.CliGroup `cli:"group=raft"`
	NodeId   string         `cli:"argument=node_id,required"`
	NodeAddr string         `cli:"argument=node_addr,required"`
	Log      *zap.Logger    `inject:""`
	Address  string         `value:"raft-grpc-client.address,default="`
}

func (t *raftJoinCommand) Command() string        { return "join" }
func (t *raftJoinCommand) Help() (string, string) { return "join a node to the cluster", "" }
func (t *raftJoinCommand) Run(ctx context.Context) error {
	fmt.Printf("Join node '%s' at '%s'\n", t.NodeId, t.NodeAddr)
	return dialControl(t.Address, t.Log, func(client raftpb.RaftServiceClient) error {
		if _, err := client.Join(ctx, &raftpb.RaftNode{NodeId: t.NodeId, NodeAddr: t.NodeAddr}); err != nil {
			return err
		}
		fmt.Println("Done")
		return nil
	})
}

type raftBootstrapCommand struct {
	Parent  cligo.CliGroup `cli:"group=raft"`
	Log     *zap.Logger    `inject:""`
	Address string         `value:"raft-grpc-client.address,default="`
}

func (t *raftBootstrapCommand) Command() string        { return "bootstrap" }
func (t *raftBootstrapCommand) Help() (string, string) { return "bootstrap a new raft cluster", "" }
func (t *raftBootstrapCommand) Run(ctx context.Context) error {
	return dialControl(t.Address, t.Log, func(client raftpb.RaftServiceClient) error {
		if _, err := client.Bootstrap(ctx, &emptypb.Empty{}); err != nil {
			return err
		}
		fmt.Println("Done")
		return nil
	})
}
