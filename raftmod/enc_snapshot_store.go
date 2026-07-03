/*
 * Copyright (c) 2025-2026 Karagatan LLC.
 * SPDX-License-Identifier: BUSL-1.1
 */

package raftmod

import (
	"crypto/sha256"
	"github.com/hashicorp/raft"
	"io"
)

type implEncryptedSnapshotStore struct {
	delegate  raft.SnapshotStore
	token     string
}

func NewEncryptedSnapshotStore(store raft.SnapshotStore, token string) (raft.SnapshotStore, error) {
	return &implEncryptedSnapshotStore{delegate: store, token: token}, nil
}

func (t *implEncryptedSnapshotStore) Create(version raft.SnapshotVersion, index, term uint64, configuration raft.Configuration,
	configurationIndex uint64, trans raft.Transport) (sink raft.SnapshotSink, err error) {
	sink, err = t.delegate.Create(version, index, term, configuration, configurationIndex, trans)
	if err != nil {
		return
	}
	sessionKey := t.newSessionKey(index, term)
	sink, err = StreamEncrypter(sessionKey, sink)
	clean(sessionKey)
	return
}

func (t *implEncryptedSnapshotStore) List() ([]*raft.SnapshotMeta, error) {
	return t.delegate.List()
}

func (t *implEncryptedSnapshotStore) Open(id string) (meta *raft.SnapshotMeta, source io.ReadCloser, err error) {
	meta, source, err = t.delegate.Open(id)
	if err != nil {
		return
	}
	sessionKey := t.newSessionKey(meta.Index, meta.Term)
	source, err = StreamDecrypter(sessionKey, source)
	clean(sessionKey)
	return
}

func (t *implEncryptedSnapshotStore) newSessionKey(index, term uint64) []byte {
	h := sha256.New()
	h.Write([]byte(t.token))
	return h.Sum(nil)
}

func clean(arr []byte) {
	n := len(arr)
	for i := 0; i < n; i++ {
		arr[i] = 0
	}
}


