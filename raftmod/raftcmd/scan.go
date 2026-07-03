/*
 * Copyright (c) 2025-2026 Karagatan LLC.
 * SPDX-License-Identifier: BUSL-1.1
 */

package raftcmd

var RaftCommands = []interface{}{
	SerfJoinCommand(),
	SerfMembersCommand(),
	SerfEventCommand(),
	SerfInfoCommand(),
	SerfVersionCommand(),
	SerfLeaveCommand(),
	SerfMonitorCommand(),
	SerfReachabilityCommand(),
	SerfRttCommand(),
	SerfTagsCommand(),
	SerfCommands(),
}
