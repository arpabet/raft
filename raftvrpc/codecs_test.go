/*
 * Copyright (c) 2025-2026 Karagatan LLC.
 * SPDX-License-Identifier: BUSL-1.1
 */

package raftvrpc

import (
	"bytes"
	"testing"

	"go.arpabet.com/raft/raftpb"
)

// Each codec must survive an encode→decode round-trip over the value model, since
// that mapping is the only hand-written bridge between the raftpb types and the
// value-rpc wire.

func TestStatusCodecRoundTrip(t *testing.T) {
	in := &raftpb.Status{Updated: true, Elapsed: 1.5, Id: "abc"}
	out, err := statusCodec.Decode(statusCodec.Encode(in))
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if out.Updated != in.Updated || out.Elapsed != in.Elapsed || out.Id != in.Id {
		t.Fatalf("round-trip = %+v, want %+v", out, in)
	}
}

func TestRaftNodeCodecRoundTrip(t *testing.T) {
	in := &raftpb.RaftNode{NodeId: "node-7", NodeAddr: "10.0.0.7:8300"}
	out, err := raftNodeCodec.Decode(raftNodeCodec.Encode(in))
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if out.NodeId != in.NodeId || out.NodeAddr != in.NodeAddr {
		t.Fatalf("round-trip = %+v, want %+v", out, in)
	}
}

func TestCommandCodecRoundTrip(t *testing.T) {
	in := &raftpb.Command{Payload: []byte{0x00, 0x01, 0xFE, 0xFF, 'x'}}
	out, err := commandCodec.Decode(commandCodec.Encode(in))
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !bytes.Equal(out.Payload, in.Payload) {
		t.Fatalf("payload round-trip = %v, want %v", out.Payload, in.Payload)
	}
}

func TestRaftConfigurationCodecRoundTrip(t *testing.T) {
	in := &raftpb.RaftConfiguration{
		State:     "Leader",
		LastIndex: 42,
		ServerList: []*raftpb.RaftServer{
			{NodeId: "n1", RaftAddr: "10.0.0.1:8300", Suffrage: "Voter", ApiAddr: "10.0.0.1:8442"},
			{NodeId: "n2", RaftAddr: "10.0.0.2:8300", Suffrage: "Voter", ApiAddr: "10.0.0.2:8442"},
		},
	}
	out, err := raftConfigurationCodec.Decode(raftConfigurationCodec.Encode(in))
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if out.State != in.State || out.LastIndex != in.LastIndex {
		t.Fatalf("scalar round-trip = %+v, want state/index %s/%d", out, in.State, in.LastIndex)
	}
	if len(out.ServerList) != len(in.ServerList) {
		t.Fatalf("server list len = %d, want %d", len(out.ServerList), len(in.ServerList))
	}
	for i, got := range out.ServerList {
		w := in.ServerList[i]
		if got.NodeId != w.NodeId || got.RaftAddr != w.RaftAddr || got.Suffrage != w.Suffrage || got.ApiAddr != w.ApiAddr {
			t.Fatalf("server[%d] = %+v, want %+v", i, got, w)
		}
	}
}
