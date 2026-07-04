/*
 * Copyright (c) 2025-2026 Karagatan LLC.
 * SPDX-License-Identifier: BUSL-1.1
 */

package raftcmd

import (
	"context"
	"fmt"

	"github.com/hashicorp/serf/serf"
	"github.com/hashicorp/serf/version"
	"go.arpabet.com/cligo"
)

type serfVersionCommand struct {
	Parent cligo.CliGroup `cli:"group=serf"`
}

func SerfVersionCommand() cligo.CliCommand {
	return &serfVersionCommand{}
}

func (t *serfVersionCommand) Command() string {
	return "version"
}

func (t *serfVersionCommand) Help() (string, string) {
	return "Prints the Serf version.", ""
}

func (t *serfVersionCommand) Run(ctx context.Context) error {
	println(version.GetHumanVersion())
	fmt.Printf("Agent Protocol: %d (Understands back to: %d)\n",
		serf.ProtocolVersionMax, serf.ProtocolVersionMin)
	return nil
}
