/*
 * Copyright (c) 2025-2026 Karagatan LLC.
 * SPDX-License-Identifier: BUSL-1.1
 */

package raftgrpc

import (
	"context"
	"io"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/raft"
	"go.arpabet.com/raft/raftapi"
	"go.arpabet.com/raft/raftpb"
	"go.arpabet.com/uuid"
	"go.uber.org/zap"
	"golang.org/x/xerrors"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
)

// --- channelReader ----------------------------------------------------------

func TestChannelReaderReassemblesChunks(t *testing.T) {
	ch := make(chan []byte, 3)
	ch <- []byte("hello ")
	ch <- []byte{}
	ch <- []byte("world")
	close(ch)

	data, err := io.ReadAll(&channelReader{incoming: ch})
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if string(data) != "hello world" {
		t.Fatalf("data = %q, want %q", data, "hello world")
	}
}

func TestChannelReaderSmallDestinationBuffer(t *testing.T) {
	ch := make(chan []byte, 1)
	ch <- []byte("abcdef")
	close(ch)

	r := &channelReader{incoming: ch}
	p := make([]byte, 4)

	n, err := r.Read(p)
	if err != nil || n != 4 || string(p[:n]) != "abcd" {
		t.Fatalf("first read = %d %q %v", n, p[:n], err)
	}
	n, err = r.Read(p)
	if err != nil || n != 2 || string(p[:n]) != "ef" {
		t.Fatalf("second read = %d %q %v", n, p[:n], err)
	}
	if _, err = r.Read(p); err != io.EOF {
		t.Fatalf("final read err = %v, want io.EOF", err)
	}
}

// A producer-set terminal error must surface instead of io.EOF, so an aborted
// upload can never be mistaken for a complete stream.
func TestChannelReaderTerminalError(t *testing.T) {
	ch := make(chan []byte, 1)
	ch <- []byte("partial")
	r := &channelReader{incoming: ch}

	streamErr := xerrors.New("client went away")
	r.err = streamErr
	close(ch)

	data, err := io.ReadAll(r)
	if string(data) != "partial" {
		t.Fatalf("data = %q, want %q", data, "partial")
	}
	if err == nil || !strings.Contains(err.Error(), "client went away") {
		t.Fatalf("err = %v, want the producer's terminal error", err)
	}
}

// --- wrapError --------------------------------------------------------------

func newTestServer(r *raft.Raft) *implRaftGrpcServer {
	return &implRaftGrpcServer{
		Auth:        stubAuth{},
		NodeService: stubNodeService{},
		RaftServer:  &stubRaftServer{r: r},
		Log:         zap.NewNop(),
	}
}

func TestWrapErrorClassifiesMessages(t *testing.T) {
	srv := newTestServer(nil)

	cases := []struct {
		issue string
		want  string
	}{
		{"key 'a' not found", "object not found"},
		{"record already exist", "object already exist"},
		{"concurrent transaction aborted", "concurrent transaction"},
		{"some other failure", "internal error"},
	}

	for _, c := range cases {
		wrapped := srv.wrapError(xerrors.New(c.issue), "Test", "user")
		st, ok := status.FromError(wrapped)
		if !ok {
			t.Fatalf("wrapError(%q) did not return a status error: %v", c.issue, wrapped)
		}
		if st.Code() != codes.Internal || !strings.Contains(st.Message(), c.want) {
			t.Fatalf("wrapError(%q) = %q (code %v), want message containing %q", c.issue, st.Message(), st.Code(), c.want)
		}
		if strings.Contains(st.Message(), c.issue) {
			t.Fatalf("wrapError(%q) leaked the raw issue into the client message %q", c.issue, st.Message())
		}
	}
}

func TestWrapErrorPassesThroughStatusAndNowrap(t *testing.T) {
	srv := newTestServer(nil)

	orig := status.Errorf(codes.Unavailable, "leader gone")
	if got := srv.wrapError(orig, "Test", "user"); got != orig {
		t.Fatalf("status error was rewrapped: %v", got)
	}

	got := srv.wrapError(xerrors.New("nowrap: visible to client"), "Test", "user")
	if got.Error() != "visible to client" {
		t.Fatalf("nowrap error = %q, want %q", got.Error(), "visible to client")
	}
}

// --- Recover ----------------------------------------------------------------

// fakeRecoverStream feeds a fixed set of chunks, then blocks on release. It
// simulates a client mid-upload while the server-side restore has already died.
type fakeRecoverStream struct {
	raftpb.RaftService_RecoverServer // panics if an unexpected method is used

	ctx     context.Context
	chunks  [][]byte
	release chan struct{}
	closed  bool
}

