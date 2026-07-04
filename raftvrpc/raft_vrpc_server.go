/*
 * Copyright (c) 2025-2026 Karagatan LLC.
 * SPDX-License-Identifier: BUSL-1.1
 */

package raftvrpc

import (
	"time"

	"go.arpabet.com/glue"
	"go.arpabet.com/raft/raftapi"
	"go.arpabet.com/value-rpc/valueserver"
	"go.uber.org/zap"
)

// ControlServer is the glue bean that registers the raft control operations on
// an injected value-rpc server. It is the value-rpc analogue of raftgrpc's
// RaftGrpcServer.
type ControlServer struct {
	Server valueserver.Server `inject:""`

	NodeService    raftapi.NodeService    `inject:""`
	RaftServer     raftapi.RaftServer     `inject:""`
	RaftService    raftapi.RaftService    `inject:""`
	RaftClientPool raftapi.RaftClientPool `inject:""`

	// Optional: when present, the control API is gated to ADMIN callers.
	Auth raftapi.AuthorizationMiddleware `inject:"optional"`

	RaftTimeout time.Duration `value:"raft.timeout,default=10s"`

	Log *zap.Logger `inject:""`
}

// RaftVrpcServer constructs the control-server bean.
func RaftVrpcServer() *ControlServer {
	return &ControlServer{}
}

var _ glue.InitializingBean = (*ControlServer)(nil)

func (t *ControlServer) PostConstruct() error {
	return Register(t.Server, &Handler{
		NodeService:    t.NodeService,
		RaftServer:     t.RaftServer,
		RaftService:    t.RaftService,
		RaftClientPool: t.RaftClientPool,
		Auth:           t.Auth,
		Timeout:        t.RaftTimeout,
		Log:            t.Log,
	})
}

func (t *ControlServer) BeanName() string { return "raft-vrpc-server" }

func (t *ControlServer) GetStats(cb func(name, value string) bool) error { return nil }
