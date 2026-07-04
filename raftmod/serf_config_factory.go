/*
 * Copyright (c) 2025-2026 Karagatan LLC.
 * SPDX-License-Identifier: BUSL-1.1
 */

package raftmod

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strconv"

	"github.com/hashicorp/serf/serf"
	"go.arpabet.com/cligo"
	"go.arpabet.com/glue"
	"go.arpabet.com/raft/raftapi"
	"go.arpabet.com/servion"
	"go.uber.org/zap"
	"golang.org/x/xerrors"
)

var SerfConfigClass = reflect.TypeOf((*serf.Config)(nil))

type implSerfConfigFactory struct {
	Log        *zap.Logger     `inject:""`
	Properties glue.Properties `inject:""`

	CliApp      cligo.CliApplication `inject:"optional"`
	Runtime     servion.Runtime      `inject:"optional"`
	NodeService raftapi.NodeService  `inject:""`

	AppName    string `value:"application.name,default="`
	AppVersion string `value:"application.version,default="`
	AppBuild   string `value:"application.build,default="`

	SerfAddress string `value:"serf.bind-address,default="`
	RaftAddress string `value:"raft.bind-address,default="`
	RPCBean     string `value:"raft.rpc-bean-name,default="`

	DataDir      string      `value:"application.data.dir,default="`
	DataDirPerm  os.FileMode `value:"application.perm.data.dir,default=-rwxrwx---"`
	DataFilePerm os.FileMode `value:"application.perm.data.file,default=-rw-rw-r--"`
}

func SerfConfigFactory() glue.FactoryBean {
	return &implSerfConfigFactory{}
}

func (t *implSerfConfigFactory) Object() (object interface{}, err error) {

	defer panicToError(&err)

	dataDir := t.DataDir
	if dataDir == "" {
		dataDir = filepath.Join(homeDir(t.Runtime), "db")

		if err := createDirIfNeeded(dataDir, t.DataDirPerm); err != nil {
			return nil, err
		}

		dataDir = filepath.Join(dataDir, t.NodeService.LocalName())
	}

	if err := createDirIfNeeded(dataDir, t.DataDirPerm); err != nil {
		return nil, err
	}

	snapshotFolder := filepath.Join(dataDir, "serf")

	if err := createDirIfNeeded(snapshotFolder, t.DataDirPerm); err != nil {
		return nil, err
	}

	conf := serf.DefaultConfig()
	conf.Init()

	conf.NodeName = t.NodeService.LANName()
	conf.SnapshotPath = filepath.Join(snapshotFolder, "local.snapshot")

	conf.Logger = zap.NewStdLog(t.Log.Named("serf"))

	conf.Tags["id"] = t.NodeService.NodeIdHex()
	conf.Tags["role"] = resolveApplicationName(t.AppName, t.CliApp)
	conf.Tags["version"] = t.applicationVersion()
	conf.Tags["build"] = t.applicationBuild()

	if t.SerfAddress == "" {
		return nil, xerrors.New("required property 'serf.bind-address' is empty")
	}

	tcpAddr, err := ParseAndAdjustTCPAddr(t.SerfAddress, t.NodeService.NodeSeq())
	if err != nil {
		return nil, xerrors.Errorf("issue in property 'serf.bind-address', %v", err)
	}

	memberConfig := conf.MemberlistConfig

	memberConfig.BindAddr = tcpAddr.IP.String()
	memberConfig.BindPort = tcpAddr.Port

	conf.Tags["port"] = strconv.Itoa(tcpAddr.Port)

	// The advertised ports must match what the servers actually bind, and every
	// server (raft, rpc) adjusts its configured port by NodeSeq — so the tags
	// must be adjusted the same way, or multi-node-per-host setups gossip ports
	// belonging to another node.
	if t.RaftAddress != "" {
		raftAddr, err := ParseAndAdjustTCPAddr(t.RaftAddress, t.NodeService.NodeSeq())
		if err != nil {
			return nil, xerrors.Errorf("invalid port in property 'raft.bind-address', %v", err)
		}
		conf.Tags["raft-port"] = strconv.Itoa(raftAddr.Port)
	}

	if t.RPCBean != "" {
		propName := fmt.Sprintf("%s.%s", t.RPCBean, "bind-address")
		value := t.Properties.GetString(propName, "")
		if value == "" {
			return nil, xerrors.Errorf("empty property '%s' needed by 'raft.rpc-bean-name' reference", propName)
		}
		rpcAddr, err := ParseAndAdjustTCPAddr(value, t.NodeService.NodeSeq())
		if err != nil {
			return nil, xerrors.Errorf("invalid port in property '%s', %v", propName, err)
		}
		conf.Tags["grpc-port"] = strconv.Itoa(rpcAddr.Port)
	}

	return conf, nil
}

func (t *implSerfConfigFactory) applicationVersion() string {
	if t.AppVersion != "" {
		return t.AppVersion
	}
	if t.CliApp != nil {
		return t.CliApp.Version()
	}
	return ""
}

func (t *implSerfConfigFactory) applicationBuild() string {
	if t.AppBuild != "" {
		return t.AppBuild
	}
	if t.CliApp != nil {
		return t.CliApp.Build()
	}
	return ""
}

func (t *implSerfConfigFactory) ObjectType() reflect.Type {
	return SerfConfigClass
}

func (t *implSerfConfigFactory) ObjectName() string {
	return "serf-config"
}

func (t *implSerfConfigFactory) Singleton() bool {
	return true
}
