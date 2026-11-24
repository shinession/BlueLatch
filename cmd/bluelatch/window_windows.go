package main

import (
	"log/slog"
	"path/filepath"
	"time"

	"github.com/shinession/bluelatch/internal/app"
	"github.com/shinession/bluelatch/internal/appdir"
	"github.com/shinession/bluelatch/internal/bt"
	"github.com/shinession/bluelatch/internal/latch"
	"github.com/shinession/bluelatch/internal/logging"
	"github.com/shinession/bluelatch/internal/ui"
)

// useWindowsHost connects the parts of the program that are Windows itself: the
// console, the message box, and the window.
func (c *cli) useWindowsHost() {
	c.console = winConsole{}
	c.showError = ui.ShowError
	c.window = c.runWindow
}

// runWindow opens the window in this process and returns when it is closed. The
// process has no console by now (it was freed, or was never there), so problems
// are shown in a message box and written to the log file.
func (c *cli) runWindow(o windowOptions) int {
	// The library of the window writes its problems with the log package.
	log, closeLog, _ := logging.Open(o.verbose, nil)
	defer closeLog()
	slog.SetDefault(log)

	title, mutex := "BlueLatch", `Local\BlueLatch-Window`
	if o.demo {
		title, mutex = "BlueLatch (demo)", `Local\BlueLatch-Window-Demo`
	}
	release, ok := ui.SingleInstance(mutex, title)
	if !ok {
		return 0 // the window that is open has been brought to the front
	}
	defer release()

	var adapter bt.Adapter
	if o.demo {
		adapter = app.NewDemo()
	} else {
		var err error
		if adapter, err = c.newAdapter(log); err != nil {
			log.Error("the Bluetooth API is not available", "err", err)
			c.showError("BlueLatch", err.Error())
			return 1
		}
		switchOnForgottenServices(adapter, log)
	}

	application := app.New(app.Config{
		Adapter: adapter,
		Policy:  latch.DefaultPolicy(),
		Log:     log,
		Version: version,
	})
	application.Start()
	if o.demoConnect != "" {
		go startDemoAction(application, adapter, o.demoConnect, false, log)
	}
	if o.demoDisconnect != "" {
		go startDemoAction(application, adapter, o.demoDisconnect, true, log)
	}

	dataDir := ""
	if dir, err := appdir.Dir(); err == nil {
		dataDir = filepath.Join(dir, "WebView2")
	}
	log.Info("starting the window", "version", version, "demo", o.demo)
	err := ui.Run(application, ui.Options{Title: title, Lang: o.lang, Debug: o.debug, DataDir: dataDir, Log: log})
	application.Shutdown(30 * time.Second)
	if err != nil {
		log.Error("the window failed", "err", err)
		c.showError("BlueLatch", err.Error()+"\n\nhttps://developer.microsoft.com/microsoft-edge/webview2/")
		return 1
	}
	log.Info("the window was closed")
	return 0
}

// switchOnForgottenServices repairs what an earlier run left behind when it was
// stopped between switching a service off and on again, before anything is shown.
func switchOnForgottenServices(a bt.Adapter, log *slog.Logger) {
	recoverer, ok := a.(bt.Recoverer)
	if !ok {
		return
	}
	restored, err := recoverer.Recover()
	for _, r := range restored {
		log.Warn("switched on a service that an earlier run had left switched off", "device", r.Device, "service", r.Service.String())
	}
	if err != nil {
		log.Warn("switching on the services that an earlier run left off failed", "err", err)
	}
}

// startDemoAction starts connecting or disconnecting a demo device a moment
// after the window is up, so the progress can be seen without a click.
func startDemoAction(a *app.App, adapter bt.Adapter, name string, disconnect bool, log *slog.Logger) {
	time.Sleep(1500 * time.Millisecond)
	devices, err := adapter.Devices()
	if err == nil {
		var dev bt.Device
		if dev, err = bt.Find(devices, name); err == nil {
			if disconnect {
				err = a.Disconnect(dev.Address.String())
			} else {
				err = a.Connect(dev.Address.String())
			}
		}
	}
	if err != nil {
		log.Error("the demo action failed", "disconnect", disconnect, "err", err)
	}
}
