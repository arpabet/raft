/*
 * Copyright (c) 2025-2026 Karagatan LLC.
 * SPDX-License-Identifier: BUSL-1.1
 */

package raftgrpc

import (
	"go.arpabet.com/raft/raftapi"
	"go.arpabet.com/raft/raftpb"
	"go.arpabet.com/sprint"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
	"time"
)

var empty = &emptypb.Empty{}

var (
	ErrRaftNotInitialized = status.Errorf(codes.Unavailable, "raft not initialized")
	ErrRaftLeaderNotFound = status.Errorf(codes.Unavailable, "raft leader not found")
)

type implRaftGrpcServer struct {
	raftpb.UnimplementedRaftServiceServer

	GrpcServer *grpc.Server `inject:""`

	AuthorizationMiddleware sprint.AuthorizationMiddleware `inject:""`
	NodeService             sprint.NodeService             `inject:""`
	RaftServer              raftapi.RaftServer             `inject:""`
	RaftService             raftapi.RaftService            `inject:""`
	RaftClientPool          raftapi.RaftClientPool         `inject:""`

	RaftTimeout time.Duration `value:"raft.timeout,default=10s"`

	Log *zap.Logger `inject:""`
}

func RaftGrpcServer() raftapi.RaftGrpcServer {
	return &implRaftGrpcServer{}
}

func (t *implRaftGrpcServer) PostConstruct() error {
	raftpb.RegisterRaftServiceServer(t.GrpcServer, t)
	return nil
}

func (t *implRaftGrpcServer) BeanName() string {
	return "raft-grpc-server"
}

func (t *implRaftGrpcServer) GetStats(cb func(name, value string) bool) error {
	return nil
}
