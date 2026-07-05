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
	stdatomic "sync/atomic"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/raft"
	"go.arpabet.com/cligo"
	"go.arpabet.com/glue"
	"go.arpabet.com/raft/raftapi"
	"go.arpabet.com/servion"
	"go.uber.org/atomic"
	"go.uber.org/zap"
	"golang.org/x/xerrors"
)

type implRaftServer struct {
	Properties glue.Properties `inject:""`
	Log        *zap.Logger     `inject:""`
	HCLog      hclog.Logger    `inject:""`
	// TlsConfig, when a bean named "raft-transport-tls" is present, secures the
	// consensus transport with mutual TLS (log replication between peers). The
	// qualifier keeps it distinct from the control-plane pool's config, so a
	// deployment can secure the two channels independently.
	TlsConfig *tls.Config `inject:"optional,bean=raft-transport-tls"`

	CliApp      cligo.CliApplication `inject:"optional"`
	NodeService raftapi.NodeService  `inject:""`

	AppName string `value:"application.name,default="`

	LogStore          raft.LogStore      `inject:""`
	StableStore       raft.StableStore   `inject:""`
	FileSnapshotStore raft.SnapshotStore `inject:""`

	ServerLookup raftapi.ServerLookup `inject:""`

	SerfAddress   string `value:"serf.bind-address,default="`
	SerfQueueSize int    `value:"serf.queue-size,default=2048"`

	//SerfConfig   *serf.Config `inject:""`
	//serf         *serf.Serf
	//serfChLAN    chan  serf.Event

	// reconcileCh is used to pass events from the serf handler
	// into the leader manager, so that the strong state can be
	// updated
	// reconcileCh chan serf.Member

	// should be defined by application
	FSM raft.FSM `inject:""`

	RaftAddress string        `value:"raft.bind-address,default="`
	MaxPool     int           `value:"raft.max-pool,default=3"`
	Timeout     time.Duration `value:"raft.timeout,default=10s"`

	listener net.Listener

	// transport and raft are written by Bind/Serve (servion lifecycle goroutine)
	// and polled concurrently through Transport()/Raft() (e.g. by bootstrap
	// watchers), so they are atomic pointers.
	transport stdatomic.Pointer[raft.NetworkTransport]
	raft      stdatomic.Pointer[raft.Raft]

	alive        atomic.Bool
	shutdownOnce sync.Once
	shutdownCh   chan struct{}
}

func RaftServer() raftapi.RaftServer {
	return &implRaftServer{
		shutdownCh: make(chan struct{}),
	}
}

func (t *implRaftServer) PostConstruct() error {
	//t.serfChLAN = make(chan serf.Event, t.SerfQueueSize)
	//t.SerfConfig.EventCh = t.serfChLAN
	return nil
}

func (t *implRaftServer) BeanName() string {
	return "raft-server"
}

// applicationName is the cluster role name; must match the serf "role" tag
// stamped by SerfConfigFactory.
func (t *implRaftServer) applicationName() string {
	return resolveApplicationName(t.AppName, t.CliApp)
}

func (t *implRaftServer) GetStats(cb func(name, value string) bool) error {
	if r := t.raft.Load(); r != nil {
		for k, v := range r.Stats() {
			cb(k, v)
		}
	}
	return nil
}

func (t *implRaftServer) Bind() (err error) {

	if t.RaftAddress == "" {
		t.Log.Warn("RaftAddressEmpty", zap.String("prop", "raft.bind-address"))
		return nil
	}

	if t.SerfAddress == "" {
		t.Log.Warn("SerfAddressEmpty", zap.String("prop", "serf.bind-address"))
		return nil
	}

	raftAddr, err := ParseAndAdjustTCPAddr(t.RaftAddress, t.NodeService.NodeSeq())
	if err != nil {
		return xerrors.Errorf("issue in property 'raft.bind-address', %v", err)
	}
	t.RaftAddress = fmt.Sprintf("%s:%d", raftAddr.IP.String(), raftAddr.Port)

	t.listener, err = net.Listen("tcp", t.RaftAddress)
	if err != nil {
		return xerrors.Errorf("bind failed on '%s', %v", t.RaftAddress, err)
	}
	if t.TlsConfig != nil {
		// Mutual TLS on the consensus transport: accepted connections are wrapped as
		// TLS servers, so the peer's client certificate is required and verified
		// (TlsConfig carries ClientCAs + RequireAndVerifyClientCert). The Dial side
		// (TCPStreamLayer.Dial) verifies the server certificate symmetrically.
		t.listener = tls.NewListener(t.listener, t.TlsConfig)
	}

	advertise, err := net.ResolveTCPAddr("tcp", ReplaceToPrivateIP(t.RaftAddress))
	if err != nil {
		return xerrors.Errorf("tcp address resolve '%s', %v", t.listener.Addr().String(), err)
	}

	t.Log.Info("RaftServerFactory", zap.String("bind", t.listener.Addr().String()), zap.String("advertise", advertise.String()))

	transport, err := newTCPTransport(t.listener, advertise, t.TlsConfig, func(stream raft.StreamLayer) *raft.NetworkTransport {
		config := &raft.NetworkTransportConfig{Stream: stream, MaxPool: t.MaxPool, Timeout: t.Timeout, Logger: t.HCLog.Named("raft-transport"),
			ServerAddressProvider: t.ServerLookup}
		return raft.NewNetworkTransportWithConfig(config)

		//return raft.NewNetworkTransport(stream, t.MaxPool, t.Timeout, os.Stderr)
	})
	if err != nil {
		return xerrors.Errorf("raft transport creation error for address '%s', %v", advertise.String(), err)
	}
	t.transport.Store(transport)

	return nil
}

