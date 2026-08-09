package webserver

import "embed"

// webAppFS embeds the browser client — the same Flutter app as the desktop
// build (frontend/), compiled for web (`flutter build web`) and copied into
// webapp/ at build time (release CI does this automatically; see
// internal/webserver/webapp/index.html's placeholder for the local-dev
// command). This replaced a hand-rolled vanilla-JS reimplementation that
// used to live here (Faz 1-4, yapacam.md) — it worked, but every feature it
// covered (providers, model management, accounts, auth) was a second,
// hand-maintained copy of logic frontend/ already had right, and it reliably
// fell behind or reimplemented things incorrectly (a live example: its
// providers panel let you "activate" an unconfigured, keyless provider with
// no warning — frontend/'s actual provider dialog never had that bug). One
// codebase, one set of bugs, compiled to every target (Linux/macOS/Windows/
// web) instead of maintaining a second implementation forever.
//
//go:embed all:webapp
var webAppFS embed.FS
