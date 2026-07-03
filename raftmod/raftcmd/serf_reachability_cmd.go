/*
 * Copyright (c) 2025-2026 Karagatan LLC.
 * SPDX-License-Identifier: BUSL-1.1
 */

package raftcmd

import (
	"flag"
	"fmt"
	"strings"
	"time"

	"github.com/hashicorp/serf/client"
	"github.com/hashicorp/serf/serf"
	"go.arpabet.com/sprint"
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
	Application sprint.Application `inject:""`
}

func SerfReachabilityCommand() SerfCommand {
	return &serfReachabilityCommand{}
}

func (t serfReachabilityCommand) SubCommand() string {
	return "reachability"
}

func (t serfReachabilityCommand) Help() string {
	helpText := `
Usage: serf reachability [options]

  Tests the network reachability of this node

Options:

  -verbose                  Verbose mode
`
	return strings.TrimSpace(helpText)
}

func (t serfReachabilityCommand) Synopsis() string {
	return "Emit a custom event through the Serf cluster"
}

func (t serfReachabilityCommand) Run(prov ClientProvider, args []string) error {
	var verbose bool
	cmdFlags := flag.NewFlagSet("reachability", flag.ContinueOnError)
	cmdFlags.Usage = func() { println(t.Help()) }
	cmdFlags.BoolVar(&verbose, "verbose", false, "verbose mode")

	if err := cmdFlags.Parse(args); err != nil {
		return err
	}

	return prov.DoWithClient(func(cli *client.RPCClient) error {
		return t.doRun(cli, verbose)
	})
}

func (t serfReachabilityCommand) doRun(cli *client.RPCClient, verbose bool) error {

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
			if verbose {
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

	if verbose {
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
