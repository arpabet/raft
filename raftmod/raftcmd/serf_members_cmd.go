/*
 * Copyright (c) 2025-2026 Karagatan LLC.
 * SPDX-License-Identifier: BUSL-1.1
 */

package raftcmd

import (
	"context"
	"fmt"
	"net"
	"sort"
	"strings"

	"github.com/hashicorp/serf/client"
	"github.com/hashicorp/serf/cmd/serf/command/agent"
	"github.com/ryanuber/columnize"
	"go.arpabet.com/cligo"
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
	Parent cligo.CliGroup `cli:"group=serf"`
	Prov   ClientProvider `inject:""`

	Detailed bool     `cli:"option=detailed,default=false,help=Show additional information such as protocol versions (text format only)."`
	Format   string   `cli:"option=format,default=text,help=Output format: 'json' or 'text'."`
	Name     string   `cli:"option=name,default=,help=Show only members matching the anchored regexp."`
	Status   string   `cli:"option=status,default=,help=Show only members with status matching the regexp."`
	Tags     []string `cli:"option=tag,help=Show only members with tag key=regexp; repeatable."`
}

func SerfMembersCommand() cligo.CliCommand {
	return &serfMembersCommand{}
}

func (t *serfMembersCommand) Command() string {
	return "members"
}

func (t *serfMembersCommand) Help() (string, string) {
	return "Lists the members of a Serf cluster.",
		`Outputs the members of a running Serf agent, optionally filtered by name,
status and tags.`
}

func (t *serfMembersCommand) Run(ctx context.Context) error {

	reqTags, err := agent.UnmarshalTags(t.Tags)
	if err != nil {
		return xerrors.Errorf("unmarshal tags, %v", err)
	}

	return t.Prov.DoWithClient(func(cli *client.RPCClient) error {

		members, err := cli.MembersFiltered(reqTags, t.Status, t.Name)
		if err != nil {
			return xerrors.Errorf("retrieving members, %v", err)
		}

		container := parseMembers(members, t.Detailed)

		output, err := formatOutput(container, t.Format)
		if err != nil {
			return xerrors.Errorf("encoding error, %v", err)
		}

		println(string(output))
		return nil
	})
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
