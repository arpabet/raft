/*
 * Copyright (c) 2025-2026 Karagatan LLC.
 * SPDX-License-Identifier: BUSL-1.1
 */

package raftcmd

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/hashicorp/serf/client"
	"go.arpabet.com/cligo"
	"golang.org/x/xerrors"
)

type serfInfoCommand struct {
	Parent cligo.CliGroup `cli:"group=serf"`
	Prov   ClientProvider `inject:""`

	Format string `cli:"option=format,default=text,help=Output format: 'json' or 'text'."`
}

func SerfInfoCommand() cligo.CliCommand {
	return &serfInfoCommand{}
}

func (t *serfInfoCommand) Command() string {
	return "info"
}

func (t *serfInfoCommand) Help() (string, string) {
	return "Provides debugging information for operators.", ""
}

func (t *serfInfoCommand) Run(ctx context.Context) error {
	return t.Prov.DoWithClient(func(cli *client.RPCClient) error {
		stats, err := cli.Stats()
		if err != nil {
			return err
		}
		output, err := formatOutput(statsString(stats), t.Format)
		if err != nil {
			return xerrors.Errorf("encoding error: %s", err)
		}
		println(string(output))
		return nil
	})
}

type statsString map[string]map[string]string

func (s statsString) String() string {
	var buf bytes.Buffer

	// Get the keys in sorted order
	keys := make([]string, 0, len(s))
	for key := range s {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	// Iterate over each top-level key
	for _, key := range keys {
		buf.WriteString(key + ":\n")

		// Sort the sub-keys
		subvals := s[key]
		subkeys := make([]string, 0, len(subvals))
		for k := range subvals {
			subkeys = append(subkeys, k)
		}
		sort.Strings(subkeys)

		// Iterate over the subkeys
		for _, subkey := range subkeys {
			val := subvals[subkey]
			buf.WriteString(fmt.Sprintf("\t%s = %s\n", subkey, val))
		}
	}
	return buf.String()
}

func formatOutput(data interface{}, format string) ([]byte, error) {
	var out string

	switch format {

	case "json":
		jsonBin, err := json.MarshalIndent(data, "", "  ")
		if err != nil {
			return nil, err
		}
		out = string(jsonBin)

	case "text":
		if s, ok := data.(fmt.Stringer); ok {
			out = s.String()
		} else {
			out = fmt.Sprint(data)
		}

	default:
		return nil, xerrors.Errorf("invalid output format \"%s\"", format)

	}
	return []byte(strings.TrimSpace(out)), nil
}
