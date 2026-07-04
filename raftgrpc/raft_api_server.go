/*
 * Copyright (c) 2025-2026 Karagatan LLC.
 * SPDX-License-Identifier: BUSL-1.1
 */

package raftgrpc

import (
	"context"
	"io"
	"time"

	"github.com/hashicorp/raft"
	"go.arpabet.com/raft/raftapi"
	"go.arpabet.com/raft/raftpb"
	"go.uber.org/zap"
	"golang.org/x/xerrors"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/emptypb"
)

func (t *implRaftGrpcServer) Bootstrap(ctx context.Context, req *emptypb.Empty) (resp *emptypb.Empty, err error) {

	return empty, t.doWithRaft(ctx, "Bootstrap", func(ctx context.Context, r *raft.Raft) error {

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
			Servers: []raft.Server{
				{
					ID:      config.LocalID,
					Address: tr.LocalAddr(),
				},
			},
		}

		t.Log.Info("Bootstrap", zap.String("id", t.NodeService.NodeIdHex()), zap.String("addr", string(tr.LocalAddr())))

		return r.BootstrapCluster(configuration).Error()

	})

}

func (t *implRaftGrpcServer) Join(ctx context.Context, node *raftpb.RaftNode) (resp *emptypb.Empty, err error) {

	return empty, t.doWithRaft(ctx, "Join", func(ctx context.Context, r *raft.Raft) error {

		configFuture := r.GetConfiguration()
		if err := configFuture.Error(); err != nil {
			t.Log.Error("GetConfiguration", zap.Error(err))
			return err
		}

		for _, srv := range configFuture.Configuration().Servers {
			// If a node already exists with either the joining node's ID or address,
			// that node may need to be removed from the config first.
			if srv.ID == raft.ServerID(node.NodeId) || srv.Address == raft.ServerAddress(node.NodeAddr) {
				// However if *both* the ID and the address are the same, then nothing -- not even
				// a join operation -- is needed.
				if srv.Address == raft.ServerAddress(node.NodeAddr) && srv.ID == raft.ServerID(node.NodeId) {
					t.Log.Info("AlreadyMember", zap.String("node", node.String()))
					return nil
				}

				future := r.RemoveServer(srv.ID, 0, 0)
				if err := future.Error(); err != nil {
					t.Log.Error("RemoveExistingNode", zap.String("node", node.String()), zap.Error(err))
					return xerrors.Errorf("removing existing node %s at %s: %v", node.NodeId, node.NodeAddr, err)
				}
			}
		}

		f := r.AddVoter(raft.ServerID(node.NodeId), raft.ServerAddress(node.NodeAddr), 0, 0)
		if f.Error() != nil {
			return f.Error()
		}

		t.Log.Info("NodeJoined", zap.String("nodeId", node.NodeId), zap.String("nodeAddr", node.NodeAddr))
		return nil

	})

}

func (t *implRaftGrpcServer) GetConfiguration(ctx context.Context, req *emptypb.Empty) (resp *raftpb.RaftConfiguration, err error) {

	resp = new(raftpb.RaftConfiguration)
	err = t.doWithRaft(ctx, "GetConfiguration", func(ctx context.Context, r *raft.Raft) error {
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

	return

}

func (t *implRaftGrpcServer) ApplyCommand(ctx context.Context, cmd *raftpb.Command) (status *raftpb.Status, err error) {

	status = new(raftpb.Status)
	err = t.doWithRaft(ctx, "ApplyCommand", func(ctx context.Context, r *raft.Raft) error {

		if r.State() != raft.Leader {
			leaderAddress := r.Leader()
			if string(leaderAddress) == "" {
				return ErrRaftLeaderNotFound
			}
			leaderConnAny, err := t.RaftClientPool.GetAPIConn(leaderAddress)
			if err != nil {
				return err
			}
			leaderConn, ok := leaderConnAny.(*grpc.ClientConn)
			if !ok {
				return xerrors.Errorf("raft client pool returned %T, want *grpc.ClientConn", leaderConnAny)
			}
			leaderClient := raftpb.NewRaftServiceClient(leaderConn)
			status, err = leaderClient.ApplyCommand(ctx, cmd)
			return err
		}

		start := time.Now()
		f := r.Apply(cmd.Payload, t.RaftTimeout)
		err = f.Error()

		if err != nil {
			return err
		}

		fr, ok := asFSMResponse(f.Response())
		if !ok {
			return xerrors.Errorf("invalid raft response %v", f.Response())
		}
		if fr.Status != nil {
			status = fr.Status
		}
		status.Elapsed = time.Since(start).Seconds()
		return fr.Err
	})

	return
}

// asFSMResponse accepts the raftapi.FSMResponse contract by value or by pointer,
// since application FSMs return either form.
func asFSMResponse(resp interface{}) (raftapi.FSMResponse, bool) {
	switch v := resp.(type) {
	case raftapi.FSMResponse:
		return v, true
	case *raftapi.FSMResponse:
		if v != nil {
			return *v, true
		}
	}
	return raftapi.FSMResponse{}, false
}

func (t *implRaftGrpcServer) Recover(stream raftpb.RaftService_RecoverServer) (err error) {

	return t.doWithRaft(stream.Context(), "Recover", func(ctx context.Context, r *raft.Raft) error {

		channel := make(chan []byte)
		reader := &channelReader{incoming: channel}
		result := make(chan error, 1)

		go func() {
			result <- t.recoverFromSnapshot(r, reader)
		}()

		var recvErr error
		for {
			content, e := stream.Recv()
			if e == io.EOF {
				break
			}
			if e != nil {
				recvErr = e
				break
			}
			select {
			case channel <- content.Content:
			case restoreErr := <-result:
				// The restore ended while the client is still streaming — surface
				// its error instead of blocking forever on an unread channel.
				if restoreErr == nil {
					restoreErr = xerrors.New("snapshot restore finished before stream completed")
				}
				return restoreErr
			}
		}

		// End the reader's stream: with a clean io.EOF after a complete upload, or
		// with the receive error so a half-uploaded snapshot is never restored.
		reader.err = recvErr
		close(channel)

		restoreErr := <-result
		if recvErr != nil {
			return recvErr
		}
		if restoreErr != nil {
			return restoreErr
		}
		return stream.SendAndClose(empty)

	})
}

func (t *implRaftGrpcServer) recoverFromSnapshot(r *raft.Raft, reader io.ReadCloser) error {

	/**
	This function only safe for empty cluster
	*/

	if r == nil {
		t.Log.Warn("RecoverFSMDirectly", zap.String("status", "raft not initialized"))
		return t.RaftService.Restore(reader)
	}

	// make copy of previous snapshot
	meta, source, err := r.Snapshot().Open()
	if err != nil {
		return err
	}
	source.Close()

	// break sequence
	meta.Index = r.LastIndex() + 2

	return r.Restore(meta, reader, 0)
}
