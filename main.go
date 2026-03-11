package main

import (
	"embed"
	"flag"
	"fmt"
	"os"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	configPath := flag.String("config", "ktuluekit.json", "path to ktuluekit.json")
	flag.Parse()

	if !isAdmin() {
		fmt.Fprintln(os.Stderr, "KtulueKit must be run as Administrator.")
		fmt.Fprintln(os.Stderr, "Right-click the .exe and choose 'Run as administrator'.")
		os.Exit(1)
	}

	app := NewApp(*configPath)

	err := wails.Run(&options.App{
		Title:  "KtulueKit",
		Width:  1100,
		Height: 780,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		OnStartup: app.startup,
		Bind: []interface{}{
			app,
		},
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}
}
