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
	app.SetEmbeddedAssets(assets)

	err := wails.Run(&options.App{
		Title:            "Memo — AI Memory Shell",
		Width:            1440,
		Height:           900,
		MinWidth:         800,
		MinHeight:        500,
		WindowStartState: options.Maximised,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		BackgroundColour:    &options.RGBA{R: 5, G: 5, B: 5, A: 255},
		OnStartup:           app.startup,
		Frameless:           false,
		StartHidden:         false,
		HideWindowOnClose:   false,
		DisableResize:       false,
		EnableDefaultContextMenu: false,
		DragAndDrop: &options.DragAndDrop{
			EnableFileDrop:     true,
			DisableWebViewDrop: true,
		},
		Linux: &linux.Options{
			ProgramName: "Memo",
		},
		Bind: []interface{}{
			app,
		},
	})

	if err != nil {
		log.Fatalf("Error starting Memo: %v", err)
	}
}
