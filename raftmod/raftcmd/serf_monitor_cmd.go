/*
 * Copyright (c) 2025-2026 Karagatan LLC.
 * SPDX-License-Identifier: BUSL-1.1
 */

package raftcmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"

	"github.com/hashicorp/logutils"
	"github.com/hashicorp/serf/client"
	"go.arpabet.com/cligo"
	"go.uber.org/atomic"
	"golang.org/x/xerrors"
)

type serfMonitorCommand struct {
	Parent cligo.CliGroup `cli:"group=serf"`
	Prov   ClientProvider `inject:""`

	LogLevel string `cli:"option=log-level,default=INFO,help=Log level of the agent."`

	quitting atomic.Bool
}

func SerfMonitorCommand() cligo.CliCommand {
	return &serfMonitorCommand{}
}

func (t *serfMonitorCommand) Command() string {
	return "monitor"
}

func (t *serfMonitorCommand) Help() (string, string) {
	return "Stream logs from a Serf agent.",
		`Shows recent log messages of a Serf agent, and attaches to the agent,
outputting log messages as they occur in real time. The monitor lets you
listen for log levels that may be filtered out of the Serf agent. For
example your agent may only be logging at INFO level, but with the monitor
you can see the DEBUG level logs.`
}

func (t *serfMonitorCommand) Run(ctx context.Context) error {
	return t.Prov.DoWithClient(func(cli *client.RPCClient) error {
		return t.doRun(cli)
	})
}

func (t *serfMonitorCommand) doRun(client *client.RPCClient) error {

	eventCh := make(chan map[string]interface{}, 1024)
	streamHandle, err := client.Stream("*", eventCh)
	if err != nil {
		return xerrors.Errorf("starting stream, %v", err)
	}
	defer client.Stop(streamHandle)

	logCh := make(chan string, 4096)
	monHandle, err := client.Monitor(logutils.LogLevel(t.LogLevel), logCh)
	if err != nil {
		return xerrors.Errorf("starting monitor, %v", err)
	}
	defer client.Stop(monHandle)

	shutdownCh := makeShutdownCh()

	eventDoneCh := make(chan struct{})
	go func() {
		defer close(eventDoneCh)
	OUTER:
		for {
			select {
			case log := <-logCh:
				if log == "" {
					break OUTER
				}
				println(log)
			case event := <-eventCh:
				if event == nil {
					break OUTER
				}
				println("Event Info:")
				for key, val := range event {
					fmt.Printf("\t%s: %#v\n", key, val)
				}
			}
		}

		if !t.quitting.Load() {
			println("")
			println("Remote side ended the monitor.")
		}
	}()

	select {
	case <-eventDoneCh:
		return nil
	case <-shutdownCh:
		t.quitting.Store(true)
	}

	return nil
}

func makeShutdownCh() <-chan struct{} {
	resultCh := make(chan struct{})

	signalCh := make(chan os.Signal, 10)
	signal.Notify(signalCh, os.Interrupt)
	go func() {
		for {
			<-signalCh
			resultCh <- struct{}{}
		}
	}()

	return resultCh
}
