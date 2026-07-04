/*
 * Copyright (c) 2025-2026 Karagatan LLC.
 * SPDX-License-Identifier: BUSL-1.1
 */

package raftcmd

import (
	"context"
	"fmt"
	"time"

	"github.com/hashicorp/serf/client"
	"github.com/hashicorp/serf/serf"
	"go.arpabet.com/cligo"
	"golang.org/x/xerrors"
)

const (
	troubleshooting = `
Troubleshooting tips:
* Ensure that the bind addr:port is accessible by all other nodes
* If an advertise address is set, ensure it routes to the bind address
* Check that no nodes are behind a NAT
* If nodes are behind firewalls or iptables, check that Serf traffic is permitted (UDP and TCP)
* Verify networking equipment is functional`
)

type serfReachabilityCommand struct {
	Parent cligo.CliGroup `cli:"group=serf"`
	Prov   ClientProvider `inject:""`

	Verbose bool `cli:"option=verbose,default=false,help=Verbose mode."`
}

func SerfReachabilityCommand() cligo.CliCommand {
	return &serfReachabilityCommand{}
}

func (t *serfReachabilityCommand) Command() string {
	return "reachability"
}

func (t *serfReachabilityCommand) Help() (string, string) {
	return "Tests the network reachability of this node.", ""
}

func (t *serfReachabilityCommand) Run(ctx context.Context) error {
	return t.Prov.DoWithClient(func(cli *client.RPCClient) error {
		return t.doRun(cli)
	})
}

func (t *serfReachabilityCommand) doRun(cli *client.RPCClient) error {

	shutdownCh := makeShutdownCh()
	ackCh := make(chan string, 128)

	// Get the list of members
	members, err := cli.Members()
	if err != nil {
		return xerrors.Errorf("getting members, %v", err)
	}

	// Get only the live members
	liveMembers := make(map[string]struct{})
	for _, m := range members {
		if m.Status == "alive" {
			liveMembers[m.Name] = struct{}{}
		}
	}
	fmt.Printf("Total members: %d, live members: %d\n", len(members), len(liveMembers))

	// Start the query
	params := client.QueryParam{
		RequestAck: true,
		Name:       serf.InternalQueryPrefix + "ping",
		AckCh:      ackCh,
	}
	if err := cli.Query(&params); err != nil {
		return xerrors.Errorf("sending query, %v", err)
	}
	println("Starting reachability test...")
	start := time.Now()
	last := time.Now()

	// Track responses and acknowledgements
	dups := false
	numAcks := 0
	acksFrom := make(map[string]struct{}, len(members))

OUTER:
	for {
		select {
		case a := <-ackCh:
			if a == "" {
				break OUTER
			}
			if t.Verbose {
				fmt.Printf("\tAck from '%s'\n", a)
			}
			numAcks++
			if _, ok := acksFrom[a]; ok {
				dups = true
				fmt.Printf("Duplicate response from '%v'\n", a)
			}
			acksFrom[a] = struct{}{}
			last = time.Now()

		case <-shutdownCh:
			return xerrors.New("Test interrupted")
		}
	}

	if t.Verbose {
		total := float64(time.Now().Sub(start)) / float64(time.Second)
		timeToLast := float64(last.Sub(start)) / float64(time.Second)
		fmt.Printf("Query time: %0.2f sec, time to last response: %0.2f sec\n", total, timeToLast)
	}

	// Print troubleshooting info for duplicate responses
	if dups {
		println("Error: duplicate responses means there is a misconfiguration. Verify that node names are unique.")
	}

	n := len(liveMembers)
	if numAcks == n {
		println("Successfully contacted all live nodes.")
	} else if numAcks > n {
		println("Received more acks than live nodes! Acks from non-live nodes:")
		for m := range acksFrom {
			if _, ok := liveMembers[m]; !ok {
				fmt.Printf("\t%s\n", m)
			}
		}
		println(troubleshooting)
		return xerrors.New("too many asks, this could mean Serf is detecting false-failures due to a misconfiguration or network issue.")

	} else if numAcks < n {
		println("Received less acks than live nodes! Missing acks from:")
		for m := range liveMembers {
			if _, ok := acksFrom[m]; !ok {
				fmt.Printf("\t%s\n", m)
			}
		}
		println(troubleshooting)
		return xerrors.New("too few asks, this could mean Serf gossip packets are being lost due to a misconfiguration or network issue.")
	}
	return nil
}
