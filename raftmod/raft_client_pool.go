/*
 * Copyright (c) 2025-2026 Karagatan LLC.
 * SPDX-License-Identifier: BUSL-1.1
 */

package raftmod

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/hashicorp/raft"
	"go.arpabet.com/glue"
	"go.arpabet.com/raft/raftapi"
	"go.uber.org/zap"
	"golang.org/x/xerrors"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/status"
)

type implRaftClientPool struct {
	Properties glue.Properties `inject:""`
	Log        *zap.Logger     `inject:""`

	RaftAddress    string `value:"raft.bind-address,default="`
	RPCBean        string `value:"raft.rpc-bean-name,default="`
	RPCServiceName string `value:"raft.rpc-service-name,default="`

	// DialTimeout bounds a single connection attempt in GetAPIConn. Without it a
	// blocking dial to a down peer would hang the caller (and everyone waiting on
	// the same address) forever.
	DialTimeout time.Duration `value:"raft.timeout,default=10s"`

	portDiff int

	clients sync.Map // key - raft.ServerAddress, value - *clientConnection or *connectingClient

	closeOnce sync.Once
}

type clientConnection struct {
	endpoint    string
	raftAddress raft.ServerAddress
	conn        *grpc.ClientConn
	serviceHC   grpc_health_v1.HealthClient
}

type connectingClient struct {
	waitCh chan struct{}
}

func RaftClientPool() raftapi.RaftClientPool {
	return &implRaftClientPool{}
}

func (t *implRaftClientPool) PostConstruct() error {
	if t.RPCServiceName == "" {
		t.Log.Warn("property 'raft.rpc-service-name' is empty, health check would be disabled")
	}

	if t.RaftAddress != "" && t.RPCBean != "" {
		raftPort, err := getPortNumber(t.RaftAddress)
		if err != nil {
			return xerrors.Errorf("invalid port in property 'raft.bind-address', %v", err)
		}
		prop := t.RPCBean + ".bind-address"
		value := t.Properties.GetString(prop, "")
		if value == "" {
			return xerrors.Errorf("empty property '%s' needed by 'raft.rpc-bean-name' reference", prop)
		}
		rpcPort, err := getPortNumber(value)
		if err != nil {
			return xerrors.Errorf("invalid port in property '%s', %v", prop, err)
		}
		t.portDiff = rpcPort - raftPort
	} else {
		t.Log.Warn("property 'raft.bind-address' or 'raft.rpc-bean-name' is empty")
	}
	return nil
}

func (t *implRaftClientPool) GetAPIEndpoint(raftAddress string) (string, error) {

	raftHost, raftPort, err := getHostAndPortNumber(raftAddress)
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("%s:%d", raftHost, raftPort+t.portDiff), nil
}

func (t *implRaftClientPool) GetAPIConn(raftAddress raft.ServerAddress) (any, error) {

tryAgain:

	if val, ok := t.clients.Load(raftAddress); ok {
		if client, ok := val.(*clientConnection); ok {
			return client.conn, nil
		}
		if stub, ok := val.(*connectingClient); ok {
			<-stub.waitCh
			goto tryAgain
		}
	}

	// let's try to connect
	stub := &connectingClient{waitCh: make(chan struct{})}
	defer close(stub.waitCh)

	actual, loaded := t.clients.LoadOrStore(raftAddress, stub)
	if loaded {
		if client, ok := actual.(*clientConnection); ok {
			return client.conn, nil
		}
		if weAreNotAlone, ok := actual.(*connectingClient); ok {
			<-weAreNotAlone.waitCh
			goto tryAgain
		}
		// go forward
		t.clients.Store(raftAddress, stub)
	}

	client, err := t.doConnect(raftAddress)
	if err != nil {
		// Remove our connecting stub so waiters (woken by the deferred close)
		// and later callers retry the connection instead of spinning forever on
		// a stub whose waitCh is already closed.
		t.clients.Delete(raftAddress)
		return nil, err
	}

	t.clients.Store(raftAddress, client)
	return client.conn, nil

}