func (t *implRaftServer) Alive() bool {
	return t.alive.Load()
}

func (t *implRaftServer) Transport() (raft.Transport, bool) {
	tr := t.transport.Load()
	if tr == nil {
		return nil, false
	}
	return tr, true
}

func (t *implRaftServer) Raft() (*raft.Raft, bool) {
	r := t.raft.Load()
	return r, r != nil
}

func (t *implRaftServer) IsLeader() bool {
	r := t.raft.Load()
	return t.alive.Load() && r != nil && r.State() == raft.Leader
}

func (t *implRaftServer) ListenAddress() net.Addr {
	if t.listener != nil {
		return t.listener.Addr()
	} else {
		return servion.EmptyAddr
	}
}

func (t *implRaftServer) Serve() (err error) {

	defer panicToError(&err)

	transport := t.transport.Load()
	if transport == nil {
		// Bind was skipped (no raft/serf bind address configured): stay dormant
		// instead of panicking inside raft.NewRaft with a nil transport. Serve
		// must still block: servion shuts the whole application down as soon
		// as the first server returns.
		t.Log.Warn("RaftServerDormant", zap.String("reason", "transport not initialized, check 'raft.bind-address' and 'serf.bind-address'"))
		<-t.shutdownCh
		return nil
	}

	t.Log.Info("RaftServerServe", zap.String("addr", t.RaftAddress), zap.Bool("tls", t.TlsConfig != nil))

	t.alive.Store(false)

	config := raft.DefaultConfig()
	config.LocalID = raft.ServerID(t.NodeService.NodeIdHex())
	config.Logger = t.HCLog.Named("raft")

	r, err := raft.NewRaft(config, t.FSM, t.LogStore, t.StableStore, t.FileSnapshotStore, transport)
	if err != nil {
		return err
	}
	t.raft.Store(r)

	/*
		t.serf, err = serf.Create(t.SerfConfig)
		if err != nil {
			t.Log.Error("SerfCreate", zap.String("action", "shutdown raft"), zap.Error(err))
			t.raft.Shutdown()
			return err
		}

		for _, m := range t.serf.Members() {
			t.Log.Info("Member", zap.Any("member", m))
			server, err := ParseServerTags(m, t.Application.Name())
			if err != nil {
				t.Log.Debug("ParseServerTags", zap.Any("member", m), zap.Error(err))
				continue
			}
			t.ServerLookup.AddServer(server)
		}

		serfAddr := fmt.Sprintf("%s:%d", t.SerfConfig.MemberlistConfig.BindAddr, t.SerfConfig.MemberlistConfig.BindPort)
		t.Log.Info("SerfServerServe", zap.String("addr", serfAddr), zap.Any("stats", t.serf.Stats()))
	*/

	t.alive.Store(true)

	// Block until Shutdown: the servion runtime stops all servers as soon as
	// the first Serve returns, and the raft node runs in the background.
	<-t.shutdownCh
	return nil
}

func (t *implRaftServer) Shutdown() error {
	t.alive.Store(false)

	t.shutdownOnce.Do(func() {

		t.Log.Info("RaftServerShutdown", zap.String("addr", t.RaftAddress))
		close(t.shutdownCh)
		/*
			if t.serf != nil {
				if err := t.serf.Leave(); err != nil {
					t.Log.Error("SerfLeave", zap.Error(err))
				}
				if err := t.serf.Shutdown(); err != nil {
					t.Log.Error("SerfShutdown", zap.Error(err))
				}
			}
		*/

		if r := t.raft.Load(); r != nil {
			future := r.Shutdown()
			go func() {
				if err := future.Error(); err != nil {
					t.Log.Error("RaftShutdown", zap.Error(err))
				}
			}()
		}
		if transport := t.transport.Load(); transport != nil {
			transport.Close()
		}
		if t.listener != nil {
			t.listener.Close()
		}
	})

	return nil
}

func (t *implRaftServer) ShutdownCh() <-chan struct{} {
	return t.shutdownCh
}

func (t *implRaftServer) Destroy() error {
	t.Shutdown()
	return nil
}
