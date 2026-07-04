/*
 * Copyright (c) 2025-2026 Karagatan LLC.
 * SPDX-License-Identifier: BUSL-1.1
 */

package raftmod

import (
	"bytes"
	"crypto/sha256"
	"io"
	"testing"

	"github.com/stretchr/testify/require"
	"golang.org/x/xerrors"
)

// failingSource yields its data and then fails with a non-EOF error, like a
// network stream dropping mid-transfer.
type failingSource struct {
	data []byte
	err  error
}

func (t *failingSource) Read(p []byte) (int, error) {
	if len(t.data) == 0 {
		return 0, t.err
	}
	n := copy(p, t.data)
	t.data = t.data[n:]
	return n, nil
}

func (t *failingSource) Close() error { return nil }

// A transport error in the middle of an encrypted snapshot stream must surface
// to the reader, not be masked as a clean io.EOF (which would silently restore
// a truncated snapshot).
func TestStreamDecrypterPropagatesSourceError(t *testing.T) {

	key := sha256.Sum256([]byte("token"))

	// Build a valid encrypted stream prefix: IV + some ciphertext.
	var sinkBuf bytes.Buffer
	sink := &memorySink{buf: &sinkBuf}
	enc, err := StreamEncrypter(key[:], sink)
	require.NoError(t, err)
	payload := []byte("payload that will be cut off")
	_, err = enc.Write(append([]byte(nil), payload...))
	require.NoError(t, err)

	// Truncate the stream and end it with a transport error.
	transportErr := xerrors.New("connection reset")
	source := &failingSource{data: sinkBuf.Bytes()[:sinkBuf.Len()-5], err: transportErr}

	dec, err := StreamDecrypter(key[:], source)
	require.NoError(t, err)

	_, err = io.ReadAll(dec)
	require.ErrorIs(t, err, transportErr)
}

// memorySink is a raft.SnapshotSink writing to a buffer.
type memorySink struct {
	buf *bytes.Buffer
}

func (t *memorySink) Write(p []byte) (int, error) { return t.buf.Write(p) }
func (t *memorySink) Close() error                { return nil }
func (t *memorySink) ID() string                  { return "mem" }
func (t *memorySink) Cancel() error               { return nil }