func (t *implRaftClientPool) doConnect(raftAddress raft.ServerAddress) (*clientConnection, error) {
	endpoint, err := t.GetAPIEndpoint(string(raftAddress))
	if err != nil {
		return nil, err
	}

	tlsConfig := &tls.Config{
		InsecureSkipVerify: true,
		NextProtos:         []string{"h2"},
	}

	dialTimeout := t.DialTimeout
	if dialTimeout <= 0 {
		dialTimeout = 10 * time.Second
	}
	ctx, cancel := context.WithTimeout(context.Background(), dialTimeout)
	defer cancel()

	conn, err := grpc.DialContext(ctx, endpoint,
		grpc.WithTransportCredentials(credentials.NewTLS(tlsConfig)),
		grpc.WithBlock(),
		grpc.WithReturnConnectionError())
	if err != nil {
		return nil, xerrors.Errorf("dial raft api endpoint '%s', %v", endpoint, err)
	}

	client := &clientConnection{
		endpoint:    endpoint,
		raftAddress: raftAddress,
		conn:        conn,
		serviceHC:   grpc_health_v1.NewHealthClient(conn),
	}

	t.Log.Info("Connected", zap.String("endpoint", endpoint), zap.String("raftAddress", string(raftAddress)), zap.String("state", conn.GetState().String()))

	if t.RPCServiceName != "" {
		go t.doHealthCheck(client)
	}

	return client, nil
}

func (t *implRaftClientPool) doHealthCheck(client *clientConnection) {

	resp, err := client.serviceHC.Check(context.Background(), &grpc_health_v1.HealthCheckRequest{
		Service: t.RPCServiceName,
	})

	if err != nil {
		if stat, ok := status.FromError(err); ok && stat.Code() == codes.Unimplemented {
			t.Log.Info("HealthCheckNotImplemented", zap.String("endpoint", client.endpoint), zap.String("raftAddress", string(client.raftAddress)))
		} else {
			t.Log.Info("HealthCheckRPC", zap.Error(err), zap.String("endpoint", client.endpoint), zap.String("raftAddress", string(client.raftAddress)))
		}
		return
	}

	t.Log.Info("HealthCheckStatus", zap.String("status", resp.Status.String()), zap.String("endpoint", client.endpoint), zap.String("raftAddress", string(client.raftAddress)))

	w, err := client.serviceHC.Watch(context.Background(), &grpc_health_v1.HealthCheckRequest{
		Service: t.RPCServiceName,
	})
	if err != nil {
		t.Log.Error("HealthCheckWatch", zap.String("endpoint", client.endpoint), zap.String("raftAddress", string(client.raftAddress)), zap.Error(err))
		return
	}

	current := resp.Status
	for {

		resp, err := w.Recv()
		if err != nil {
			if err != io.EOF {
				t.Log.Error("HealthCheckError", zap.String("endpoint", client.endpoint), zap.String("raftAddress", string(client.raftAddress)), zap.Error(err))
			}
			break
		}

		if current != resp.Status {
			t.Log.Info("HealthCheckStatus", zap.String("status", resp.Status.String()), zap.String("endpoint", client.endpoint), zap.String("raftAddress", string(client.raftAddress)))
			current = resp.Status
		}

	}

	t.removeClient(client.raftAddress, client.conn)

}

func (t *implRaftClientPool) removeClient(raftAddress raft.ServerAddress, conn *grpc.ClientConn) {

	if value, ok := t.clients.Load(raftAddress); ok {
		if client, ok := value.(*clientConnection); ok {
			if client.conn == conn {
				t.clients.Delete(raftAddress)
			}
		}
	}

}

func (t *implRaftClientPool) Close() error {
	t.closeOnce.Do(func() {

		t.clients.Range(func(key, value interface{}) bool {
			if client, ok := value.(*clientConnection); ok {
				client.conn.Close()
			}
			return true
		})

	})
	return nil
}

func (t *implRaftClientPool) Destroy() error {
	return t.Close()
}
