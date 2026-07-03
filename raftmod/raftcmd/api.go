/*
 * Copyright (c) 2025-2026 Karagatan LLC.
 * SPDX-License-Identifier: BUSL-1.1
 */

package raftcmd

import "github.com/hashicorp/serf/client"

type SerfCommand interface {

	/**
	Full description of the command
	*/
	Help() string

	/**
	Sub command name
	*/

	SubCommand() string

	/**
	Run sub command
	*/

	Run(prov ClientProvider, args []string) error

	/**
	One line description of the command
	*/
	Synopsis() string

}

type ClientProvider interface {

	DoWithClient(func(cli *client.RPCClient) error) error

}