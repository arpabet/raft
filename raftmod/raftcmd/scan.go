/*
 * Copyright (c) 2025-2026 Karagatan LLC.
 * SPDX-License-Identifier: BUSL-1.1
 */

package raftcmd

/**
RaftCommands are the cligo beans of the 'serf' command group. Register them
with the application, e.g. cligo.New(cligo.Beans(raftcmd.RaftCommands...)).
*/
var RaftCommands = []interface{}{
	SerfGroup(),
	SerfClientProvider(),
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
}
