/*
 * Copyright (c) 2025-2026 Karagatan LLC.
 * SPDX-License-Identifier: BUSL-1.1
 */

package raftcmd

import (
	"context"

	"github.com/hashicorp/serf/client"
	"go.arpabet.com/cligo"
	"golang.org/x/xerrors"
)

type serfLeaveCommand struct {
	Parent cligo.CliGroup `cli:"group=serf"`
	Prov   ClientProvider `inject:""`

	Node  string `cli:"argument=node,default="`
	Force bool   `cli:"option=force,default=false,help=Forces the given member of the Serf cluster to enter the 'left' state."`
	Prune bool   `cli:"option=prune,default=false,help=Remove agent forcibly from list of members."`
}

func SerfLeaveCommand() cligo.CliCommand {
	return &serfLeaveCommand{}
}

func (t *serfLeaveCommand) Command() string {
	return "leave"
}

func (t *serfLeaveCommand) Help() (string, string) {
	return "Leaves the Serf cluster.",
		`Causes the local agent to gracefully leave the Serf cluster. With --force,
NODE names a member that is forced to enter the 'left' state.`
}

func (t *serfLeaveCommand) Run(ctx context.Context) error {
	return t.Prov.DoWithClient(func(cli *client.RPCClient) error {

		if t.Force {

			if t.Node == "" {
				return xerrors.New("a node name must be specified to force leave")
			}

			if t.Prune {
				if err := cli.ForceLeavePrune(t.Node); err != nil {
					return xerrors.Errorf("force leaving with prune, %v", err)
				}
			} else {
				if err := cli.ForceLeave(t.Node); err != nil {
					return xerrors.Errorf("force leaving, %v", err)
				}
			}

			println("Force leave complete")
			return nil
		}

		if err := cli.Leave(); err != nil {
			return xerrors.Errorf("error leaving, %v", err)
		}

		println("Graceful leave complete")
		return nil
	})
}
