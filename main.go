package main

import (
	"embed"
	"log"
	"time"

	"github.com/GennoBou/gnb-twview/backend"
	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
)

// Wails uses Go's `embed` package to embed the frontend files into the binary.
//
//go:embed all:frontend/dist
var assets embed.FS

func init() {
	// Register custom events for bindings generator
	application.RegisterEvent[map[string]interface{}]("login-status-changed")
	application.RegisterEvent[[]backend.StreamInfo]("streamers-updated")
	application.RegisterEvent[map[string]interface{}]("auto-state-changed")
	application.RegisterEvent[map[string]interface{}]("streamer-switched")
	application.RegisterEvent[map[string]interface{}]("device-auth-status")
}

func main() {
	// Initialize Twitch Service
	twitchService := backend.NewTwitchService()

	// Create Wails application
	app := application.New(application.Options{
		Name:        "gnb-twview",
		Description: "A minimalist Twitch viewer browser",
		Services: []application.Service{
			application.NewService(&backend.GreetService{}),
			application.NewService(twitchService),
		},
		Assets: application.AssetOptions{
			Handler: application.AssetFileServerFS(assets),
		},
		Mac: application.MacOptions{
			ApplicationShouldTerminateAfterLastWindowClosed: true,
		},
		OnShutdown: func() {
			// アプリ終了時に視聴時間を保存する
			twitchService.SaveWatchedTimes()
		},
	})

	// Inject App instance into Twitch Service
	twitchService.SetApp(app)

	// Create window
	mainWindow := app.Window.NewWithOptions(application.WebviewWindowOptions{
		Title: "GNB - Twitch Viewer",
		Mac: application.MacWindow{
			InvisibleTitleBarHeight: 50,
			Backdrop:                application.MacBackdropTranslucent,
			TitleBar:                application.MacTitleBarHiddenInset,
		},
		BackgroundColour: application.NewRGB(10, 14, 23), // Dark Slate
		URL:              "/",
		Width:            1200,
		Height:           720,
	})

	// Create Twitch window (initially loading twitch homepage)
	twitchWindow := app.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:     "Twitch - gnb-twview",
		URL:       "https://www.twitch.tv/",
		Frameless: true,
	})
	twitchWindow.Hide()

	// メインウィンドウが閉じられた場合、アプリケーションプロセスを確実に終了させる
	mainWindow.OnWindowEvent(events.Common.WindowClosing, func(e *application.WindowEvent) {
		app.Quit()
	})

	// Give sub windows to twitchService for sync
	twitchService.SetWindows(mainWindow, twitchWindow)

	// Register window event synchronization
	mainWindow.OnWindowEvent(events.Common.WindowDidMove, func(e *application.WindowEvent) {
		twitchService.SyncSubWindowPositions()
	})
	mainWindow.OnWindowEvent(events.Common.WindowDidResize, func(e *application.WindowEvent) {
		twitchService.SyncSubWindowPositions()
	})
	mainWindow.OnWindowEvent(events.Common.WindowMinimise, func(e *application.WindowEvent) {
		twitchWindow.Hide()
	})
	mainWindow.OnWindowEvent(events.Common.WindowRestore, func(e *application.WindowEvent) {
		twitchService.SyncSubWindowPositions()
	})

	// Background ticker goroutines
	go func() {
		// LoadSavedToken will be triggered by frontend onMount after settings are loaded.
		// Periodic loops
		secondTicker := time.NewTicker(1 * time.Second)
		apiRefreshTicker := time.NewTicker(5 * time.Minute)

		defer secondTicker.Stop()
		defer apiRefreshTicker.Stop()

		for {
			select {
			case <-secondTicker.C:
				// Tick second if auto mode is active
				state := twitchService.TickSecond()
				if state["auto_mode"].(bool) {
					app.Event.Emit("auto-state-changed", state)
				}
			case <-apiRefreshTicker.C:
				// Refresh followed streams list from Twitch API
				twitchService.RefreshLiveStreamers()
			}
		}
	}()

	// Run the application
	err := app.Run()
	if err != nil {
		log.Fatal(err)
	}
}
