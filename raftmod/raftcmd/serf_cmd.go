/*
 * Copyright (c) 2025-2026 Karagatan LLC.
 * SPDX-License-Identifier: BUSL-1.1
 */

package raftcmd

import (
	"fmt"
	"strings"

	"github.com/hashicorp/serf/client"
	"go.arpabet.com/cligo"
	"go.arpabet.com/raft/raftmod"
	"golang.org/x/xerrors"
)

/**
Cli group 'serf' that hosts the serf agent management commands.
*/
type serfGroup struct {
	Parent cligo.CliGroup `cli:"group=cli"`
}

func SerfGroup() cligo.CliGroup {
	return &serfGroup{}
}

func (t *serfGroup) Group() string {
	return "serf"
}

func (t *serfGroup) Help() (string, string) {
	return "Manages the Serf (gossip) server.", ""
}

/**
Default ClientProvider connecting to the serf agent RPC endpoint of the local
node. Uses the same 'serf.rpc-address' property as the serf server bean and
adjusts the port by 'node.seq', so the command talks to its own node in
multi-node-per-host setups.
*/
type serfClientProvider struct {
	SerfAddress string `value:"serf.rpc-address,default=127.0.0.1:8700"`
	SerfToken   string `value:"serf.rpc-auth,default="`
	NodeSeq     int    `value:"node.seq,default=0"`
}

func SerfClientProvider() ClientProvider {
	return &serfClientProvider{}
}

func (t *serfClientProvider) DoWithClient(cb func(cli *client.RPCClient) error) error {

	addr := getConnectAddress(t.SerfAddress)

	tcpAddr, err := raftmod.ParseAndAdjustTCPAddr(addr, t.NodeSeq)
	if err != nil {
		return err
	}
	addr = fmt.Sprintf("%s:%d", tcpAddr.IP.String(), tcpAddr.Port)

	config := client.Config{Addr: addr, AuthKey: t.SerfToken}
	cli, err := client.ClientFromConfig(&config)
	if err != nil {
		return xerrors.Errorf("connecting to Serf agent '%s', %v", addr, err)
	}
	defer cli.Close()
	return cb(cli)
}

func getConnectAddress(listenAddr string) string {
	if strings.HasPrefix(listenAddr, "0.0.0.0:") {
		return "127.0.0.1" + listenAddr[7:]
	}
	if strings.HasPrefix(listenAddr, ":") {
		return "127.0.0.1" + listenAddr
	}
	return listenAddr
}
