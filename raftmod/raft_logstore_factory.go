/*
 * Copyright (c) 2025-2026 Karagatan LLC.
 * SPDX-License-Identifier: BUSL-1.1
 */

package raftmod

import (
	"reflect"

	"github.com/dgraph-io/badger/v4"
	"github.com/hashicorp/raft"
	"go.arpabet.com/glue"
	raftbadger "go.arpabet.com/raft-badger"
	"go.arpabet.com/store"
	"golang.org/x/xerrors"
)

var LogStoreClass = reflect.TypeOf((*raft.LogStore)(nil)).Elem()

type implRaftLogStoreFactory struct {
	RaftStore     store.ManagedDataStore `inject:"bean=raft-store"`
	RaftLogPrefix string                 `value:"raft-store.log-prefix,default=log"`
}

func RaftLogStoreFactory() glue.FactoryBean {
	return &implRaftLogStoreFactory{}
}

func (t *implRaftLogStoreFactory) Object() (object interface{}, err error) {

	defer panicToError(&err)

	db, ok := t.RaftStore.Instance().(*badger.DB)
	if !ok {
		return nil, xerrors.New("managed data delegate 'raft-store' must have badger backend")
	}

	return raftbadger.NewLogStore(db, []byte(t.RaftLogPrefix)), nil

}

func (t *implRaftLogStoreFactory) ObjectType() reflect.Type {
	return LogStoreClass
}

func (t *implRaftLogStoreFactory) ObjectName() string {
	return "raft-store-log"
}

func (t *implRaftLogStoreFactory) Singleton() bool {
	return true
}
