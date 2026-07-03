/*
 * Copyright (c) 2025-2026 Karagatan LLC.
 * SPDX-License-Identifier: BUSL-1.1
 */

package raftmod

import (
	"sync"

	"github.com/hashicorp/raft"
	"go.arpabet.com/raft/raftapi"
	"golang.org/x/xerrors"
)

type implServerLookup struct {
	mutex           sync.RWMutex
	addressToServer map[raft.ServerAddress]*raftapi.Server
	idToServer      map[raft.ServerID]*raftapi.Server
}

func ServerLookup() raftapi.ServerLookup {
	return &implServerLookup{
		addressToServer: make(map[raft.ServerAddress]*raftapi.Server),
		idToServer:      make(map[raft.ServerID]*raftapi.Server),
	}
}

func (t *implServerLookup) AddServer(server *raftapi.Server) {
	t.mutex.Lock()
	defer t.mutex.Unlock()
	t.addressToServer[raft.ServerAddress(server.Addr.String())] = server
	t.idToServer[raft.ServerID(server.ID)] = server
}

func (t *implServerLookup) RemoveServer(server *raftapi.Server) {
	t.mutex.Lock()
	defer t.mutex.Unlock()
	delete(t.addressToServer, raft.ServerAddress(server.Addr.String()))
	delete(t.idToServer, raft.ServerID(server.ID))
}

func (t *implServerLookup) ServerAddr(id raft.ServerID) (raft.ServerAddress, error) {
	t.mutex.RLock()
	defer t.mutex.RUnlock()
	svr, ok := t.idToServer[id]
	if !ok {
		return "", xerrors.Errorf("could not find address for server id %v", id)
	}
	return raft.ServerAddress(svr.Addr.String()), nil
}

func (t *implServerLookup) Server(addr raft.ServerAddress) *raftapi.Server {
	t.mutex.RLock()
	defer t.mutex.RUnlock()
	return t.addressToServer[addr]
}

func (t *implServerLookup) Servers() []*raftapi.Server {
	t.mutex.RLock()
	defer t.mutex.RUnlock()
	var servers []*raftapi.Server
	for _, srv := range t.addressToServer {
		servers = append(servers, srv)
	}
	return servers
}
