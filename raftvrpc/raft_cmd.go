/*
 * Copyright (c) 2025-2026 Karagatan LLC.
 * SPDX-License-Identifier: BUSL-1.1
 */

package raftvrpc

import (
	"context"
	"fmt"
	"strings"

	"go.arpabet.com/raft/raftpb"
	"go.arpabet.com/sprint"
	"go.arpabet.com/value-rpc/valueclient"
	"go.uber.org/zap"
	"golang.org/x/xerrors"
)

// raftCommand is the value-rpc analogue of raftgrpc's `raft` CLI. It dials the
// control endpoint at raft-vrpc-client.address and issues config/join/bootstrap.
type raftCommand struct {
	Application sprint.Application `inject:""`
	Log         *zap.Logger        `inject:""`

	Address string `value:"raft-vrpc-client.address,default="`
}

func RaftCommand() sprint.Command {
	return &raftCommand{}
}

func (t *raftCommand) BeanName() string { return "raft" }

func (t *raftCommand) Synopsis() string {
	return "raft commands [config,join,bootstrap]"
}

func (t *raftCommand) Help() string {
	helpText := `
Usage: ./%s raft [command]

	Manages the raft cluster over value-rpc.

Commands:

  config                   Returns the configuration of the existing Raft cluster.

  join    node_id node_addr  Joins the given node to the cluster.

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
			return t.doConfig(false)
		case "join":
			return t.doJoin(args)
		case "bootstrap":
			return t.doBootstrap()
		}
		return xerrors.Errorf("Usage: ./%s raft [config,join,bootstrap] [args]", t.Application.Executable())
	}
	return t.doConfig(true)
}

func (t *raftCommand) withClient(cb func(cli valueclient.Client) error) error {
	if t.Address == "" {
		return xerrors.New("empty property 'raft-vrpc-client.address'")
	}
	cli := valueclient.NewClient(t.Address, "", valueclient.WithLogger(t.Log))
	if err := cli.Connect(); err != nil {
		return xerrors.Errorf("connect to %s: %v", t.Address, err)
	}
	defer cli.Close()
	return cb(cli)
}

func (t *raftCommand) doConfig(printState bool) error {
	return t.withClient(func(cli valueclient.Client) error {
		resp, err := CallGetConfiguration(context.Background(), cli)
		if err != nil {
			return err
		}
		if printState {
			fmt.Println(resp.State)
		} else {
			fmt.Println(resp.String())
		}
		return nil
	})
}

func (t *raftCommand) doJoin(args []string) error {
	if len(args) < 2 {
		return xerrors.Errorf("Usage: ./%s raft join node_id node_addr", t.Application.Executable())
	}
	node, address := args[0], args[1]
	fmt.Printf("Join remote node '%s' '%s' to us\n", node, address)
	return t.withClient(func(cli valueclient.Client) error {
		if _, err := CallJoin(context.Background(), cli, &raftpb.RaftNode{NodeId: node, NodeAddr: address}); err != nil {
			return err
		}
		fmt.Println("Done")
		return nil
	})
}

func (t *raftCommand) doBootstrap() error {
	return t.withClient(func(cli valueclient.Client) error {
		if _, err := CallBootstrap(context.Background(), cli); err != nil {
			return err
		}
		fmt.Println("Done")
		return nil
	})
}
