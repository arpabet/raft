/*
 * Copyright (c) 2025-2026 Karagatan LLC.
 * SPDX-License-Identifier: BUSL-1.1
 */

package raftcmd

import (
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/serf/client"
	"go.arpabet.com/cligo"
	"golang.org/x/xerrors"
)

type serfJoinCommand struct {
	Parent cligo.CliGroup `cli:"group=serf"`
	Prov   ClientProvider `inject:""`

	Address string `cli:"argument=address"`
	Replay  bool   `cli:"option=replay,default=false,help=Replay past user events."`
}

func SerfJoinCommand() cligo.CliCommand {
	return &serfJoinCommand{}
}

func (t *serfJoinCommand) Command() string {
	return "join"
}

func (t *serfJoinCommand) Help() (string, string) {
	return "Tell Serf agent to join cluster.",
		`Tells a running Serf agent to join the cluster by specifying at least one
existing member. ADDRESS accepts a comma-separated list of addresses.`
}

func (t *serfJoinCommand) Run(ctx context.Context) error {

	var nodes []string
	for _, addr := range strings.Split(t.Address, ",") {
		if addr = strings.TrimSpace(addr); addr != "" {
			nodes = append(nodes, addr)
		}
	}
	if len(nodes) == 0 {
		return xerrors.New("at least one address to join must be specified")
	}

	return t.Prov.DoWithClient(func(cli *client.RPCClient) error {
		n, err := cli.Join(nodes, t.Replay)
		if err != nil {
			return xerrors.Errorf("joining the cluster '%+v', %v", nodes, err)
		}
		fmt.Printf("Successfully joined cluster by contacting %d nodes.\n", n)
		return nil
	})
}
