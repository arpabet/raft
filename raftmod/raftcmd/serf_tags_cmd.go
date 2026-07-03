/*
 * Copyright (c) 2025-2026 Karagatan LLC.
 * SPDX-License-Identifier: BUSL-1.1
 */

package raftcmd

import (
	"flag"
	"strings"

	"github.com/hashicorp/serf/client"
	"github.com/hashicorp/serf/cmd/serf/command/agent"
	"go.arpabet.com/sprint"
	"golang.org/x/xerrors"
)

type serfTagsCommand struct {
	Application sprint.Application `inject:""`
}

func SerfTagsCommand() SerfCommand {
	return &serfTagsCommand{}
}

func (t serfTagsCommand) Help() string {
	helpText := `
Usage: serf tags [options] ...

  Modifies tags on a running Serf agent.

Options:

  -set key=value            Creates or modifies the value of a tag
  -unset key               Removes a tag, if present
`
	return strings.TrimSpace(helpText)
}

func (t serfTagsCommand) SubCommand() string {
	return "tags"
}

func (t serfTagsCommand) Synopsis() string {
	return "Modify tags of a running Serf agent"
}

func (t serfTagsCommand) Run(prov ClientProvider, args []string) error {
	var tagPairs []string
	var delTags []string
	cmdFlags := flag.NewFlagSet("tags", flag.ContinueOnError)
	cmdFlags.Usage = func() { println(t.Help()) }
	cmdFlags.Var((*agent.AppendSliceValue)(&tagPairs), "set",
		"tag pairs, specified as key=value")
	cmdFlags.Var((*agent.AppendSliceValue)(&delTags), "unset",
		"tag keys to unset")

	if err := cmdFlags.Parse(args); err != nil {
		return err
	}

	if len(tagPairs) == 0 && len(delTags) == 0 {
		println(t.Help())
		return nil
	}

	tags, err := agent.UnmarshalTags(tagPairs)
	if err != nil {
		return err
	}

	err = prov.DoWithClient(func(cli *client.RPCClient) error {
		return cli.UpdateTags(tags, delTags)
	})
	if err != nil {
		return xerrors.Errorf("setting tags '%s', %v", tags, err)
	}

	println("Successfully updated agent tags")
	return nil
}
