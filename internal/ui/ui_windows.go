//go:build windows

// Package ui is the window of BlueLatch: a WebView2 window that shows the state
// of internal/app and passes the clicks on to it. The page is one HTML file that
// is embedded in the executable (web/index.html). It has no logic of its own
// beyond drawing: it reads the state with blState every half second, and calls
// blConnect and blCancel.
package ui

import (
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"

	webview2 "github.com/jchv/go-webview2"

	"github.com/shinession/bluelatch/internal/app"
)

//go:embed web/index.html
var indexHTML string

// Sizes of the window at 96 DPI, in pixels of the page.
const (
	clientWidth, clientHeight = 480, 660
	minWidth, minHeight       = 380, 440
)

// ErrNoWebView2 is returned when the window cannot be created. The usual cause
// is that the Microsoft Edge WebView2 Runtime is not installed.
var ErrNoWebView2 = errors.New("the window could not be created; the Microsoft Edge WebView2 Runtime may be missing")

// Options tunes the window.
type Options struct {
	Title   string       // the title of the window, which is also how a second start finds it
	Lang    string       // auto, en or zh
	Debug   bool         // switches on the developer tools and the context menu
	DataDir string       // where WebView2 keeps its data
	Log     *slog.Logger // may be nil
}

// Run shows the window and returns when it has been closed. It must run on the
// main thread, which the webview2 package locks to the main goroutine.
//
// Closing the window first stops the application, which waits for a request that
// Windows is working on (at most shutdownTimeout), because exiting in the middle
// of it could leave a service switched off.
func Run(a *app.App, opt Options) error {
	if opt.Log == nil {
		opt.Log = slog.New(slog.DiscardHandler)
	}
	enableDPIAwareness()
	scale := systemDPIScale()

	w := webview2.NewWithOptions(webview2.WebViewOptions{
		Debug:     opt.Debug,
		AutoFocus: true,
		DataPath:  opt.DataDir,
		WindowOptions: webview2.WindowOptions{
			Title:  opt.Title,
			Width:  uint(float64(clientWidth+16) * scale),
			Height: uint(float64(clientHeight+39) * scale),
			Center: true,
		},
	})
	if w == nil {
		return ErrNoWebView2
	}
	defer w.Destroy()
	w.SetSize(int(float64(clientWidth)*scale), int(float64(clientHeight)*scale), webview2.HintNone)
	w.SetSize(int(float64(minWidth)*scale), int(float64(minHeight)*scale), webview2.HintMin)

	hwnd := uintptr(w.Window())
	applyDarkTitleBar(hwnd)
	installCloseGuard(hwnd, a, w, opt.Log)

	// The bound functions run on the thread of the window, so they must return at
	// once: Connect only starts a job, and the state is a copy.
	if err := w.Bind("blState", func() app.State { return a.Snapshot() }); err != nil {
		return err
	}
	if err := w.Bind("blConnect", func(address string) error { return a.Connect(address) }); err != nil {
		return err
	}
	if err := w.Bind("blCancel", func() { a.Cancel() }); err != nil {
		return err
	}
	lang, err := json.Marshal(opt.Lang)
	if err != nil {
		return fmt.Errorf("language: %w", err)
	}
	w.Init("window.blLang = " + string(lang) + ";")
	w.SetHtml(indexHTML)

	w.Run()
	return nil
}
