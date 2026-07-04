/*
 * Copyright (c) 2025-2026 Karagatan LLC.
 * SPDX-License-Identifier: BUSL-1.1
 */

package raftapi

import (
	"reflect"

	"go.arpabet.com/glue"
	"go.arpabet.com/uuid"
)

var NodeServiceClass = reflect.TypeOf((*NodeService)(nil)).Elem()

/**
NodeService provides the identity of the local node inside the cluster.

raftmod ships a default property-driven implementation (raftmod.NodeService());
applications may register their own bean instead.
*/
type NodeService interface {
	glue.NamedBean

	/**
	Returns the node id unique number. Usually derived on the first startup
	and stable across restarts.
	*/
	NodeId() uint64

	/**
	Returns the node id in hex format. Used as the raft ServerID and the
	serf "id" tag.
	*/
	NodeIdHex() string

	/**
	Returns the node sequence number. Servers adjust their configured bind
	ports by this number, so multiple nodes can run on one host.
	*/
	NodeSeq() int

	/**
	Returns the node local name, a combination of the application name and
	the node sequence number. Used for per-node data directories.
	*/
	LocalName() string

	/**
	Returns the node name announced in the gossip (serf) protocol. Must be
	unique across the cluster.
	*/
	LANName() string

	/**
	Issues a random time-based UUID carrying the node id. Used to identify
	ordered events in distributed systems.
	*/
	Issue() uuid.UUID
}
