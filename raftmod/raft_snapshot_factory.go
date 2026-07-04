/*
 * Copyright (c) 2025-2026 Karagatan LLC.
 * SPDX-License-Identifier: BUSL-1.1
 */

package raftmod

import (
	"os"
	"path/filepath"
	"reflect"

	"github.com/hashicorp/raft"
	"go.arpabet.com/cligo"
	"go.arpabet.com/glue"
	"go.arpabet.com/servion"
	"golang.org/x/xerrors"
)

var SnapshotStoreClass = reflect.TypeOf((*raft.SnapshotStore)(nil)).Elem()

type implRaftSnapshotFactory struct {
	CliApp     cligo.CliApplication `inject:"optional"`
	Runtime    servion.Runtime      `inject:"optional"`
	Properties glue.Properties      `inject:""`

	AppName string `value:"application.name,default="`

	RetainSnapshotCount int    `value:"raft.snapshot-retain-count,default=5"`
	KeyProperty         string `value:"raft.snapshot-key-bean,default="`

	DataDir      string      `value:"application.data.dir,default="`
	DataDirPerm  os.FileMode `value:"application.perm.data.dir,default=-rwxrwx---"`
	DataFilePerm os.FileMode `value:"application.perm.data.file,default=-rw-rw-r--"`
}

func RaftSnapshotFactory() glue.FactoryBean {
	return &implRaftSnapshotFactory{}
}

func (t *implRaftSnapshotFactory) Object() (object interface{}, err error) {

	defer panicToError(&err)

	dataDir := t.DataDir
	if dataDir == "" {
		dataDir = filepath.Join(homeDir(t.Runtime), "db")

		if err := createDirIfNeeded(dataDir, t.DataDirPerm); err != nil {
			return nil, err
		}

		dataDir = filepath.Join(dataDir, resolveApplicationName(t.AppName, t.CliApp))
	}

	if err := createDirIfNeeded(dataDir, t.DataDirPerm); err != nil {
		return nil, err
	}

	snapshotsFolder := filepath.Join(dataDir, "raft-snapshot")

	if err := createDirIfNeeded(snapshotsFolder, t.DataDirPerm); err != nil {
		return nil, err
	}

	// Create the snapshot delegate. This allows the Raft to truncate the log.
	snapshots, err := raft.NewFileSnapshotStore(snapshotsFolder, t.RetainSnapshotCount, os.Stderr)
	if err != nil {
		return nil, xerrors.Errorf("raft snapshots '%s' creation error, %v", snapshotsFolder, err)
	}

	if t.KeyProperty != "" {
		encryptionToken := t.Properties.GetString(t.KeyProperty, "")
		if encryptionToken == "" {
			// fall back to the environment, e.g. raft.snapshot-key -> RAFT_SNAPSHOT_KEY
			encryptionToken = os.Getenv(envKey(t.KeyProperty))
			if encryptionToken == "" {
				return nil, xerrors.Errorf("'%s' encryption token is required, set the property or the '%s' environment variable", t.KeyProperty, envKey(t.KeyProperty))
			}
		}
		return NewEncryptedSnapshotStore(snapshots, encryptionToken)
	}

	return snapshots, nil
}

func (t *implRaftSnapshotFactory) ObjectType() reflect.Type {
	return SnapshotStoreClass
}

func (t *implRaftSnapshotFactory) ObjectName() string {
	return "raft-snapshot"
}

func (t *implRaftSnapshotFactory) Singleton() bool {
	return true
}
