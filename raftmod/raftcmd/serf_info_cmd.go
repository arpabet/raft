/*
 * Copyright (c) 2025-2026 Karagatan LLC.
 * SPDX-License-Identifier: BUSL-1.1
 */

package raftcmd

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"sort"
	"strings"

	"github.com/hashicorp/serf/client"
	"golang.org/x/xerrors"
)

type serfInfoCommand struct {
}

func SerfInfoCommand() SerfCommand {
	return &serfInfoCommand{}
}

func (t serfInfoCommand) Help() string {
	helpText := `
Usage: serf info [options]

	Provides debugging information for operators

Options:

  -format                  If provided, output is returned in the specified
                           format. Valid formats are 'json', and 'text' (default)
`
	return strings.TrimSpace(helpText)
}

func (t serfInfoCommand) SubCommand() string {
	return "info"
}

func (t serfInfoCommand) Synopsis() string {
	return "Provides debugging information for operators"
}

func (t serfInfoCommand) Run(prov ClientProvider, args []string) error {

	var format string
	cmdFlags := flag.NewFlagSet("info", flag.ContinueOnError)
	cmdFlags.Usage = func() { println(t.Help()) }
	cmdFlags.StringVar(&format, "format", "text", "output format")

	if err := cmdFlags.Parse(args); err != nil {
		return err
	}

	return prov.DoWithClient(func(cli *client.RPCClient) error {
		return t.doRun(cli, format)
	})
}

func (t serfInfoCommand) doRun(client *client.RPCClient, format string) error {

	stats, err := client.Stats()
	if err != nil {
		return err
	}

	output, err := formatOutput(statsString(stats), format)
	if err != nil {
		return xerrors.Errorf("encoding error: %s", err)
	}

	println(output)
	return nil
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
