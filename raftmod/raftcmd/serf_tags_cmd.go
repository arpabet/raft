/*
 * Copyright (c) 2025-2026 Karagatan LLC.
 * SPDX-License-Identifier: BUSL-1.1
 */

package raftcmd

import (
	"context"

	"github.com/hashicorp/serf/client"
	"github.com/hashicorp/serf/cmd/serf/command/agent"
	"go.arpabet.com/cligo"
	"golang.org/x/xerrors"
)

type serfTagsCommand struct {
	Parent cligo.CliGroup `cli:"group=serf"`
	Prov   ClientProvider `inject:""`

	Set   []string `cli:"option=set,help=Creates or modifies the value of a tag, specified as key=value; repeatable."`
	Unset []string `cli:"option=unset,help=Removes a tag, if present; repeatable."`
}

func SerfTagsCommand() cligo.CliCommand {
	return &serfTagsCommand{}
}

func (t *serfTagsCommand) Command() string {
	return "tags"
}

func (t *serfTagsCommand) Help() (string, string) {
	return "Modify tags of a running Serf agent.", ""
}

func (t *serfTagsCommand) Run(ctx context.Context) error {

	if len(t.Set) == 0 && len(t.Unset) == 0 {
		return xerrors.New("at least one of --set or --unset must be specified")
	}

	tags, err := agent.UnmarshalTags(t.Set)
	if err != nil {
		return err
	}

	err = t.Prov.DoWithClient(func(cli *client.RPCClient) error {
		return cli.UpdateTags(tags, t.Unset)
	})
	if err != nil {
		return xerrors.Errorf("setting tags '%s', %v", tags, err)
	}

	println("Successfully updated agent tags")
	return nil
}
