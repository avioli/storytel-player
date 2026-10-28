package main

// Wails entry point: replaces the Electron main process (src/main.ts and
// src/modules/*). The React app is embedded from server/frontend (built from client/) and reaches
// the Go backend same-origin through /api/* and /native/*.

import (
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
)

//go:embed all:frontend
var clientBuild embed.FS

//go:embed assets/icon.png assets/tray-icon.png assets/tray-icon-mac@2x.png
var assetFS embed.FS

const appVersion = "1.4.1"

type App struct {
	app      *application.App
	window   *application.WebviewWindow
	store    *Store
	i18n     *I18n
	tray     *application.SystemTray
	quitting bool

	mu              sync.Mutex
	isPlaying       bool
	bookTitle       string
	isAuthenticated bool
	alwaysOnTop     bool
}

// emit delivers an event to the renderer as a DOM CustomEvent named
// "native:<name>"; see client/src/platform/wails.ts.
func (a *App) emit(name string, detail any) {
	raw, _ := json.Marshal(detail)
	a.window.ExecJS(fmt.Sprintf(`window.dispatchEvent(new CustomEvent(%q, {detail: %s}))`, "native:"+name, raw))
}

func (a *App) show() {
	if a.window.IsMinimised() {
		a.window.Restore()
	}
	a.window.Show()
	a.window.Focus()
}

// nativeRoutes replaces the Electron preload/IPC bridge.
func (a *App) nativeRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /native/store/get", func(w http.ResponseWriter, r *http.Request) {
		var b struct{ Key string }
		readBody(r, &b)
		sendJSON(w, 200, map[string]any{"value": a.store.Get(b.Key)})
	})
	mux.HandleFunc("POST /native/store/set", func(w http.ResponseWriter, r *http.Request) {
		var b struct {
			Key   string
			Value any
		}
		readBody(r, &b)
		a.store.Set(b.Key, b.Value)
		sendJSON(w, 200, map[string]any{})
	})
	mux.HandleFunc("POST /native/store/remove", func(w http.ResponseWriter, r *http.Request) {
		var b struct{ Key string }
		readBody(r, &b)
		a.store.Remove(b.Key)
		sendJSON(w, 200, map[string]any{})
	})
	mux.HandleFunc("GET /native/locale", func(w http.ResponseWriter, r *http.Request) {
		sendJSON(w, 200, a.i18n.Language())
	})
	mux.HandleFunc("GET /native/locale/state", func(w http.ResponseWriter, r *http.Request) {
		sendJSON(w, 200, a.i18n.State())
	})
	mux.HandleFunc("POST /native/locale", func(w http.ResponseWriter, r *http.Request) {
		var b struct{ Locale string }
		readBody(r, &b)
		a.store.Set("appLanguage", b.Locale)
		a.i18n.Detect()
		a.updateTrayMenu()
		sendJSON(w, 200, a.i18n.State())
	})
	mux.HandleFunc("GET /native/window/always-on-top", func(w http.ResponseWriter, r *http.Request) {
		a.mu.Lock()
		defer a.mu.Unlock()
		sendJSON(w, 200, a.alwaysOnTop)
	})
	mux.HandleFunc("POST /native/window/always-on-top", func(w http.ResponseWriter, r *http.Request) {
		var b struct{ Value bool }
		readBody(r, &b)
		a.mu.Lock()
		a.alwaysOnTop = b.Value
		a.mu.Unlock()
		a.window.SetAlwaysOnTop(b.Value)
		a.store.Set("settings.alwaysOnTop", b.Value)
		sendJSON(w, 200, map[string]any{})
	})
	mux.HandleFunc("POST /native/logs/open-folder", func(w http.ResponseWriter, r *http.Request) {
		revealInFileManager(filepath.Join(userDataDir(), "app.log"))
		sendJSON(w, 200, map[string]any{})
	})
	mux.HandleFunc("POST /native/tray/playing", func(w http.ResponseWriter, r *http.Request) {
		var b struct {
			IsPlaying bool    `json:"isPlaying"`
			BookTitle *string `json:"bookTitle"`
		}
		readBody(r, &b)
		a.mu.Lock()
		a.isPlaying = b.IsPlaying
		a.bookTitle = ""
		if b.BookTitle != nil {
			a.bookTitle = *b.BookTitle
		}
		a.mu.Unlock()
		a.updateTrayMenu()
		sendJSON(w, 200, map[string]any{})
	})
	mux.HandleFunc("POST /native/tray/auth", func(w http.ResponseWriter, r *http.Request) {
		var b struct {
			IsAuthenticated bool `json:"isAuthenticated"`
		}
		readBody(r, &b)
		a.mu.Lock()
		a.isAuthenticated = b.IsAuthenticated
		a.mu.Unlock()
		a.updateTrayMenu()
		sendJSON(w, 200, map[string]any{})
	})
}

