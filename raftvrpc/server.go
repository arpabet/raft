/*
 * Copyright (c) 2025-2026 Karagatan LLC.
 * SPDX-License-Identifier: BUSL-1.1
 */

package raftvrpc

import (
	"context"
	"time"

	"github.com/hashicorp/raft"
	"go.arpabet.com/raft/raftapi"
	"go.arpabet.com/raft/raftpb"
	"go.arpabet.com/sprint"
	"go.arpabet.com/value-rpc/valueclient"
	"go.arpabet.com/value-rpc/valueserver"
	"go.arpabet.com/value-rpc/valuerpc"
	"go.uber.org/zap"
	"golang.org/x/xerrors"
)

var (
	ErrRaftNotInitialized = valuerpc.NewError(valuerpc.CodeUnavailable, "raft not initialized")
	ErrRaftLeaderNotFound = valuerpc.NewError(valuerpc.CodeUnavailable, "raft leader not found")
)

// Function names of the raft control service on the value-rpc wire.
const (
	FnBootstrap        = "raft.bootstrap"
	FnJoin             = "raft.join"
	FnGetConfiguration = "raft.config"
	FnApplyCommand     = "raft.apply"
)

// Handler implements the raft control operations over value-rpc. It mirrors the
// gRPC raftgrpc server: same raft semantics, valuerpc errors instead of gRPC
// status codes. Construct one and Register it on a valueserver.Server.
type Handler struct {
	NodeService    sprint.NodeService
	RaftServer     raftapi.RaftServer
	RaftService    raftapi.RaftService
	RaftClientPool raftapi.RaftClientPool // used to forward to the leader; may be nil
	// Auth gates the control API to ADMIN callers. Nil disables the gate (e.g. in
	// tests or when the transport already authenticates peers via mTLS).
	Auth    sprint.AuthorizationMiddleware
	Timeout time.Duration
	Log     *zap.Logger
}

// Register wires the handler's operations onto a value-rpc server.
func Register(srv valueserver.Server, h *Handler) error {
	if err := valueserver.AddUnary(srv, FnBootstrap, emptyCodec, statusCodec, h.Bootstrap); err != nil {
		return err
	}
	if err := valueserver.AddUnary(srv, FnJoin, raftNodeCodec, statusCodec, h.Join); err != nil {
		return err
	}
	if err := valueserver.AddUnary(srv, FnGetConfiguration, emptyCodec, raftConfigurationCodec, h.GetConfiguration); err != nil {
		return err
	}
	if err := valueserver.AddUnary(srv, FnApplyCommand, commandCodec, statusCodec, h.ApplyCommand); err != nil {
		return err
	}
	return nil
}

func (t *Handler) Bootstrap(ctx context.Context, _ struct{}) (*raftpb.Status, error) {
	err := t.doWithRaft(ctx, "Bootstrap", func(ctx context.Context, r *raft.Raft) error {
		tr, ok := t.RaftServer.Transport()
		if !ok {
			return ErrRaftNotInitialized
		}
		if r.State() != raft.Follower {
			return xerrors.Errorf("raft node not in follower mode")
		}
		config := raft.DefaultConfig()
		config.LocalID = raft.ServerID(t.NodeService.NodeIdHex())
		configuration := raft.Configuration{
			Servers: []raft.Server{{ID: config.LocalID, Address: tr.LocalAddr()}},
		}
		t.Log.Info("Bootstrap", zap.String("id", t.NodeService.NodeIdHex()), zap.String("addr", string(tr.LocalAddr())))
		return r.BootstrapCluster(configuration).Error()
	})
	return &raftpb.Status{Updated: err == nil}, err
}

func (t *Handler) Join(ctx context.Context, node *raftpb.RaftNode) (*raftpb.Status, error) {
	err := t.doWithRaft(ctx, "Join", func(ctx context.Context, r *raft.Raft) error {
		configFuture := r.GetConfiguration()
		if err := configFuture.Error(); err != nil {
			t.Log.Error("GetConfiguration", zap.Error(err))
			return err
		}
		for _, srv := range configFuture.Configuration().Servers {
			if srv.ID == raft.ServerID(node.NodeId) || srv.Address == raft.ServerAddress(node.NodeAddr) {
				if srv.Address == raft.ServerAddress(node.NodeAddr) && srv.ID == raft.ServerID(node.NodeId) {
					t.Log.Info("AlreadyMember", zap.String("node", node.String()))
					return nil
				}
				if err := r.RemoveServer(srv.ID, 0, 0).Error(); err != nil {
					return xerrors.Errorf("removing existing node %s at %s: %v", node.NodeId, node.NodeAddr, err)
				}
			}
		}
		if err := r.AddVoter(raft.ServerID(node.NodeId), raft.ServerAddress(node.NodeAddr), 0, 0).Error(); err != nil {
			return err
		}
		t.Log.Info("NodeJoined", zap.String("nodeId", node.NodeId), zap.String("nodeAddr", node.NodeAddr))
		return nil
	})
	return &raftpb.Status{Updated: err == nil}, err
}

func (t *Handler) GetConfiguration(ctx context.Context, _ struct{}) (*raftpb.RaftConfiguration, error) {
	resp := new(raftpb.RaftConfiguration)
	err := t.doWithRaft(ctx, "GetConfiguration", func(ctx context.Context, r *raft.Raft) error {
		config := r.GetConfiguration()
		var list []*raftpb.RaftServer
		for _, server := range config.Configuration().Servers {
			apiAddr, err := t.RaftClientPool.GetAPIEndpoint(string(server.Address))
			if err != nil {
				return err
			}
			list = append(list, &raftpb.RaftServer{
				NodeId:   string(server.ID),
				RaftAddr: string(server.Address),
				Suffrage: server.Suffrage.String(),
				ApiAddr:  apiAddr,
			})
		}
		resp = &raftpb.RaftConfiguration{
			State:      r.State().String(),
			LastIndex:  config.Index(),
			ServerList: list,
		}
		return nil
	})
	return resp, err
}

func (t *Handler) ApplyCommand(ctx context.Context, cmd *raftpb.Command) (*raftpb.Status, error) {
	status := &raftpb.Status{}
	err := t.doWithRaft(ctx, "ApplyCommand", func(ctx context.Context, r *raft.Raft) error {
		if r.State() != raft.Leader {
			// Forward to the current leader over value-rpc.
			leaderAddress := r.Leader()
			if string(leaderAddress) == "" {
				return ErrRaftLeaderNotFound
			}
			if t.RaftClientPool == nil {
				return xerrors.Errorf("not leader and no client pool configured for forwarding")
			}
			connAny, err := t.RaftClientPool.GetAPIConn(leaderAddress)
			if err != nil {
				return err
			}
			cli, ok := connAny.(valueclient.Client)
			if !ok {
				return xerrors.Errorf("raft client pool returned %T, want valueclient.Client", connAny)
			}
			fwd, err := CallApplyCommand(ctx, cli, cmd)
			if err != nil {
				return err
			}
			status = fwd
			return nil
		}

		start := time.Now()
		f := r.Apply(cmd.Payload, t.Timeout)
		if err := f.Error(); err != nil {
			return err
		}
		resp := f.Response()
		fr, ok := resp.(raftapi.FSMResponse)
		if !ok {
			return xerrors.Errorf("invalid raft response %v", resp)
		}
		if fr.Status != nil {
			status = fr.Status
		}
		status.Updated = fr.Err == nil
		status.Elapsed = time.Since(start).Seconds()
		return fr.Err
	})
	return status, err
}
