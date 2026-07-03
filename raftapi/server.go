/*
 * Copyright (c) 2025-2026 Karagatan LLC.
 * SPDX-License-Identifier: BUSL-1.1
 */

package raftapi

import (
	"fmt"
	"github.com/hashicorp/raft"
	"net"
	"reflect"
)

type Server struct {
	Name                string
	ID                  string
	Port                int
	JoinPort            int
	RPCPort             int
	RaftPort            int
	Build               string
	Version             string
	Addr                net.Addr
	Status              string
}

func (s *Server) String() string {
	var addrStr, networkStr string
	if s.Addr != nil {
		addrStr = s.Addr.String()
		networkStr = s.Addr.Network()
	}
	return fmt.Sprintf("%s (Addr: %s/%s)", s.Name, networkStr, addrStr)
}

var ServerLookupClass = reflect.TypeOf((*ServerLookup)(nil)).Elem()

type ServerLookup interface {

	AddServer(*Server)

	RemoveServer(*Server)

	// Callback Interface for Raft Transport
	ServerAddr(id raft.ServerID) (raft.ServerAddress, error)

	Server(addr raft.ServerAddress) *Server

	Servers() []*Server

}

