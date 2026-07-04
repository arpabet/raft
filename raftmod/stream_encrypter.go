/*
 * Copyright (c) 2025-2026 Karagatan LLC.
 * SPDX-License-Identifier: BUSL-1.1
 */

package raftmod

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"io"

	"github.com/hashicorp/raft"
	"golang.org/x/xerrors"
)

/**
STREAM ENCRYPTER

Warning: fast but modifies stream data
*/

type implStreamEncrypter struct {
	sink   raft.SnapshotSink
	stream cipher.Stream
}

func StreamEncrypter(sessionKey []byte, sink raft.SnapshotSink) (raft.SnapshotSink, error) {
	block, err := aes.NewCipher(sessionKey)
	if err != nil {
		return nil, err
	}
	iv := make([]byte, block.BlockSize())
	_, err = rand.Read(iv)
	if err != nil {
		return nil, err
	}
	stream := cipher.NewCTR(block, iv)
	n, err := sink.Write(iv)
	if err != nil {
		return nil, err
	}
	if len(iv) != n {
		return nil, xerrors.Errorf("i/o write error, written %d bytes whereas expected %d bytes", n, len(iv))
	}
	// clean IV
	for i := 0; i < n; i++ {
		iv[i] = 0
	}
	return &implStreamEncrypter{
		sink:   sink,
		stream: stream,
	}, nil
}

func (t *implStreamEncrypter) Write(p []byte) (int, error) {
	t.stream.XORKeyStream(p, p)
	return t.sink.Write(p)
}

func (t *implStreamEncrypter) Close() error {
	return t.sink.Close()
}

func (t *implStreamEncrypter) ID() string {
	return t.sink.ID()
}

func (t *implStreamEncrypter) Cancel() error {
	return t.sink.Cancel()
}

/**
STREAM DECRYPTER

Warning: fast but modifies stream data
*/

type implStreamDecrypter struct {
	source io.ReadCloser
	stream cipher.Stream
}

func StreamDecrypter(sessionKey []byte, source io.ReadCloser) (io.ReadCloser, error) {
	block, err := aes.NewCipher(sessionKey)
	if err != nil {
		return nil, err
	}
	iv := make([]byte, block.BlockSize())
	n, err := io.ReadFull(source, iv)
	if err != nil {
		return nil, err
	}
	if n < len(iv) {
		return nil, io.EOF
	}
	stream := cipher.NewCTR(block, iv)
	// clean IV
	for i := 0; i < n; i++ {
		iv[i] = 0
	}
	return &implStreamDecrypter{
		source: source,
		stream: stream,
	}, nil
}

func (t *implStreamDecrypter) Read(p []byte) (int, error) {
	n, err := t.source.Read(p)
	if n > 0 {
		t.stream.XORKeyStream(p[:n], p[:n])
	}
	// Propagate the source's n/err verbatim: masking a transport error as io.EOF
	// would silently truncate the decrypted stream.
	return n, err
}

func (t *implStreamDecrypter) Close() error {
	return t.source.Close()
}
