package main

import (
	"embed"
	"runtime"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/mac"
	"github.com/wailsapp/wails/v2/pkg/options/windows"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	app := NewApp()

	err := wails.Run(&options.App{
		Title:     "PaieMaroc — Paie, CNSS et virements",
		Width:     1440,
		Height:    920,
		MinWidth:  1120,
		MinHeight: 720,
		Frameless: runtime.GOOS != "darwin",
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		BackgroundColour: &options.RGBA{R: 2, G: 58, B: 116, A: 1},
		// Production builds keep the native webview context menu and inspector hidden.
		EnableDefaultContextMenu: false,
		Mac: &mac.Options{
			TitleBar: mac.TitleBarHiddenInset(),
		},
		Windows: &windows.Options{
			Theme:                             windows.Light,
			DisableFramelessWindowDecorations: false,
			BackdropType:                      windows.Mica,
		},
		OnStartup:  app.startup,
		OnShutdown: app.shutdown,
		Bind: []interface{}{
			app,
		},
	})

	if err != nil {
		println("Erreur:", err.Error())
	}
}
