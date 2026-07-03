/*
 * Copyright (c) 2025-2026 Karagatan LLC.
 * SPDX-License-Identifier: BUSL-1.1
 */

package raftmod

import (
	"bytes"
	"fmt"
	"github.com/hashicorp/raft"
	"github.com/stretchr/testify/require"
	"io"
	"os"
	"testing"
)

func TestEncryptedSnapshotStore(t *testing.T) {

	dir, err := os.MkdirTemp(os.TempDir(), "raftmodtest")
	require.NoError(t, err)
	defer os.RemoveAll(dir)

	snapshots, err := raft.NewFileSnapshotStore(dir, 5, os.Stderr)
	require.NoError(t, err)

	testing, err := NewEncryptedSnapshotStore(snapshots, "123")
	require.NoError(t, err)

	sink, err := testing.Create(raft.SnapshotVersionMax, 100, 1, raft.Configuration{}, 0,nil)
	require.NoError(t, err)

	welcome := "Hello World!"
	buf := []byte(welcome)
	n, err := sink.Write(buf)
	require.NoError(t, err)
	require.Equal(t, len(welcome), n)

	err = sink.Close()
	require.NoError(t, err)

	// MODIFIES BUF during encryption
	require.False(t, bytes.Equal([]byte(welcome), buf))

	list, err := testing.List()
	require.NoError(t, err)
	require.Equal(t, len(list), 1)

	fmt.Printf("Snapshot ID: %s\n", list[0].ID)

	meta, reader, err := testing.Open(list[0].ID)
	require.NoError(t, err)

	fmt.Printf("Meta = %v\n", meta)

	content, err := io.ReadAll(reader)
	require.NoError(t, err)

	require.True(t, bytes.Equal([]byte(welcome), content))

	err = reader.Close()
	require.NoError(t, err)


}

