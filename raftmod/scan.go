/*
 * Copyright (c) 2025-2026 Karagatan LLC.
 * SPDX-License-Identifier: BUSL-1.1
 */

package raftmod

var RaftServices = []interface{}{
	NodeService(),
	RaftLogStoreFactory(),
	RaftStableStoreFactory(),
	RaftSnapshotFactory(),
	SerfConfigFactory(),
	ServerLookup(),
	SerfRPCServer(),
	RaftServer(),
	RaftClientPool(),
}
