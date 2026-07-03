/*
 * Copyright (c) 2025-2026 Karagatan LLC.
 * SPDX-License-Identifier: BUSL-1.1
 */

package raftgrpc

import (
	"context"
	"fmt"
	"strings"

	"go.arpabet.com/glue"
	"go.arpabet.com/raft/raftpb"
	"go.arpabet.com/sprint"
	"golang.org/x/xerrors"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/emptypb"
)

type raftCommand struct {
	Application sprint.Application `inject:""`
	Context     glue.Container     `inject:""`

	ScannerName string `value:"raft-client.scanner-name,default=raft"`
}

func RaftCommand() sprint.Command {
	return &raftCommand{}
}

func (t *raftCommand) BeanName() string {
	return "raft"
}

func (t *raftCommand) Help() string {
	helpText := `
Usage: ./%s raft [command]

	Provides management functionality for the internal storage(s).

Commands:

  config                   Returns the configuration of the existing Raft cluster.

  join                     Joins the cluster by provided node.

  bootstrap                Bootstrap the new raft cluster.

`
	return strings.TrimSpace(fmt.Sprintf(helpText, t.Application.Executable()))
}

func (t *raftCommand) Run(args []string) error {

	if len(args) >= 1 {
		cmd := args[0]
		args = args[1:]
		switch cmd {
		case "config":
			return t.doConfig(args, false)
		case "join":
			return t.doJoin(args)
		case "bootstrap":
			return t.doBootstrap(args)
		}
		return xerrors.Errorf("Usage: ./%s raft [config,join,bootstrap] [args]", t.Application.Executable())
	}

	return t.doConfig(args, true)
}

func (t *raftCommand) Synopsis() string {
	return "raft commands [config,join,bootstrap]"
}

func (t *raftCommand) doConfig(args []string, printState bool) error {

	return sprint.DoWithClientConn(t.Context, t.ScannerName, func(conn *grpc.ClientConn) error {
		client := raftpb.NewRaftServiceClient(conn)
		resp, err := client.GetConfiguration(context.Background(), &emptypb.Empty{})
		if err != nil {
			return err
		}
		if printState {
			println(resp.State)
		} else {
			println(resp.String())
		}
		return nil
	})
}

func (t *raftCommand) doJoin(args []string) error {

	if len(args) < 2 {
		return xerrors.Errorf("Usage: ./%s raft join node_id node_addr", t.Application.Executable())
	}

	node := args[0]
	address := args[1]
	args = args[2:]

	fmt.Printf("Join remote node '%s' '%s' to us\n", node, address)

	return sprint.DoWithClientConn(t.Context, t.ScannerName, func(conn *grpc.ClientConn) error {
		client := raftpb.NewRaftServiceClient(conn)

		_, err := client.Join(context.Background(), &raftpb.RaftNode{
			NodeId:   node,
			NodeAddr: address,
		})

		if err != nil {
			return err
		}

		fmt.Println("Done")
		return nil
	})

}

func (t raftCommand) doBootstrap(args []string) error {

	return sprint.DoWithClientConn(t.Context, t.ScannerName, func(conn *grpc.ClientConn) error {
		client := raftpb.NewRaftServiceClient(conn)
		_, err := client.Bootstrap(context.Background(), &emptypb.Empty{})
		if err != nil {
			return err
		}

		fmt.Println("Done")
		return nil
	})

}
