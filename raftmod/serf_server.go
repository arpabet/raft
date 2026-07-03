/*
 * Copyright (c) 2025-2026 Karagatan LLC.
 * SPDX-License-Identifier: BUSL-1.1
 */

package raftmod

import (
	"crypto/tls"
	"fmt"
	"net"
	"sync"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/serf/cmd/serf/command/agent"
	"github.com/hashicorp/serf/serf"
	"go.arpabet.com/glue"
	"go.arpabet.com/raft/raftapi"
	"go.arpabet.com/sprint"
	"go.uber.org/atomic"
	"go.uber.org/zap"
	"golang.org/x/xerrors"
)

type implSerfServer struct {
	Properties  glue.Properties    `inject:""`
	Log         *zap.Logger        `inject:""`
	HCLog       hclog.Logger       `inject:""`
	TlsConfig   *tls.Config        `inject:"optional"`
	NodeService sprint.NodeService `inject:""`

	SerfConfig  *serf.Config `inject:""`
	agentConfig *agent.Config

	listener  net.Listener
	serfAgent *agent.Agent
	ipc       *agent.AgentIPC

	EventHandlers []agent.EventHandler `inject:""`

	/**
	RPCAddr is the address and port to listen on for the agent's RPC interface.
	*/

	RPCAddress string `value:"serf.rpc-address,default=:8700"`

	/**
	RPCAuthKey is a key that can be set to optionally require that
	RPC's provide an authentication key.
	*/
	RPCAuthKey string `value:"serf.rpc-auth,default="`

	/**
	Discover is used to setup an mDNS Discovery name. When this is set, the
	Serf agent will setup an mDNS responder and periodically run an mDNS query
	to look for peers. For peers on a network that supports multicast, this
	allows Serf agents to join each other with zero configuration.
	*/
	Discover string `value:"serf.discover,default="`

	/**
	Interface is used to provide a binding interface to use. It can be used instead of
	providing a bind address, as Serf will discover the address of the provided interface.
	It is also used to set the multicast device used with `discover`.
	*/
	Interface string `value:"serf.iface,default="`

	alive        atomic.Bool
	shutdownOnce sync.Once
	shutdownCh   chan struct{}
}

func SerfRPCServer() raftapi.SerfServer {
	return &implSerfServer{
		shutdownCh: make(chan struct{}),
	}
}

func (t *implSerfServer) PostConstruct() (err error) {
	t.agentConfig = agent.DefaultConfig()
	t.agentConfig.BindAddr = fmt.Sprintf("%s:%d", t.SerfConfig.MemberlistConfig.BindAddr, t.SerfConfig.MemberlistConfig.BindPort)
	t.agentConfig.RPCAddr = t.RPCAddress
	t.agentConfig.RPCAuthKey = t.RPCAuthKey
	t.agentConfig.EnableCompression = t.SerfConfig.MemberlistConfig.EnableCompression
	t.agentConfig.Tags = t.SerfConfig.Tags

	t.serfAgent, err = agent.Create(t.agentConfig, t.SerfConfig, t.SerfConfig.LogOutput)
	if err != nil {
		return xerrors.Errorf("failed to create the Serf agent, %v", err)
	}

	for _, eh := range t.EventHandlers {
		t.Log.Info("RegisterEventHandler", zap.Any("eh", eh))
		t.serfAgent.RegisterEventHandler(eh)
	}

	return nil
}

func (t *implSerfServer) BeanName() string {
	return "serf-rpc-server"
}

func (t *implSerfServer) GetStats(cb func(name, value string) bool) error {
	return nil
}

func (t *implSerfServer) Bind() error {

	tcpAddr, err := ParseAndAdjustTCPAddr(t.agentConfig.RPCAddr, t.NodeService.NodeSeq())
	if err != nil {
		return err
	}
	t.RPCAddress = fmt.Sprintf("%s:%d", tcpAddr.IP.String(), tcpAddr.Port)
	t.agentConfig.RPCAddr = t.RPCAddress

	// Setup the RPC listener
	t.listener, err = net.Listen("tcp", t.agentConfig.RPCAddr)
	if err != nil {
		return xerrors.Errorf("failed to bind on address '%s', %v", t.agentConfig.RPCAddr, err)
	}

	return nil
}

func (t *implSerfServer) Alive() bool {
	return t.alive.Load()
}

func (t *implSerfServer) ListenAddress() net.Addr {
	if t.listener != nil {
		return t.listener.Addr()
	} else {
		return sprint.EmptyAddr
	}
}

func (t *implSerfServer) Serve() (err error) {

	panicToError(&err)

	t.Log.Info("SerfRPCServerServe", zap.String("addr", t.RPCAddress), zap.Bool("tls", t.TlsConfig != nil))

	t.alive.Store(false)
	err = t.serfAgent.Start()
	if err != nil {
		return err
	}

	// serf v0.10.2 added msgpackUseNewTimeFormat; false matches serf's own
	// DefaultConfig (decoders accept both formats, so this stays compatible).
	t.ipc = agent.NewAgentIPC(t.serfAgent, t.RPCAuthKey, t.listener, t.SerfConfig.LogOutput, agent.NewLogWriter(512), false)
	t.alive.Store(true)

	return nil
}

func (t *implSerfServer) Shutdown() (err error) {
	t.alive.Store(false)

	t.shutdownOnce.Do(func() {

		t.Log.Info("SerfServerShutdown", zap.String("addr", t.RPCAddress))
		close(t.shutdownCh)

		if t.ipc != nil {
			t.ipc.Shutdown()
		}
		if t.serfAgent != nil {

			if err := t.serfAgent.Serf().Leave(); err != nil {
				t.Log.Error("SerfLeave", zap.Error(err))
			}

			err = t.serfAgent.Shutdown()
		}
		if t.listener != nil {
			t.listener.Close()
		}

	})

	return
}

func (t *implSerfServer) Config() (*serf.Config, bool) {
	return t.SerfConfig, t.SerfConfig != nil
}

func (t *implSerfServer) Serf() (*serf.Serf, bool) {
	return t.serfAgent.Serf(), t.serfAgent != nil
}

func (t *implSerfServer) Agent() (*agent.Agent, bool) {
	return t.serfAgent, t.serfAgent != nil
}

func (t *implSerfServer) ShutdownCh() <-chan struct{} {
	return t.shutdownCh
}

func (t *implSerfServer) Destroy() error {
	t.Shutdown()
	return nil
}
