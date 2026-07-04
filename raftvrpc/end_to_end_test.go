/*
 * Copyright (c) 2025-2026 Karagatan LLC.
 * SPDX-License-Identifier: BUSL-1.1
 */

package raftvrpc

import (
	"context"
	"io"
	"net"
	"testing"
	"time"

	"github.com/hashicorp/raft"
	"go.arpabet.com/raft/raftapi"
	"go.arpabet.com/raft/raftpb"
	"go.arpabet.com/uuid"
	"go.arpabet.com/value-rpc/valueclient"
	"go.arpabet.com/value-rpc/valueserver"
	"go.uber.org/zap"
)

// stubNodeService satisfies raftapi.NodeService; only NodeIdHex is meaningful.
type stubNodeService struct{}

func (stubNodeService) BeanName() string  { return "test-node-service" }
func (stubNodeService) NodeId() uint64    { return 1 }
func (stubNodeService) NodeIdHex() string { return testNodeID }
func (stubNodeService) LocalName() string { return "test" }
func (stubNodeService) LANName() string   { return "test" }
func (stubNodeService) NodeSeq() int      { return 0 }
func (stubNodeService) Issue() uuid.UUID  { return uuid.UUID{} }

const testNodeID = "node-abc"

// --- test doubles -----------------------------------------------------------

// testFSM is a raft.FSM whose Apply reports a raftapi.FSMResponse, so the handler
// can read the Status back exactly as in production.
type testFSM struct{ applied [][]byte }

func (f *testFSM) Apply(l *raft.Log) interface{} {
	f.applied = append(f.applied, l.Data)
	return raftapi.FSMResponse{Status: &raftpb.Status{Updated: true}}
}
func (f *testFSM) Snapshot() (raft.FSMSnapshot, error) { return nil, nil }
func (f *testFSM) Restore(io.ReadCloser) error         { return nil }

// testRaftServer implements raftapi.RaftServer over a real in-memory raft. Only
// Raft/Transport/IsLeader are exercised by the handlers; the server lifecycle
// methods are no-ops to satisfy the interface.
type testRaftServer struct {
	r  *raft.Raft
	tr raft.Transport
}

func (s *testRaftServer) Raft() (*raft.Raft, bool)         { return s.r, s.r != nil }
func (s *testRaftServer) Transport() (raft.Transport, bool) { return s.tr, s.tr != nil }
func (s *testRaftServer) IsLeader() bool                    { return s.r.State() == raft.Leader }
func (s *testRaftServer) PostConstruct() error              { return nil }
func (s *testRaftServer) Destroy() error                    { return nil }
func (s *testRaftServer) Bind() error                       { return nil }
func (s *testRaftServer) Alive() bool                       { return true }
func (s *testRaftServer) ListenAddress() net.Addr           { return nil }
func (s *testRaftServer) Serve() error                      { return nil }
func (s *testRaftServer) Shutdown() error                   { return nil }
func (s *testRaftServer) ShutdownCh() <-chan struct{}       { return nil }
func (s *testRaftServer) BeanName() string                  { return "test-raft-server" }
func (s *testRaftServer) GetStats(func(string, string) bool) error { return nil }

// echoPool is a RaftClientPool whose endpoint resolution just echoes the raft
// address (the in-memory transport addresses are not host:port). It is enough for
// GetConfiguration; GetAPIConn is unused in the single-node test.
type echoPool struct{}

func (echoPool) GetAPIEndpoint(raftAddress string) (string, error)   { return raftAddress, nil }
func (echoPool) GetAPIConn(raft.ServerAddress) (any, error)          { return nil, nil }
func (echoPool) Close() error                                        { return nil }
func (echoPool) PostConstruct() error                                { return nil }
func (echoPool) Destroy() error                                      { return nil }

func newInmemRaft(t *testing.T, fsm raft.FSM) (*raft.Raft, raft.Transport) {
	t.Helper()
	store := raft.NewInmemStore()
	snaps := raft.NewInmemSnapshotStore()
	_, transport := raft.NewInmemTransport("")
	config := raft.DefaultConfig()
	config.LocalID = raft.ServerID(testNodeID)
	config.Logger = nil
	r, err := raft.NewRaft(config, fsm, store, store, snaps, transport)
	if err != nil {
		t.Fatalf("new raft: %v", err)
	}
	t.Cleanup(func() { _ = r.Shutdown() })
	return r, transport
}

func waitLeader(t *testing.T, r *raft.Raft) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if r.State() == raft.Leader {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("node did not become leader in time")
}

// --- tests ------------------------------------------------------------------

