// SPDX-License-Identifier: AGPL-3.0-or-later

// Package procjob makes the engines Memo starts (llama-server, the embedding
// server, whisper-server, the Subscriptions sidecar) end when the backend
// ends — however it ends.
//
// On Windows a child process does not die with its parent: a backend that was
// force-killed (Task Manager, Stop-Process -Force, a crash) left every engine
// running with nothing to stop it, holding RAM, VRAM and ports. Bind puts an
// engine into one Job Object of the backend created with
// JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE; the job handle is never closed by Memo,
// so the OS closes it when the backend process exits, and that kills every
// process still in the job.
//
// Only engines are bound, never the backend itself: anything the backend
// starts that must outlive it (memo-updater swapping in a new release, an
// application opened with open_app) would otherwise die with it.
//
// On other systems Bind does nothing (Linux/macOS stop engines through their
// own process-group handling).
package procjob
