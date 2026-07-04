/*
 * Copyright (c) 2025-2026 Karagatan LLC.
 * SPDX-License-Identifier: BUSL-1.1
 */

package raftcmd

import "github.com/hashicorp/serf/client"

/**
ClientProvider connects to the local Serf agent RPC endpoint and runs the
callback with a connected client.
*/
type ClientProvider interface {
	DoWithClient(func(cli *client.RPCClient) error) error
}
