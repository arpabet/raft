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

type serfEventCommand struct {
	Parent cligo.CliGroup `cli:"group=serf"`
	Prov   ClientProvider `inject:""`

	Name     string `cli:"argument=name"`
	Payload  string `cli:"argument=payload,default="`
	Coalesce bool   `cli:"option=coalesce,default=true,help=Whether repeated events of the same name within a short period of time are coalesced."`
}

func SerfEventCommand() cligo.CliCommand {
	return &serfEventCommand{}
}

func (t *serfEventCommand) Command() string {
	return "event"
}

func (t *serfEventCommand) Help() (string, string) {
	return "Emit a custom event through the Serf cluster.",
		`Dispatches a custom event across the Serf cluster with the given NAME and
optional PAYLOAD.`
}

func (t *serfEventCommand) Run(ctx context.Context) error {
	return t.Prov.DoWithClient(func(cli *client.RPCClient) error {
		if err := cli.UserEvent(t.Name, []byte(t.Payload), t.Coalesce); err != nil {
			return xerrors.Errorf("sending event '%s', %v", t.Name, err)
		}
		fmt.Printf("Event '%s' dispatched! Coalescing enabled: %#v\n", t.Name, t.Coalesce)
		return nil
	})
}