// The whole control plane travels over value-rpc against a real raft: the client
// bootstraps the cluster, reads the configuration back, and applies a command —
// all via the typed Call* helpers over an in-memory value-rpc transport.
func TestControlPlaneOverVrpc(t *testing.T) {
	fsm := &testFSM{}
	r, transport := newInmemRaft(t, fsm) // fresh raft starts as Follower (not bootstrapped)

	handler := &Handler{
		NodeService:    stubNodeService{},
		RaftServer:     &testRaftServer{r: r, tr: transport},
		RaftClientPool: echoPool{},
		Timeout:        5 * time.Second,
		Log:            zap.NewNop(),
	}

	srv, err := valueserver.NewMemServer("raft-control-test", zap.NewNop())
	if err != nil {
		t.Fatalf("mem server: %v", err)
	}
	defer srv.Close()
	if err := Register(srv, handler); err != nil {
		t.Fatalf("register: %v", err)
	}
	go srv.Run()

	cli := valueclient.NewMemClient("raft-control-test")
	if err := cli.Connect(); err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer cli.Close()
	ctx := context.Background()

	// Bootstrap the cluster through the RPC.
	if st, err := CallBootstrap(ctx, cli); err != nil || !st.Updated {
		t.Fatalf("bootstrap rpc: st=%+v err=%v", st, err)
	}
	waitLeader(t, r)

	// Configuration reflects the single bootstrapped voter.
	cfg, err := CallGetConfiguration(ctx, cli)
	if err != nil {
		t.Fatalf("get configuration: %v", err)
	}
	if cfg.State != raft.Leader.String() {
		t.Fatalf("state = %q, want Leader", cfg.State)
	}
	if len(cfg.ServerList) != 1 || cfg.ServerList[0].NodeId != testNodeID {
		t.Fatalf("server list = %+v, want one voter %s", cfg.ServerList, testNodeID)
	}

	// Apply a command through the RPC; the FSM records it and returns Updated.
	st, err := CallApplyCommand(ctx, cli, &raftpb.Command{Payload: []byte("hello-raft")})
	if err != nil || !st.Updated {
		t.Fatalf("apply command: st=%+v err=%v", st, err)
	}
	if len(fsm.applied) != 1 || string(fsm.applied[0]) != "hello-raft" {
		t.Fatalf("fsm applied = %v, want [hello-raft]", fsm.applied)
	}
}

// The client pool dials a real value-rpc control server over TCP and the returned
// client can drive an RPC — this is the connection path used for follower→leader
// forwarding of ApplyCommand.
func TestClientPoolDialsControlServer(t *testing.T) {
	fsm := &testFSM{}
	r, transport := newInmemRaft(t, fsm)

	handler := &Handler{
		NodeService:    stubNodeService{},
		RaftServer:     &testRaftServer{r: r, tr: transport},
		RaftClientPool: echoPool{},
		Timeout:        5 * time.Second,
		Log:            zap.NewNop(),
	}
	srv, err := valueserver.NewServer("tcp://127.0.0.1:0", zap.NewNop())
	if err != nil {
		t.Fatalf("tcp server: %v", err)
	}
	defer srv.Close()
	if err := Register(srv, handler); err != nil {
		t.Fatalf("register: %v", err)
	}
	go srv.Run()

	// Bootstrap directly so the node is a leader that can apply.
	if err := r.BootstrapCluster(raft.Configuration{
		Servers: []raft.Server{{Suffrage: raft.Voter, ID: raft.ServerID(testNodeID), Address: transport.LocalAddr()}},
	}).Error(); err != nil {
		t.Fatalf("bootstrap: %v", err)
	}
	waitLeader(t, r)

	// The pool with portDiff 0 dials the server's actual TCP address.
	addr := srv.Addr().(*net.TCPAddr)
	pool := &ClientPool{clients: make(map[raft.ServerAddress]valueclient.Client), Log: zap.NewNop()}
	defer pool.Close()

	connAny, err := pool.GetAPIConn(raft.ServerAddress(addr.String()))
	if err != nil {
		t.Fatalf("pool GetAPIConn: %v", err)
	}
	cli, ok := connAny.(valueclient.Client)
	if !ok {
		t.Fatalf("pool returned %T, want valueclient.Client", connAny)
	}

	st, err := CallApplyCommand(context.Background(), cli, &raftpb.Command{Payload: []byte("via-pool")})
	if err != nil || !st.Updated {
		t.Fatalf("apply via pooled client: st=%+v err=%v", st, err)
	}

	// GetAPIEndpoint math: host preserved, port shifted by portDiff.
	pool.portDiff = 5
	ep, err := pool.GetAPIEndpoint("10.0.0.4:8300")
	if err != nil || ep != "10.0.0.4:8305" {
		t.Fatalf("endpoint = %q err=%v, want 10.0.0.4:8305", ep, err)
	}
}
