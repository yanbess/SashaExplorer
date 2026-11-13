package main

import (
	"embed"
	"os"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/windows"
)

//go:embed all:frontend/dist
var assets embed.FS

// version is set by the release build from its tag, as in
// -ldflags "-X main.version=0.2.0".
var version = "dev"

func main() {
	app, err := NewApp(os.Args[1:])
	if err != nil {
		println("Error:", err.Error())
		os.Exit(2)
	}

	layout := currentLayout()
	app.layout = layout
	startState := options.Normal
	if app.startMaximized() {
		startState = options.Maximised
	}
	// The window is painted before the page loads, so it starts in the color of
	// the theme.
	background := &options.RGBA{R: 15, G: 18, B: 20, A: 1}
	if light := lightTheme(app.started.Theme); light {
		background = &options.RGBA{R: 249, G: 250, B: 251, A: 1}
	}
	err = wails.Run(&options.App{
		Title:     "SashaExplorer",
		Width:     layout.width,
		Height:    layout.height,
		MinWidth:  layout.minWidth,
		MinHeight: layout.minHeight,
		AssetServer: &assetserver.Options{
			Assets:     assets,
			Middleware: app.mediaMiddleware,
		},
		WindowStartState: startState,
		BackgroundColour: background,
		OnStartup:        app.startup,
		OnDomReady:       app.domReady,
		OnBeforeClose:    app.beforeClose,
		OnShutdown:       app.shutdown,
		Bind:             []interface{}{app},
		// One window: starting the program again opens its folders here.
		SingleInstanceLock: &options.SingleInstanceLock{
			UniqueId:               "c2a7e0d4-5b1f-4f7e-9d0a-sashaexplorer",
			OnSecondInstanceLaunch: app.secondInstance,
		},
		// Files dropped from other programs arrive as paths. The page itself must
		// accept drops from outside for that, so DisableWebViewDrop stays off;
		// the runtime still keeps a dropped file from replacing the page.
		DragAndDrop: &options.DragAndDrop{
			EnableFileDrop: true,
		},
		Windows: &windows.Options{
			Theme:      windows.SystemDefault,
			ZoomFactor: layout.zoom,
		},
	})
	if err != nil {
		println("Error:", err.Error())
	}
}
