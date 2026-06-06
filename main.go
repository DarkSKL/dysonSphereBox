package main

import (
	"embed"

	"DysonSphereBox/service"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
)

//go:embed all:frontend/dist
var assets embed.FS

//go:embed app.db
var databaseTemplate []byte

func main() {
	// Create an instance of the app structure
	app := NewApp()
	services := append([]interface{}{app}, service.RegisterServices()...)

	// Create application with options
	err := wails.Run(&options.App{
		Title:  "DysonSphereBox",
		Width:  1024,
		Height: 768,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		//BackgroundColour: &options.RGBA{R: 224, G: 218, B: 220, A: 1},
		OnStartup:  app.startup,
		OnShutdown: app.shutdown,
		Bind:       services,
	})

	if err != nil {
		println("Error:", err.Error())
	}
}
