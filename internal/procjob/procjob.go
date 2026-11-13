// SPDX-License-Identifier: AGPL-3.0-or-later

// Package procjob binds the lifetime of Memo's desktop UI, owned backend, and
// local engines (llama-server, the embedding server, whisper-server, the
// Subscriptions sidecar) to the Memo session owner while ensuring detached
// external launches (memo-updater, open_app, browseropen) survive exit.
//
// On Windows a child process does not die with its parent unless contained in a
// Job Object created with JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE. EnsureSelf puts
// the desktop-session owner (or standalone backend) into a Job Object configured
// with JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE | JOB_OBJECT_LIMIT_BREAKAWAY_OK so
// every managed child inherits containment automatically at CreateProcess time
// while DetachedAttr allows external apps and the updater to break away.
// Bind additionally binds individually spawned engine processes.
//
// On Unix systems ManagedAttr / UIAttr isolate owned children in dedicated
// process groups killed with KillProcessTree on hung teardown, while
// WatchOwnerPipe delivers instant EOF notification whenever the desktop owner
// exits or is killed (even via SIGKILL).
package procjob
