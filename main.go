package main

import (
	"embed"
	"log"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/linux"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	app := NewApp()

	err := wails.Run(&options.App{
		Title:     "Cortex — AI Memory Shell",
		Width:     1280,
		Height:    800,
		MinWidth:  900,
		MinHeight: 600,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		BackgroundColour:   &options.RGBA{R: 10, G: 10, B: 15, A: 255},
		OnStartup:          app.startup,
		Frameless:          false,
		StartHidden:        false,
		HideWindowOnClose:  false,
		DisableResize:      false,
		Linux: &linux.Options{
			ProgramName: "Cortex",
		},
		Bind: []interface{}{
			app,
		},
	})

	if err != nil {
		log.Fatalf("Error starting Cortex: %v", err)
	}
}
