/*
 * Copyright (c) 2025-2026 Karagatan LLC.
 * SPDX-License-Identifier: BUSL-1.1
 */

package raftvrpc

import (
	"context"

	"github.com/hashicorp/raft"
	"go.arpabet.com/value-rpc/valuerpc"
	"go.uber.org/zap"
	"golang.org/x/xerrors"
)

// doWithRaft runs cb with the live *raft.Raft, gated by doAuthorized.
func (t *Handler) doWithRaft(ctx context.Context, methodName string, cb func(ctx context.Context, r *raft.Raft) error) error {
	return t.doAuthorized(ctx, methodName, func(ctx context.Context) error {
		r, ok := t.RaftServer.Raft()
		if !ok {
			return ErrRaftNotInitialized
		}
		return cb(ctx, r)
	})
}

// doAuthorized enforces the ADMIN gate (when an Auth middleware is configured)
// and recovers panics into errors. When Auth is nil the gate is skipped — the
// transport (mTLS) is expected to authenticate peers.
func (t *Handler) doAuthorized(ctx context.Context, methodName string, cb func(ctx context.Context) error) (err error) {
	if t.Auth != nil {
		user, ok := t.Auth.GetUser(ctx)
		if !ok || !user.Roles["ADMIN"] {
			return valuerpc.NewError(valuerpc.CodeUnauthenticated, "role ADMIN is required")
		}
	}

	defer func() {
		if r := recover(); r != nil {
			switch v := r.(type) {
			case error:
				err = v
			case string:
				err = xerrors.New(v)
			default:
				err = xerrors.Errorf("%v", v)
			}
		}
		if err != nil {
			t.Log.Error(methodName, zap.Error(err))
		}
	}()

	return cb(ctx)
}
