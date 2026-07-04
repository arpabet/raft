/*
 * Copyright (c) 2025-2026 Karagatan LLC.
 * SPDX-License-Identifier: BUSL-1.1
 */

package raftvrpc

import (
	"crypto/tls"
	"fmt"
	"net"
	"strconv"
	"strings"
	"sync"

	"github.com/hashicorp/raft"
	"go.arpabet.com/glue"
	"go.arpabet.com/raft/raftapi"
	"go.arpabet.com/value-rpc/valueclient"
	"go.uber.org/zap"
	"golang.org/x/xerrors"
)

// ClientPool is the value-rpc implementation of raftapi.RaftClientPool. It maps a
// raft consensus address to the peer's control-service endpoint (same host, port
// shifted by the raft↔rpc port difference) and dials it with a cached, reconnecting
// value-rpc client. GetAPIConn returns a valueclient.Client (via the generalized
// `any`); callers use it with the raftvrpc Call* helpers.
type ClientPool struct {
	Properties glue.Properties `inject:""`
	Log        *zap.Logger     `inject:""`

	RaftAddress string `value:"raft.bind-address,default="`
	RPCBean     string `value:"raft.rpc-bean-name,default="`

	// TLSConfig, when injected, dials the control service over TLS — mTLS parity
	// with the gRPC pool. Nil means a plaintext TCP dial.
	TLSConfig *tls.Config `inject:"optional"`

	portDiff int

	mu      sync.Mutex
	clients map[raft.ServerAddress]valueclient.Client
}

// RaftVrpcClientPool constructs the pool bean.
func RaftVrpcClientPool() raftapi.RaftClientPool {
	return &ClientPool{clients: make(map[raft.ServerAddress]valueclient.Client)}
}

var _ raftapi.RaftClientPool = (*ClientPool)(nil)

func (t *ClientPool) BeanName() string { return "raft-vrpc-client-pool" }

func (t *ClientPool) PostConstruct() error {
	if t.clients == nil {
		t.clients = make(map[raft.ServerAddress]valueclient.Client)
	}
	// The control endpoint shares the host with the raft address and shifts the
	// port by (rpcPort - raftPort), derived from the referenced RPC server's bind
	// address — same scheme as the gRPC pool.
	if t.RaftAddress != "" && t.RPCBean != "" {
		raftPort, err := portOf(t.RaftAddress)
		if err != nil {
			return xerrors.Errorf("invalid port in 'raft.bind-address': %v", err)
		}
		prop := t.RPCBean + ".bind-address"
		rpcAddr := t.Properties.GetString(prop, "")
		if rpcAddr == "" {
			return xerrors.Errorf("empty property '%s' needed by 'raft.rpc-bean-name'", prop)
		}
		rpcPort, err := portOf(rpcAddr)
		if err != nil {
			return xerrors.Errorf("invalid port in '%s': %v", prop, err)
		}
		t.portDiff = rpcPort - raftPort
	} else {
		t.Log.Warn("raft vrpc pool: 'raft.bind-address' or 'raft.rpc-bean-name' empty; portDiff=0")
	}
	return nil
}

func (t *ClientPool) GetAPIEndpoint(raftAddress string) (string, error) {
	host, port, err := hostPort(raftAddress)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s:%d", host, port+t.portDiff), nil
}

func (t *ClientPool) GetAPIConn(raftAddress raft.ServerAddress) (any, error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if cli, ok := t.clients[raftAddress]; ok {
		return cli, nil
	}

	endpoint, err := t.GetAPIEndpoint(string(raftAddress))
	if err != nil {
		return nil, err
	}
	address := "tcp://" + endpoint

	var cli valueclient.Client
	if t.TLSConfig != nil {
		cli = valueclient.NewTLSClient(address, t.TLSConfig, valueclient.WithLogger(t.Log))
	} else {
		cli = valueclient.NewClient(address, "", valueclient.WithLogger(t.Log))
	}
	if err := cli.Connect(); err != nil {
		return nil, xerrors.Errorf("dial raft control endpoint %s: %v", address, err)
	}

	t.clients[raftAddress] = cli
	t.Log.Info("RaftControlConnected", zap.String("endpoint", endpoint), zap.String("raftAddress", string(raftAddress)))
	return cli, nil
}

func (t *ClientPool) Close() error {
	t.mu.Lock()
	defer t.mu.Unlock()
	for addr, cli := range t.clients {
		_ = cli.Close()
		delete(t.clients, addr)
	}
	return nil
}

func (t *ClientPool) Destroy() error { return t.Close() }

// portOf returns the port number of a host:port address.
func portOf(address string) (int, error) {
	_, port, err := hostPort(address)
	return port, err
}

// hostPort splits a host:port address into its host and numeric port. A leading
// scheme ("tcp://host:port", the style value-rpc bind addresses use) is accepted
// and ignored.
func hostPort(address string) (string, int, error) {
	if i := strings.Index(address, "://"); i >= 0 {
		address = address[i+3:]
	}
	host, portStr, err := net.SplitHostPort(address)
	if err != nil {
		return "", 0, err
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		return "", 0, xerrors.Errorf("invalid port %q: %v", portStr, err)
	}
	return host, port, nil
}
