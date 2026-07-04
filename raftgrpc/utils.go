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

	// When no Auth bean is configured the gate is skipped — the transport
	// (e.g. mTLS) is expected to authenticate peers.
	username := "anonymous"
	if t.Auth != nil {
		user, ok := t.Auth.GetUser(ctx)
		if !ok || !user.Roles["ADMIN"] {
			return status.Errorf(codes.Unauthenticated, "role ADMIN is required")
		}
		username = user.Username
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
			err = t.wrapError(err, methodName, username)
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
	if strings.Contains(issue, "concurrent transaction") {
		message = "concurrent transaction"
	} else if strings.Contains(issue, "not found") {
		message = "object not found"
	} else if strings.Contains(issue, "exist") {
		message = "object already exist"
	}
	id := t.NodeService.Issue().String()
	t.Log.Error(method, zap.String("errorId", id), zap.Any("username", username), zap.Error(err))
	return status.Errorf(codes.Internal, "%s %s", message, id)
}

// channelReader adapts a channel of chunks to an io.Reader. The producer may set
// err before closing the channel to end the stream with that error instead of a
// clean io.EOF (close(chan) is the happens-before edge making err visible).
type channelReader struct {
	incoming <-chan []byte
	buf      []byte
	err      error
}

func (t *channelReader) Read(p []byte) (int, error) {
	for len(t.buf) == 0 {
		var ok bool
		t.buf, ok = <-t.incoming
		if !ok {
			if t.err != nil {
				return 0, t.err
			}
			return 0, io.EOF
		}
	}
	n := copy(p, t.buf)
	t.buf = t.buf[n:]
	return n, nil
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
