/*
 * Copyright (c) 2025-2026 Karagatan LLC.
 * SPDX-License-Identifier: BUSL-1.1
 */

package raftmod

import (
	"net"
	"testing"

	"github.com/hashicorp/raft"
	"github.com/hashicorp/serf/serf"
	"github.com/stretchr/testify/require"
)

// ServerLookup.ServerAddr is the raft transport's ServerAddressProvider, so the
// address parsed from serf tags must carry the raft consensus port — not the
// serf gossip port.
func TestParseServerTagsUsesRaftPortForAddr(t *testing.T) {

	member := serf.Member{
		Name: "node-1",
		Addr: net.ParseIP("10.0.0.7"),
		Port: 7946, // serf memberlist port
		Tags: map[string]string{
			"role":      "testapp",
			"id":        "abcdef",
			"port":      "7946",
			"raft-port": "8300",
			"grpc-port": "8443",
		},
	}

	server, err := ParseServerTags(member, "testapp")
	require.NoError(t, err)

	require.Equal(t, "10.0.0.7:8300", server.Addr.String())
	require.Equal(t, 8300, server.RaftPort)
	require.Equal(t, 8443, server.RPCPort)
	require.Equal(t, 7946, server.Port)
	require.Equal(t, 7946, server.JoinPort)

	lookup := ServerLookup()
	lookup.AddServer(server)

	addr, err := lookup.ServerAddr(raft.ServerID("abcdef"))
	require.NoError(t, err)
	require.Equal(t, raft.ServerAddress("10.0.0.7:8300"), addr)
}

func TestParseServerTagsRejectsWrongRole(t *testing.T) {
	member := serf.Member{
		Name: "node-1",
		Addr: net.ParseIP("10.0.0.7"),
		Tags: map[string]string{"role": "other"},
	}
	_, err := ParseServerTags(member, "testapp")
	require.Error(t, err)
}
