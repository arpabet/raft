/*
 * Copyright (c) 2025-2026 Karagatan LLC.
 * SPDX-License-Identifier: BUSL-1.1
 */

package raftcmd

import (
	"context"
	"fmt"

	"github.com/hashicorp/serf/client"
	"go.arpabet.com/cligo"
	"golang.org/x/xerrors"
)

type serfRttCommand struct {
	Parent cligo.CliGroup `cli:"group=serf"`
	Prov   ClientProvider `inject:""`

	Node1 string `cli:"argument=node1"`
	Node2 string `cli:"argument=node2,default="`
}

func SerfRttCommand() cligo.CliCommand {
	return &serfRttCommand{}
}

func (t *serfRttCommand) Command() string {
	return "rtt"
}

func (t *serfRttCommand) Help() (string, string) {
	return "Estimates network round trip time between nodes.",
		`Estimates the round trip time between two nodes using Serf's network
coordinate model of the cluster. At least one node name is required. If the
second node name isn't given, it is set to the agent's node name. Note that
these are node names as known to Serf as "serf members" would show, not IP
addresses.`
}

func (t *serfRttCommand) Run(ctx context.Context) error {
	return t.Prov.DoWithClient(func(cli *client.RPCClient) error {

		node1, node2 := t.Node1, t.Node2

		if node2 == "" {
			stats, err := cli.Stats()
			if err != nil {
				return xerrors.Errorf("querying agent, %v", err)
			}
			node2 = stats["agent"]["name"]
		}

		// Get the coordinates.
		coord1, err := cli.GetCoordinate(node1)
		if err != nil {
			return xerrors.Errorf("getting coordinates, %v", err)
		}

		if coord1 == nil {
			return xerrors.Errorf("could not find a coordinate for node %q", node1)
		}

		coord2, err := cli.GetCoordinate(node2)
		if err != nil {
			return xerrors.Errorf("getting coordinates, %v", err)
		}

		if coord2 == nil {
			return xerrors.Errorf("could not find a coordinate for node %q", node2)
		}

		// Report the round trip time.
		dist := fmt.Sprintf("%.3f ms", coord1.DistanceTo(coord2).Seconds()*1000.0)
		fmt.Printf("Estimated %s <-> %s rtt: %s\n", node1, node2, dist)
		return nil
	})
}
