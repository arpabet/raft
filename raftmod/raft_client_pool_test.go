/*
 * Copyright (c) 2025-2026 Karagatan LLC.
 * SPDX-License-Identifier: BUSL-1.1
 */

package raftmod

import (
	"testing"
	"time"

	"github.com/hashicorp/raft"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// A failed connection attempt must not poison the pool: every subsequent
// GetAPIConn for the same address has to retry (and fail) instead of spinning
// forever on the leftover connecting stub.
func TestGetAPIConnRetriesAfterDialFailure(t *testing.T) {

	pool := &implRaftClientPool{
		Log:         zap.NewNop(),
		DialTimeout: 200 * time.Millisecond,
	}

	// TEST-NET-1 address: connection attempts fail (refused or timed out).
	addr := raft.ServerAddress("127.0.0.1:1")

	_, err := pool.GetAPIConn(addr)
	require.Error(t, err)

	// Before the fix this call spun forever on the closed stub; guard with a
	// timeout so a regression fails fast instead of hanging the suite.
	done := make(chan error, 1)
	go func() {
		_, err := pool.GetAPIConn(addr)
		done <- err
	}()

	select {
	case err := <-done:
		require.Error(t, err)
	case <-time.After(5 * time.Second):
		t.Fatal("GetAPIConn hung after a failed dial: connecting stub was not cleaned up")
	}
}
