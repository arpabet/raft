/*
 * Copyright (c) 2025-2026 Karagatan LLC.
 * SPDX-License-Identifier: BUSL-1.1
 */

package raftcmd

import (
	"fmt"
	"github.com/hashicorp/serf/serf"
	"github.com/hashicorp/serf/version"
	"go.arpabet.com/sprint"
	"strings"
)

type serfVersionCommand struct {
	Application  sprint.Application   `inject:""`
}

func SerfVersionCommand() SerfCommand {
	return &serfVersionCommand{}
}

func (t serfVersionCommand) Help() string {
	helpText := `
Usage: serf version

  Prints the Serf version.

`
	return strings.TrimSpace(helpText)
}

func (t serfVersionCommand) SubCommand() string {
	return "version"
}

func (t serfVersionCommand) Synopsis() string {
	return "Prints the Serf version"
}

func (t serfVersionCommand) Run(_ ClientProvider, args []string) error {
	println(version.GetHumanVersion())
	fmt.Printf("Agent Protocol: %d (Understands back to: %d)\n",
		serf.ProtocolVersionMax, serf.ProtocolVersionMin)
	return nil
}