func revealInFileManager(path string) {
	switch runtime.GOOS {
	case "darwin":
		_ = startCommand("open", "-R", path)
	case "windows":
		_ = startCommand("explorer", "/select,", path)
	default:
		_ = startCommand("xdg-open", filepath.Dir(path))
	}
}

func truncateTitle(title string, maxLen int) string {
	r := []rune(title)
	if len(r) <= maxLen {
		return title
	}
	return string(r[:maxLen-3]) + "..."
}

func (a *App) updateTrayMenu() {
	if a.tray == nil {
		return
	}
	a.mu.Lock()
	isPlaying, title, authed := a.isPlaying, a.bookTitle, a.isAuthenticated
	a.mu.Unlock()
	t := a.i18n.T

	menu := a.app.NewMenu()
	display := t("tray.noBookPlaying")
	if title != "" {
		display = truncateTitle(title, 50)
	}
	menu.Add(display).SetEnabled(false)
	menu.AddSeparator()
	menu.Add(t("tray.showApp")).OnClick(func(*application.Context) { a.show() })

	if isPlaying || title != "" {
		menu.AddSeparator()
		label := t("tray.play")
		if isPlaying {
			label = t("tray.pause")
		}
		menu.Add(label).OnClick(func(*application.Context) { a.emit("tray-play-pause", nil) })
		speeds := menu.AddSubmenu(t("tray.playbackSpeed"))
		for _, s := range []float64{0.5, 1.0, 1.25, 1.75, 2.0} {
			speed := s
			speeds.Add(fmt.Sprintf("%gx", speed)).OnClick(func(*application.Context) { a.emit("tray-set-speed", speed) })
		}
	}

	menu.AddSeparator()
	menu.Add(t("tray.about")).OnClick(func(*application.Context) {
		a.app.Dialog.Info().
			SetTitle(t("tray.about")).
			SetMessage(fmt.Sprintf("storytel-player@%s\n\nwails: v3\ngo: %s", appVersion, runtime.Version())).
			Show()
	})
	help := menu.AddSubmenu(t("tray.help"))
	help.Add(t("settings.githubRepo")).OnClick(func(*application.Context) {
		_ = a.app.Browser.OpenURL("https://github.com/debba/storytel-player")
	})
	if authed {
		menu.Add(t("tray.logout")).OnClick(func(*application.Context) {
			a.show()
			a.emit("tray-logout", nil)
		})
	}
	menu.Add(t("tray.quit")).OnClick(func(*application.Context) { a.app.Quit() })

	a.tray.SetMenu(menu)
}

func (a *App) saveBoundsLater() func() {
	var timer *time.Timer
	return func() {
		if timer != nil {
			timer.Stop()
		}
		timer = time.AfterFunc(500*time.Millisecond, func() {
			if a.window.IsMinimised() || a.window.IsMaximised() {
				return
			}
			x, y := a.window.Position()
			wd, ht := a.window.Size()
			a.store.Set("window.bounds", map[string]any{"x": x, "y": y, "width": wd, "height": ht})
		})
	}
}

