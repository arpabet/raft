/*
 * Copyright (c) 2025-2026 Karagatan LLC.
 * SPDX-License-Identifier: BUSL-1.1
 */

package raftcmd

import (
	"flag"
	"strings"

	"github.com/hashicorp/serf/client"
	"golang.org/x/xerrors"
)

type serfLeaveCommand struct {
}

func SerfLeaveCommand() SerfCommand {
	return &serfLeaveCommand{}
}

func (t serfLeaveCommand) Help() string {
	helpText := `
Usage: serf leave [name]

  Causes the agent to leave the Serf cluster.

  -force           Forces a member of a Serf cluster to enter the "left" state
  -prune           Remove agent forcibly from list of members.
`
	return strings.TrimSpace(helpText)
}

func (t serfLeaveCommand) SubCommand() string {
	return "leave"
}

func (t serfLeaveCommand) Synopsis() string {
	return "Leaves the Serf cluster"
}

func (t serfLeaveCommand) Run(prov ClientProvider, args []string) error {

	var force bool
	var prune bool

	cmdFlags := flag.NewFlagSet("leave", flag.ContinueOnError)
	cmdFlags.Usage = func() { println(t.Help()) }
	cmdFlags.BoolVar(&force, "force", false, "forces a member leave")
	cmdFlags.BoolVar(&prune, "prune", false, "remove forcibly from list")

	if err := cmdFlags.Parse(args); err != nil {
		return err
	}

	nodes := cmdFlags.Args()

	return prov.DoWithClient(func(cli *client.RPCClient) error {
		return t.doRun(cli, nodes, force, prune)
	})
}

func (t serfLeaveCommand) doRun(client *client.RPCClient, nodes []string, force, prune bool) error {

	if force {

		if len(nodes) != 1 {
			return xerrors.Errorf("A node name must be specified to force leave.")
		}

		if prune {
			err := client.ForceLeavePrune(nodes[0])
			if err != nil {
				return xerrors.Errorf("force leaving with prune, %v", err)
			}
		} else {
			err := client.ForceLeave(nodes[0])
			if err != nil {
				return xerrors.Errorf("force leaving, %v", err)
			}
		}

		println("Force leave complete")
		return nil

	} else {
		if err := client.Leave(); err != nil {
			return xerrors.Errorf("error leaving, %v", err)
		}

		println("Graceful leave complete")
		return nil
	}

}
