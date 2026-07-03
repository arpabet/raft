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

var StableStoreClass = reflect.TypeOf((*raft.StableStore)(nil)).Elem()

type implRaftStableStoreFactory struct {
	RaftStore      store.ManagedDataStore `inject:"bean=raft-store"`
	RaftConfPrefix string                 `value:"raft-store.conf-prefix,default=conf"`
}

func RaftStableStoreFactory() glue.FactoryBean {
	return &implRaftStableStoreFactory{}
}

func (t *implRaftStableStoreFactory) Object() (object interface{}, err error) {

	defer panicToError(&err)

	db, ok := t.RaftStore.Instance().(*badger.DB)
	if !ok {
		return nil, xerrors.Errorf("managed data delegate 'raft-store' must have badger backend")
	}

	return raftbadger.NewStableStore(db, []byte(t.RaftConfPrefix)), nil

}

func (t *implRaftStableStoreFactory) ObjectType() reflect.Type {
	return StableStoreClass
}

func (t *implRaftStableStoreFactory) ObjectName() string {
	return "raft-store-stable"
}

func (t *implRaftStableStoreFactory) Singleton() bool {
	return true
}