func (t *fakeRecoverStream) Context() context.Context { return t.ctx }

func (t *fakeRecoverStream) SendAndClose(*emptypb.Empty) error {
	t.closed = true
	return nil
}

func (t *fakeRecoverStream) Recv() (*raftpb.Content, error) {
	if len(t.chunks) > 0 {
		c := t.chunks[0]
		t.chunks = t.chunks[1:]
		return &raftpb.Content{Content: c}, nil
	}
	<-t.release
	return nil, io.EOF
}

// Recover must fail fast when the snapshot restore errors out, instead of
// blocking forever feeding a channel nobody reads (the pre-fix behavior).
func TestRecoverPropagatesRestoreError(t *testing.T) {
	fsm := &noopFSM{}
	r := newInmemRaft(t, fsm)
	srv := newTestServer(r)

	stream := &fakeRecoverStream{
		ctx:     context.Background(),
		chunks:  [][]byte{[]byte("chunk-1"), []byte("chunk-2"), []byte("chunk-3")},
		release: make(chan struct{}),
	}
	defer close(stream.release)

	done := make(chan error, 1)
	go func() { done <- srv.Recover(stream) }()

	select {
	case err := <-done:
		// A fresh raft has nothing to snapshot, so recoverFromSnapshot fails and
		// that failure must reach the client.
		if err == nil {
			t.Fatal("Recover returned nil, want the restore error")
		}
		if stream.closed {
			t.Fatal("SendAndClose called despite restore failure")
		}
	case <-time.After(10 * time.Second):
		t.Fatal("Recover hung: restore error was swallowed and the handler blocked on the chunk channel")
	}
}

// --- stubs -------------------------------------------------------------------

type noopFSM struct{}

func (noopFSM) Apply(*raft.Log) interface{}         { return nil }
func (noopFSM) Snapshot() (raft.FSMSnapshot, error) { return nil, nil }
func (noopFSM) Restore(io.ReadCloser) error         { return nil }

func newInmemRaft(t *testing.T, fsm raft.FSM) *raft.Raft {
	t.Helper()
	store := raft.NewInmemStore()
	snaps := raft.NewInmemSnapshotStore()
	_, transport := raft.NewInmemTransport("")
	config := raft.DefaultConfig()
	config.LocalID = raft.ServerID("test-node")
	r, err := raft.NewRaft(config, fsm, store, store, snaps, transport)
	if err != nil {
		t.Fatalf("new raft: %v", err)
	}
	t.Cleanup(func() { _ = r.Shutdown() })
	return r
}

type stubNodeService struct{}

func (stubNodeService) BeanName() string  { return "test-node-service" }
func (stubNodeService) NodeId() uint64    { return 1 }
func (stubNodeService) NodeIdHex() string { return "node-test" }
func (stubNodeService) LocalName() string { return "test" }
func (stubNodeService) LANName() string   { return "test" }
func (stubNodeService) NodeSeq() int      { return 0 }
func (stubNodeService) Issue() uuid.UUID  { return uuid.UUID{} }

type stubAuth struct{}

func (stubAuth) GetUser(context.Context) (*raftapi.AuthorizedUser, bool) {
	return &raftapi.AuthorizedUser{Username: "admin", Roles: map[string]bool{"ADMIN": true}}, true
}

type stubRaftServer struct {
	r *raft.Raft
}

func (s *stubRaftServer) Raft() (*raft.Raft, bool)                 { return s.r, s.r != nil }
func (s *stubRaftServer) Transport() (raft.Transport, bool)        { return nil, false }
func (s *stubRaftServer) IsLeader() bool                           { return false }
func (s *stubRaftServer) PostConstruct() error                     { return nil }
func (s *stubRaftServer) Destroy() error                           { return nil }
func (s *stubRaftServer) Bind() error                              { return nil }
func (s *stubRaftServer) Alive() bool                              { return true }
func (s *stubRaftServer) ListenAddress() net.Addr                  { return nil }
func (s *stubRaftServer) Serve() error                             { return nil }
func (s *stubRaftServer) Shutdown() error                          { return nil }
func (s *stubRaftServer) ShutdownCh() <-chan struct{}              { return nil }
func (s *stubRaftServer) BeanName() string                         { return "stub-raft-server" }
func (s *stubRaftServer) GetStats(func(string, string) bool) error { return nil }
