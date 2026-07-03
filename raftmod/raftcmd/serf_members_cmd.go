/*
 * Copyright (c) 2025-2026 Karagatan LLC.
 * SPDX-License-Identifier: BUSL-1.1
 */

package raftcmd

import (
	"flag"
	"fmt"
	"net"
	"sort"
	"strings"

	"github.com/hashicorp/serf/client"
	"github.com/hashicorp/serf/cmd/serf/command/agent"
	"github.com/ryanuber/columnize"
	"golang.org/x/xerrors"
)

type MemberOutput struct {
	detail bool
	Name   string            `json:"name"`
	Addr   string            `json:"addr"`
	Port   uint16            `json:"port"`
	Tags   map[string]string `json:"tags"`
	Status string            `json:"status"`
	Proto  map[string]uint8  `json:"protocol"`
}

type MembersContainer struct {
	Members []*MemberOutput `json:"members"`
}

type serfMembersCommand struct {
}

func SerfMembersCommand() SerfCommand {
	return &serfMembersCommand{}
}

func (t serfMembersCommand) Help() string {
	helpText := `
Usage: serf members [options]

  Outputs the members of a running Serf agent.

Options:

  -detailed                 Additional information such as protocol versions
                            will be shown (only affects text output format).

  -format                   If provided, output is returned in the specified
                            format. Valid formats are 'json', and 'text' (default)

  -name=<regexp>            If provided, only members matching the regexp are
                            returned. The regexp is anchored at the start and end,
                            and must be a full match.

  -status=<regexp>          If provided, output is filtered to only nodes matching
                            the regular expression for status

  -tag <key>=<regexp>       If provided, output is filtered to only nodes with the
                            tag <key> with value matching the regular expression.
                            tag can be specified multiple times to filter on
                            multiple keys. The regexp is anchored at the start and end,
                            and must be a full match.

`
	return strings.TrimSpace(helpText)
}

func (t serfMembersCommand) SubCommand() string {
	return "members"
}

func (t serfMembersCommand) Synopsis() string {
	return "Lists the members of a Serf cluster"
}

func (t serfMembersCommand) Run(prov ClientProvider, args []string) error {

	var detailed bool
	var statusFilter, nameFilter, format string
	var tags []string
	cmdFlags := flag.NewFlagSet("members", flag.ContinueOnError)
	cmdFlags.Usage = func() { println(t.Help()) }
	cmdFlags.BoolVar(&detailed, "detailed", false, "detailed output")
	cmdFlags.StringVar(&statusFilter, "status", "", "status filter")
	cmdFlags.StringVar(&format, "format", "text", "output format")
	cmdFlags.Var((*agent.AppendSliceValue)(&tags), "tag", "tag filter")
	cmdFlags.StringVar(&nameFilter, "name", "", "name filter")

	if err := cmdFlags.Parse(args); err != nil {
		return err
	}

	reqTags, err := agent.UnmarshalTags(tags)
	if err != nil {
		return xerrors.Errorf("unmarshal tags, %v", err)
	}

	return prov.DoWithClient(func(cli *client.RPCClient) error {
		return t.doRun(cli, reqTags, statusFilter, nameFilter, format, detailed)
	})
}

func (t serfMembersCommand) doRun(client *client.RPCClient, tags map[string]string, statusFilter, nameFilter, format string, detailed bool) error {

	members, err := client.MembersFiltered(tags, statusFilter, nameFilter)
	if err != nil {
		return xerrors.Errorf("retrieving members, %v", err)
	}

	container := parseMembers(members, detailed)

	output, err := formatOutput(container, format)
	if err != nil {
		return xerrors.Errorf("encoding error, %v", err)
	}

	println(string(output))
	return nil
}

func parseMembers(members []client.Member, detailed bool) MembersContainer {

	result := MembersContainer{}

	for _, member := range members {
		addr := net.TCPAddr{IP: member.Addr, Port: int(member.Port)}

		result.Members = append(result.Members, &MemberOutput{
			detail: detailed,
			Name:   member.Name,
			Addr:   addr.String(),
			Port:   member.Port,
			Tags:   member.Tags,
			Status: member.Status,
			Proto: map[string]uint8{
				"min":     member.DelegateMin,
				"max":     member.DelegateMax,
				"version": member.DelegateCur,
			},
		})
	}

	return result
}

func (t MembersContainer) String() string {
	var result []string
	for _, member := range t.Members {
		listOfTags := agent.MarshalTags(member.Tags)
		sort.Strings(listOfTags)
		tags := strings.Join(listOfTags, ",")
		line := fmt.Sprintf("%s|%s|%s|%s",
			member.Name, member.Addr, member.Status, tags)
		if member.detail {
			line += fmt.Sprintf(
				"|Protocol Version: %d|Available Protocol Range: [%d, %d]",
				member.Proto["version"], member.Proto["min"], member.Proto["max"])
		}
		result = append(result, line)
	}
	return columnize.SimpleFormat(result)
}