func main() {
	store := NewStore()
	downloads, _ := os.UserHomeDir()
	downloads = filepath.Join(downloads, "Downloads")
	api := NewAPI(downloads)

	a := &App{store: store, i18n: NewI18n(store)}
	a.alwaysOnTop, _ = store.Get("settings.alwaysOnTop").(bool)

	mux := api.Routes()
	a.nativeRoutes(mux)

	assets, err := fs.Sub(clientBuild, "frontend")
	if err != nil {
		log.Fatal(err)
	}
	icon, _ := assetFS.ReadFile("assets/icon.png")

	a.app = application.New(application.Options{
		Name:        "Storytel Player",
		Description: "Storytel audiobook player for desktop",
		Icon:        icon,
		Assets: application.AssetOptions{
			Handler: application.AssetFileServerFS(assets),
			Middleware: func(next http.Handler) http.Handler {
				return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if strings.HasPrefix(r.URL.Path, "/api/") || strings.HasPrefix(r.URL.Path, "/native/") {
						mux.ServeHTTP(w, r)
						return
					}
					next.ServeHTTP(w, r)
				})
			},
		},
		Mac: application.MacOptions{
			ApplicationShouldTerminateAfterLastWindowClosed: false,
		},
		SingleInstance: &application.SingleInstanceOptions{
			UniqueID: "com.storytel.player.wails",
			OnSecondInstanceLaunch: func(application.SecondInstanceData) {
				if a.window != nil {
					a.show()
				}
			},
		},
		ShouldQuit: func() bool {
			// Let the renderer pause and sync the position first, as the
			// Electron build did on quit.
			a.quitting = true
			a.mu.Lock()
			playing := a.isPlaying
			a.mu.Unlock()
			if playing {
				a.emit("tray-play-pause", nil)
				time.Sleep(700 * time.Millisecond)
			}
			return true
		},
	})

	width, height := 1280, 800
	var x, y int
	hasPos := false
	if b, ok := store.Get("window.bounds").(map[string]any); ok {
		if w := int(num(b["width"])); w >= 880 {
			width = w
		}
		if h := int(num(b["height"])); h >= 600 {
			height = h
		}
		if _, ok := b["x"]; ok {
			x, y, hasPos = int(num(b["x"])), int(num(b["y"])), true
		}
	}

	opts := application.WebviewWindowOptions{
		Name:             "main",
		Title:            "Storytel Player",
		Width:            width,
		Height:           height,
		MinWidth:         880,
		MinHeight:        600,
		AlwaysOnTop:      a.alwaysOnTop,
		BackgroundColour: application.NewRGB(0x0A, 0x0A, 0x0A),
		URL:              "/",
		DevToolsEnabled:  os.Getenv("IS_DEBUG") == "true",
	}
	if hasPos {
		opts.InitialPosition = application.WindowXY
		opts.X, opts.Y = x, y
	}
	a.window = a.app.Window.NewWithOptions(opts)

	// Closing the window hides it to the tray; Quit really exits.
	a.window.RegisterHook(events.Common.WindowClosing, func(e *application.WindowEvent) {
		if !a.quitting {
			e.Cancel()
			a.window.Hide()
		}
	})
	save := a.saveBoundsLater()
	a.window.OnWindowEvent(events.Common.WindowDidMove, func(*application.WindowEvent) { save() })
	a.window.OnWindowEvent(events.Common.WindowDidResize, func(*application.WindowEvent) { save() })
	a.app.Event.OnApplicationEvent(events.Mac.ApplicationShouldHandleReopen, func(*application.ApplicationEvent) { a.show() })

	a.tray = a.app.SystemTray.New()
	if runtime.GOOS == "darwin" {
		trayIcon, _ := assetFS.ReadFile("assets/tray-icon-mac@2x.png")
		a.tray.SetTemplateIcon(trayIcon)
	} else {
		trayIcon, _ := assetFS.ReadFile("assets/tray-icon.png")
		a.tray.SetIcon(trayIcon)
	}
	a.tray.SetTooltip("Storytel Player")
	a.tray.OnDoubleClick(func() {
		if a.window.IsVisible() {
			a.window.Hide()
		} else {
			a.show()
		}
	})
	a.updateTrayMenu()

	if err := a.app.Run(); err != nil {
		log.Fatal(err)
	}
}
