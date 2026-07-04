/*
 * Copyright (c) 2025-2026 Karagatan LLC.
 * SPDX-License-Identifier: BUSL-1.1
 */

package raftapi

import (
	"go.arpabet.com/glue"
	"github.com/hashicorp/raft"
	"github.com/hashicorp/serf/cmd/serf/command/agent"
	"github.com/hashicorp/serf/serf"
	"go.arpabet.com/raft/raftpb"
	"go.arpabet.com/servion"
	"reflect"
)

var RaftGrpcServerClass = reflect.TypeOf((*RaftGrpcServer)(nil)).Elem()

type RaftGrpcServer interface {
	glue.InitializingBean
	servion.Component
}

var RaftClientPoolClass = reflect.TypeOf((*RaftClientPool)(nil)).Elem()

type RaftClientPool interface {
	glue.InitializingBean
	glue.DisposableBean

	GetAPIEndpoint(raftAddress string) (string, error)

	// GetAPIConn returns a transport-specific client connection to the control
	// service at raftAddress. The concrete type depends on the pool implementation
	// (a *grpc.ClientConn for the gRPC pool, a value-rpc client for the vrpc pool);
	// callers type-assert to the type their transport expects. Keeping this
	// transport-neutral is what lets raftgrpc and raftvrpc share one raftapi.
	GetAPIConn(raftAddress raft.ServerAddress) (any, error)

	Close() error

}

/**
Finite State Machine Response
 */
type FSMResponse struct {
	Status   *raftpb.Status
	Err      error
}

var RaftServiceClass = reflect.TypeOf((*RaftService)(nil)).Elem()

type RaftService interface {
	glue.InitializingBean
	raft.FSM

}

var RaftServerClass = reflect.TypeOf((*RaftServer)(nil)).Elem()

type RaftServer interface {
	servion.Server
	servion.Component

	Transport() (raft.Transport, bool)

	Raft() (*raft.Raft, bool)

	IsLeader() bool

}

var SerfServerClass = reflect.TypeOf((*SerfServer)(nil)).Elem()

type SerfServer interface {
	servion.Server
	servion.Component

	Config() (*serf.Config, bool)

	Serf() (*serf.Serf, bool)

	Agent() (*agent.Agent, bool)

}
