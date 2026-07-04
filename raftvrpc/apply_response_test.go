/*
 * Copyright (c) 2025-2026 Karagatan LLC.
 * SPDX-License-Identifier: BUSL-1.1
 */

package raftvrpc

import (
	"context"
	"io"
	"testing"
	"time"

	"github.com/hashicorp/raft"
	"go.arpabet.com/raft/raftapi"
	"go.arpabet.com/raft/raftpb"
	"go.uber.org/zap"
)

// pointerFSM returns *raftapi.FSMResponse (the pointer form of the contract),
// like application FSMs commonly do.
type pointerFSM struct{ applied [][]byte }

func (f *pointerFSM) Apply(l *raft.Log) interface{} {
	f.applied = append(f.applied, l.Data)
	return &raftapi.FSMResponse{Status: &raftpb.Status{Updated: true, Id: "ptr"}}
}
func (f *pointerFSM) Snapshot() (raft.FSMSnapshot, error) { return nil, nil }
func (f *pointerFSM) Restore(io.ReadCloser) error         { return nil }

// ApplyCommand must accept an FSM that reports its result as *raftapi.FSMResponse,
// not only as the bare value.
func TestApplyCommandAcceptsPointerFSMResponse(t *testing.T) {
	fsm := &pointerFSM{}
	r, transport := newInmemRaft(t, fsm)

	if err := r.BootstrapCluster(raft.Configuration{
		Servers: []raft.Server{{Suffrage: raft.Voter, ID: raft.ServerID(testNodeID), Address: transport.LocalAddr()}},
	}).Error(); err != nil {
		t.Fatalf("bootstrap: %v", err)
	}
	waitLeader(t, r)

	handler := &Handler{
		NodeService:    stubNodeService{},
		RaftServer:     &testRaftServer{r: r, tr: transport},
		RaftClientPool: echoPool{},
		Timeout:        5 * time.Second,
		Log:            zap.NewNop(),
	}

	st, err := handler.ApplyCommand(context.Background(), &raftpb.Command{Payload: []byte("ptr-cmd")})
	if err != nil {
		t.Fatalf("apply command: %v", err)
	}
	if !st.Updated || st.Id != "ptr" {
		t.Fatalf("status = %+v, want Updated with Id \"ptr\"", st)
	}
	if len(fsm.applied) != 1 || string(fsm.applied[0]) != "ptr-cmd" {
		t.Fatalf("fsm applied = %v, want [ptr-cmd]", fsm.applied)
	}
}

// hostPort accepts value-rpc style scheme'd addresses ("tcp://host:port") as
// configured for vrpc-server.bind-address, alongside bare host:port.
func TestHostPortAcceptsScheme(t *testing.T) {
	for _, c := range []struct {
		in   string
		host string
		port int
	}{
		{"tcp://127.0.0.1:8444", "127.0.0.1", 8444},
		{"tls://10.0.0.4:9000", "10.0.0.4", 9000},
		{"0.0.0.0:8444", "0.0.0.0", 8444},
	} {
		host, port, err := hostPort(c.in)
		if err != nil {
			t.Fatalf("hostPort(%q): %v", c.in, err)
		}
		if host != c.host || port != c.port {
			t.Fatalf("hostPort(%q) = %s:%d, want %s:%d", c.in, host, port, c.host, c.port)
		}
	}
	if _, _, err := hostPort("no-port"); err == nil {
		t.Fatal("hostPort accepted an address without a port")
	}
}
