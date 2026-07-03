/*
 * Copyright (c) 2025-2026 Karagatan LLC.
 * SPDX-License-Identifier: BUSL-1.1
 */

package raftcmd

import (
	"fmt"
	"sort"
	"strings"

	"github.com/hashicorp/serf/client"
	"github.com/ryanuber/columnize"
	"go.arpabet.com/glue"
	"go.arpabet.com/raft/raftmod"
	"go.arpabet.com/sprint"
	"golang.org/x/xerrors"
)

type serfCommand struct {
	Application      sprint.Application      `inject:""`
	ApplicationFlags sprint.ApplicationFlags `inject:""`
	Context          glue.Container          `inject:""`

	// keep it sorted by SubCommand()
	SerfCommands []SerfCommand `inject:""`

	SerfAddress string `value:"serf-server.rpc-address,default=127.0.0.1:8700"`
	SerfToken   string `value:"serf-server.rpc-auth,default="`
}

func SerfCommands() sprint.Command {
	return &serfCommand{}
}

func (t *serfCommand) BeanName() string {
	return "serf"
}

func (t *serfCommand) PostConstruct() error {
	sort.Slice(t.SerfCommands, func(i, j int) bool {
		left, right := t.SerfCommands[i].SubCommand(), t.SerfCommands[j].SubCommand()
		return left < right
	})
	return nil
}

func (t *serfCommand) findCommand(key string) (SerfCommand, bool) {
	n := len(t.SerfCommands)
	i := sort.Search(n, func(i int) bool {
		return t.SerfCommands[i].SubCommand() >= key
	})
	if i == n {
		return nil, false
	} else if t.SerfCommands[i].SubCommand() == key {
		return t.SerfCommands[i], true
	} else {
		return nil, false
	}
}

func (t *serfCommand) Help() string {
	helpText := `
Usage: ./%s serf [command]

   Provides management functionality for the Serf (gossip) server.

Commands:

%s
`
	var lines []string
	for _, cmd := range t.SerfCommands {
		lines = append(lines, fmt.Sprintf("%s\t%s", cmd.SubCommand(), cmd.Synopsis()))
	}
	commands := columnize.Format(lines, &columnize.Config{
		Delim:  "\t",
		Glue:   "  ",
		Prefix: "   ",
	})

	return strings.TrimSpace(fmt.Sprintf(helpText, t.Application.Executable(), commands))
}

func (t *serfCommand) Run(args []string) error {

	if len(args) == 0 {
		println(t.Help())
		return nil
	}

	cmd := args[0]
	args = args[1:]

	if handler, ok := t.findCommand(cmd); ok {
		return t.doRun(handler, args)
	} else {
		return xerrors.Errorf("unknown sub command '%s' for serf, Usage: ./%s serf [%s]",
			cmd, t.Application.Name(), t.subCommands())
	}
}

func (t *serfCommand) doRun(handler SerfCommand, args []string) (err error) {
	addr := t.getConnectAddress(t.SerfAddress)

	tcpAddr, err := raftmod.ParseAndAdjustTCPAddr(addr, t.ApplicationFlags.Node())
	if err != nil {
		return err
	}
	addr = fmt.Sprintf("%s:%d", tcpAddr.IP.String(), tcpAddr.Port)

	prov := clientProviderImpl{Addr: addr, AuthKey: t.SerfToken}
	err = handler.Run(prov, args)
	if err != nil {
		return xerrors.Errorf("connect self client '%s', %v", addr, err)
	}
	return nil
}

type clientProviderImpl struct {
	Addr    string
	AuthKey string
}

func (t clientProviderImpl) DoWithClient(cb func(cli *client.RPCClient) error) error {
	config := client.Config{Addr: t.Addr, AuthKey: t.AuthKey}
	cli, err := client.ClientFromConfig(&config)
	if err != nil {
		return xerrors.Errorf("connecting to Serf agent, %v", err)
	}
	defer cli.Close()
	return cb(cli)
}

func (t *serfCommand) subCommands() string {
	var sub []string
	for _, cmd := range t.SerfCommands {
		sub = append(sub, cmd.SubCommand())
	}
	return strings.Join(sub, ",")
}

func (t *serfCommand) Synopsis() string {

	var sub []string
	for _, cmd := range t.SerfCommands {
		sub = append(sub, cmd.SubCommand())
	}

	return fmt.Sprintf("serf commands [%s]", t.subCommands())
}

func (t *serfCommand) getConnectAddress(listenAddr string) string {
	if strings.HasPrefix(listenAddr, "0.0.0.0:") {
		return "127.0.0.1" + listenAddr[7:]
	}
	if strings.HasPrefix(listenAddr, ":") {
		return "127.0.0.1" + listenAddr
	}
	return listenAddr
}
