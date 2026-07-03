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
	"go.arpabet.com/glue"
	"go.arpabet.com/sprint"
	"golang.org/x/xerrors"
)

var SnapshotStoreClass = reflect.TypeOf((*raft.SnapshotStore)(nil)).Elem()

type implRaftSnapshotFactory struct {
	Application                       sprint.Application                       `inject:""`
	Properties                        glue.Properties                          `inject:""`
	SystemEnvironmentPropertyResolver sprint.SystemEnvironmentPropertyResolver `inject:""`

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
		dataDir = filepath.Join(t.Application.ApplicationDir(), "db")

		if err := createDirIfNeeded(dataDir, t.DataDirPerm); err != nil {
			return nil, err
		}

		dataDir = filepath.Join(dataDir, t.Application.Name())
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
			var ok bool
			encryptionToken, ok = t.SystemEnvironmentPropertyResolver.PromptProperty(t.KeyProperty)
			if !ok || encryptionToken == "" {
				return nil, xerrors.Errorf("'%s' encryption token is required", t.KeyProperty)
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
