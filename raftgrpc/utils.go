/*
 * Copyright (c) 2025-2026 Karagatan LLC.
 * SPDX-License-Identifier: BUSL-1.1
 */

package raftgrpc

import (
	"context"
	"io"
	"strings"

	"github.com/hashicorp/raft"
	"go.arpabet.com/raft/raftpb"
	"go.uber.org/zap"
	"golang.org/x/xerrors"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func (t *implRaftGrpcServer) doWithRaft(ctx context.Context, methodName string, cb func(ctx context.Context, r *raft.Raft) error) (err error) {

	return t.doAuthorized(ctx, methodName, func(ctx context.Context) error {

		r, ok := t.RaftServer.Raft()
		if !ok {
			return ErrRaftNotInitialized
		}

		return cb(ctx, r)

	})

}

func (t *implRaftGrpcServer) doAuthorized(ctx context.Context, methodName string, cb func(ctx context.Context) error) (err error) {

	user, ok := t.AuthorizationMiddleware.GetUser(ctx)
	if !ok || !user.Roles["ADMIN"] {
		return status.Errorf(codes.Unauthenticated, "role USER is required")
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
			err = t.wrapError(err, methodName, user.Username)
		}
	}()

	return cb(ctx)
}

func (t *implRaftGrpcServer) wrapError(err error, method, username string) error {
	if _, ok := status.FromError(err); ok {
		return err
	}
	issue := err.Error()
	if strings.HasPrefix(issue, "nowrap:") {
		issue = strings.TrimSpace(strings.TrimPrefix(issue, "nowrap:"))
		return xerrors.New(issue)
	}
	message := "internal error"
	if strings.Contains("concurrent transaction", issue) {
		message = "concurrent transaction"
	} else if strings.Contains("not found", issue) {
		message = "object not found"
	} else if strings.Contains("exist", issue) {
		message = "object already exist"
	}
	id := t.NodeService.Issue().String()
	t.Log.Error(method, zap.String("errorId", id), zap.Any("username", username), zap.Error(err))
	return status.Errorf(codes.Internal, "%s %s", message, id)
}

type channelReader struct {
	incoming <-chan []byte
	buf      []byte
}

func (t *channelReader) Read(p []byte) (int, error) {
	if t.buf == nil {
		var ok bool
		t.buf, ok = <-t.incoming
		if !ok {
			return 0, io.EOF
		}
	}
	n := len(p)
	m := len(t.buf)
	if m <= n {
		copy(p[:m], t.buf)
		t.buf = nil
		return m, nil
	} else {
		copy(p, t.buf[:n])
		t.buf = t.buf[n:]
		return n, nil
	}
}

func (t *channelReader) Close() error {
	return nil
}

type ContentStreamServer interface {
	Send(*raftpb.Content) error
	grpc.ServerStream
}

type contentWriter struct {
	stream ContentStreamServer
}

func (t contentWriter) Write(p []byte) (n int, err error) {
	n = len(p)
	err = t.stream.Send(&raftpb.Content{
		Content: p,
	})
	return
}
