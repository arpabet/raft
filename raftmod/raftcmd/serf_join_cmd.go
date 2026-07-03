/*
 * Copyright (c) 2025-2026 Karagatan LLC.
 * SPDX-License-Identifier: BUSL-1.1
 */

package raftcmd

import (
	"flag"
	"fmt"
	"strings"

	"github.com/hashicorp/serf/client"
	"golang.org/x/xerrors"
)

type serfJoinCommand struct {
}

func SerfJoinCommand() SerfCommand {
	return &serfJoinCommand{}
}

func (t serfJoinCommand) Help() string {
	helpText := `
Usage: serf join [options] address ...

  Tells a running Serf agent (with "serf agent") to join the cluster
  by specifying at least one existing member.

Options:

  -replay                   Replay past user events.
`
	return strings.TrimSpace(helpText)
}

func (t serfJoinCommand) SubCommand() string {
	return "join"
}

func (t serfJoinCommand) Synopsis() string {
	return "Tell Serf agent to join cluster"
}

func (t serfJoinCommand) Run(prov ClientProvider, args []string) error {
	var replayEvents bool

	cmdFlags := flag.NewFlagSet("join", flag.ContinueOnError)
	cmdFlags.Usage = func() { println(t.Help()) }
	cmdFlags.BoolVar(&replayEvents, "replay", false, "replay past user events")

	if err := cmdFlags.Parse(args); err != nil {
		return err
	}

	nodes := cmdFlags.Args()
	if len(nodes) == 0 {
		return xerrors.Errorf("at least one address to join must be specified\n%s", t.Help())
	}

	return prov.DoWithClient(func(cli *client.RPCClient) error {
		return t.doRun(cli, nodes, replayEvents)
	})
}

func (t serfJoinCommand) doRun(client *client.RPCClient, nodes []string, replayEvents bool) error {

	n, err := client.Join(nodes, replayEvents)
	if err != nil {
		return xerrors.Errorf("joining the cluster '%+v', %v", nodes, err)
	}

	fmt.Printf("Successfully joined cluster by contacting %d nodes.\n", n)
	return nil
}
