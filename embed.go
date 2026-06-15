package main

import "embed"

//go:embed binaries/placeholder
var embeddedBinaries embed.FS

//go:embed version
var versionFile string
