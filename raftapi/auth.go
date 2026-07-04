/*
 * Copyright (c) 2025-2026 Karagatan LLC.
 * SPDX-License-Identifier: BUSL-1.1
 */

package raftapi

import (
	"context"
	"reflect"
)

var AuthorizationMiddlewareClass = reflect.TypeOf((*AuthorizationMiddleware)(nil)).Elem()

/**
AuthorizedUser is the authenticated caller of the raft control plane API.
*/
type AuthorizedUser struct {

	/**
	User id or unique username of the authorized user.
	*/
	Username string

	/**
	Roles assigned to the user. The control plane requires the "ADMIN" role.
	*/
	Roles map[string]bool

	/**
	Additional permissions and ACL list for the user.
	*/
	Context map[string]string
}

/**
AuthorizationMiddleware gates the raft control plane API (bootstrap, join,
apply, recover) to ADMIN callers.

The bean is optional: when absent, the gate is skipped and the transport
(e.g. mTLS) is expected to authenticate peers.
*/
type AuthorizationMiddleware interface {

	/**
	Gets the AuthorizedUser from the request context.
	*/
	GetUser(ctx context.Context) (*AuthorizedUser, bool)
}
