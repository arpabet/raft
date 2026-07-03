/*
 * Copyright (c) 2025-2026 Karagatan LLC.
 * SPDX-License-Identifier: BUSL-1.1
 */

// Package raftvrpc serves the raft control plane (bootstrap / join /
// configuration / command-forwarding) over value-rpc — the same contract as
// raftgrpc, on a schemaless Go-to-Go wire. Messages are the raftpb types mapped
// to value.Map by the codecs below (no protobuf on the wire).
package raftvrpc

import (
	"go.arpabet.com/raft/raftpb"
	"go.arpabet.com/value"
	"go.arpabet.com/value-rpc/valuerpc"
	"golang.org/x/xerrors"
)

// emptyCodec encodes a request/response with no fields (an empty map).
var emptyCodec = valuerpc.Codec[struct{}]{
	Encode: func(struct{}) value.Value { return value.EmptyMap(true) },
	Decode: func(value.Value) (struct{}, error) { return struct{}{}, nil },
}

var statusCodec = valuerpc.Codec[*raftpb.Status]{
	Encode: func(s *raftpb.Status) value.Value {
		if s == nil {
			s = &raftpb.Status{}
		}
		return value.EmptyMap(true).
			Put("updated", value.Boolean(s.Updated)).
			Put("elapsed", value.Double(s.Elapsed)).
			Put("id", value.Utf8(s.Id))
	},
	Decode: func(v value.Value) (*raftpb.Status, error) {
		m, ok := v.(value.Map)
		if !ok {
			return nil, xerrors.New("status: expected a map")
		}
		return &raftpb.Status{
			Updated: m.GetBool("updated").Boolean(),
			Elapsed: m.GetNumber("elapsed").Double(),
			Id:      m.GetString("id").String(),
		}, nil
	},
}

var raftNodeCodec = valuerpc.Codec[*raftpb.RaftNode]{
	Encode: func(n *raftpb.RaftNode) value.Value {
		if n == nil {
			n = &raftpb.RaftNode{}
		}
		return value.EmptyMap(true).
			Put("node_id", value.Utf8(n.NodeId)).
			Put("node_addr", value.Utf8(n.NodeAddr))
	},
	Decode: func(v value.Value) (*raftpb.RaftNode, error) {
		m, ok := v.(value.Map)
		if !ok {
			return nil, xerrors.New("raft node: expected a map")
		}
		return &raftpb.RaftNode{
			NodeId:   m.GetString("node_id").String(),
			NodeAddr: m.GetString("node_addr").String(),
		}, nil
	},
}

var commandCodec = valuerpc.Codec[*raftpb.Command]{
	Encode: func(c *raftpb.Command) value.Value {
		var payload []byte
		if c != nil {
			payload = c.Payload
		}
		return value.EmptyMap(true).Put("payload", value.Raw(payload, false))
	},
	Decode: func(v value.Value) (*raftpb.Command, error) {
		m, ok := v.(value.Map)
		if !ok {
			return nil, xerrors.New("command: expected a map")
		}
		return &raftpb.Command{Payload: m.GetString("payload").Raw()}, nil
	},
}

func encodeRaftServer(s *raftpb.RaftServer) value.Value {
	return value.EmptyMap(true).
		Put("node_id", value.Utf8(s.NodeId)).
		Put("raft_addr", value.Utf8(s.RaftAddr)).
		Put("suffrage", value.Utf8(s.Suffrage)).
		Put("api_addr", value.Utf8(s.ApiAddr))
}

func decodeRaftServer(m value.Map) *raftpb.RaftServer {
	return &raftpb.RaftServer{
		NodeId:   m.GetString("node_id").String(),
		RaftAddr: m.GetString("raft_addr").String(),
		Suffrage: m.GetString("suffrage").String(),
		ApiAddr:  m.GetString("api_addr").String(),
	}
}

var raftConfigurationCodec = valuerpc.Codec[*raftpb.RaftConfiguration]{
	Encode: func(c *raftpb.RaftConfiguration) value.Value {
		if c == nil {
			c = &raftpb.RaftConfiguration{}
		}
		list := value.EmptyList(true)
		for _, s := range c.ServerList {
			list = list.Append(encodeRaftServer(s))
		}
		return value.EmptyMap(true).
			Put("state", value.Utf8(c.State)).
			Put("last_index", value.Long(int64(c.LastIndex))).
			Put("server_list", list)
	},
	Decode: func(v value.Value) (*raftpb.RaftConfiguration, error) {
		m, ok := v.(value.Map)
		if !ok {
			return nil, xerrors.New("raft configuration: expected a map")
		}
		list := m.GetList("server_list")
		var servers []*raftpb.RaftServer
		if list != nil {
			for i := 0; i < list.Len(); i++ {
				if sm := list.GetMapAt(i); sm != nil {
					servers = append(servers, decodeRaftServer(sm))
				}
			}
		}
		return &raftpb.RaftConfiguration{
			State:      m.GetString("state").String(),
			LastIndex:  uint64(m.GetNumber("last_index").Long()),
			ServerList: servers,
		}, nil
	},
}
