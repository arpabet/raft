/*
 * Copyright (c) 2025-2026 Karagatan LLC.
 * SPDX-License-Identifier: BUSL-1.1
 */

package raftmod

import (
	"net"
	"strconv"

	"github.com/hashicorp/serf/serf"
	"go.arpabet.com/raft/raftapi"
	"golang.org/x/xerrors"
)

func ParseServerTags(m serf.Member, role string) (*raftapi.Server, error) {
	if m.Tags["role"] != role {
		return nil, xerrors.Errorf("joining role '%s' whereas expected role '%s'", m.Tags["role"], role)
	}

	portStr := m.Tags["port"]
	port, err := strconv.Atoi(portStr)
	if err != nil {
		return nil, xerrors.Errorf("parsing 'port' tag '%s', %v", portStr, err)
	}

	raftStr := m.Tags["raft-port"]
	raftPort, err := strconv.Atoi(raftStr)
	if err != nil {
		return nil, xerrors.Errorf("parsing 'raft-port' tag '%s', %v", raftStr, err)
	}

	grpcStr := m.Tags["grpc-port"]
	grpcPort, err := strconv.Atoi(grpcStr)
	if err != nil {
		return nil, xerrors.Errorf("parsing 'grpc-port' tag '%s', %v", grpcStr, err)
	}

	// Addr feeds ServerLookup.ServerAddr, the raft transport's
	// ServerAddressProvider callback, so it must point at the peer's raft
	// consensus port — not the serf gossip port carried by the "port" tag.
	addr := &net.TCPAddr{IP: m.Addr, Port: raftPort}

	server := &raftapi.Server{
		Name:     m.Name,
		ID:       m.Tags["id"],
		Port:     port,
		JoinPort: int(m.Port),
		RaftPort: raftPort,
		RPCPort:  grpcPort,
		Addr:     addr,
		Build:    m.Tags["build"],
		Version:  m.Tags["version"],
		Status:   m.Status.String(),
	}
	return server, nil
}
