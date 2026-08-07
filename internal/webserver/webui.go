package webserver

import "embed"

// webUIFS embeds the minimal browser client (index.html/style.css/app.js)
// used for CasaOS/headless deployments — a lightweight chat + critical-
// settings UI, deliberately not a port of frontend/ (which stays dart:io-
// based and desktop-only). Anyone with the full frontend/ app can already
// point it at a remote backend via Settings → Remote Access and manage
// everything; this exists for the case where that isn't installed yet.
//
//go:embed webui/index.html webui/style.css webui/app.js
var webUIFS embed.FS
