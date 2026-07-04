/*
 * Copyright (c) 2025-2026 Karagatan LLC.
 * SPDX-License-Identifier: BUSL-1.1
 */

package raftmod

import (
	"hash/fnv"
	"os"
	"strconv"
	"time"

	"go.arpabet.com/cligo"
	"go.arpabet.com/raft/raftapi"
	"go.arpabet.com/uuid"
)

/**
Default property-driven raftapi.NodeService implementation.

Node identity is resolved from properties:
  node.id    pins a stable node id; when zero the id is derived
             deterministically from name, hostname and sequence number,
             so it survives restarts on the same host.
  node.name  base node name; defaults to the cligo application name.
  node.seq   node sequence number, used to adjust bind ports when several
             nodes run on one host.
  node.lan   overrides the gossip (serf) node name.

Applications can replace this bean by registering their own
raftapi.NodeService implementation.
*/
type implNodeService struct {
	CliApp cligo.CliApplication `inject:"optional"`

	NodeIdProp  uint64 `value:"node.id,default=0"`
	NodeName    string `value:"node.name,default="`
	NodeSeqNum  int    `value:"node.seq,default=0"`
	LANNameProp string `value:"node.lan,default="`

	AppName string `value:"application.name,default="`

	nodeId   uint64
	baseName string
}

func NodeService() raftapi.NodeService {
	return &implNodeService{}
}

func (t *implNodeService) PostConstruct() error {

	t.baseName = t.NodeName
	if t.baseName == "" {
		t.baseName = resolveApplicationName(t.AppName, t.CliApp)
	}

	if t.NodeIdProp != 0 {
		t.nodeId = t.NodeIdProp
	} else {
		h := fnv.New64a()
		host, _ := os.Hostname()
		h.Write([]byte(t.baseName + ":" + host + ":" + strconv.Itoa(t.NodeSeqNum)))
		t.nodeId = h.Sum64()
	}

	return nil
}

func (t *implNodeService) BeanName() string {
	return "node-service"
}

func (t *implNodeService) GetStats(cb func(name, value string) bool) error {
	cb("node.id", t.NodeIdHex())
	cb("node.seq", strconv.Itoa(t.NodeSeqNum))
	cb("node.name", t.LocalName())
	cb("node.lan", t.LANName())
	return nil
}

func (t *implNodeService) NodeId() uint64 {
	return t.nodeId
}

func (t *implNodeService) NodeIdHex() string {
	return strconv.FormatUint(t.nodeId, 16)
}

func (t *implNodeService) NodeSeq() int {
	return t.NodeSeqNum
}

func (t *implNodeService) LocalName() string {
	if t.NodeSeqNum == 0 {
		return t.baseName
	}
	return t.baseName + "-" + strconv.Itoa(t.NodeSeqNum)
}

func (t *implNodeService) LANName() string {
	if t.LANNameProp != "" {
		return t.LANNameProp
	}
	return t.LocalName()
}

func (t *implNodeService) Issue() uuid.UUID {
	return uuid.Create(int64(t.nodeId), time.Now().UnixNano())
}
