package main

import (
	"embed"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/mac"

	"github.com/StanislavStefanovPetrov/mmi-tunes/internal/tools"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	// macOS .app bundles launched from Finder inherit a minimal PATH
	// that excludes /opt/homebrew/bin etc. Augment so child processes
	// (yt-dlp, ffmpeg) are findable regardless of how the app started.
	tools.AugmentPATH()

	app := NewApp()

	err := wails.Run(&options.App{
		Title:     "MMI Tunes",
		Width:     900,
		Height:    640,
		MinWidth:  640,
		MinHeight: 480,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		BackgroundColour: &options.RGBA{R: 18, G: 18, B: 18, A: 1},
		OnStartup:        app.startup,
		OnShutdown:       app.shutdown,
		Bind: []interface{}{
			app,
		},
		Mac: &mac.Options{
			TitleBar:             mac.TitleBarHiddenInset(),
			WebviewIsTransparent: false,
			WindowIsTranslucent:  false,
			About: &mac.AboutInfo{
				Title:   "MMI Tunes",
				Message: "macOS app for downloading YouTube audio as Audi MMI compatible MP3.\n\n© Stanislav Petrov",
			},
		},
	})

	if err != nil {
		println("Error:", err.Error())
	}
}
