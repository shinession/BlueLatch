//go:build windows

// Command bluelatch-gui is the window of BlueLatch: it lists the paired Bluetooth
// audio devices, and a click on Connect retries the connection until it holds. It
// is built for the Windows GUI subsystem (no console window); the command line
// is bluelatch.exe.
package main

import (
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/shinession/bluelatch/internal/app"
	"github.com/shinession/bluelatch/internal/appdir"
	"github.com/shinession/bluelatch/internal/bt"
	"github.com/shinession/bluelatch/internal/latch"
	"github.com/shinession/bluelatch/internal/logging"
	"github.com/shinession/bluelatch/internal/ui"
)

// version is set at build time (-ldflags "-X main.version=...").
var version = "dev"

const usage = `bluelatch-gui [-lang auto|en|zh] [-v] [-debug] [-demo [-demo-connect NAME]]

  -lang    language of the window: auto follows Windows (default), en, or zh
  -v       log every read of the device state to the log file
  -debug   switch on the developer tools and the context menu of the window
  -demo    use made-up devices, and touch no real device
  -demo-connect NAME
           with -demo, start connecting that device a moment after the window is up`

func main() { os.Exit(run(os.Args[1:])) }

func run(args []string) int {
	fs := flag.NewFlagSet("bluelatch-gui", flag.ContinueOnError)
	fs.SetOutput(io.Discard) // there is no console; mistakes are shown in a message box
	lang := fs.String("lang", "auto", "")
	verbose := fs.Bool("v", false, "")
	debug := fs.Bool("debug", false, "")
	demo := fs.Bool("demo", false, "")
	demoConnect := fs.String("demo-connect", "", "")
	if err := fs.Parse(args); err != nil {
		ui.ShowError("BlueLatch", err.Error()+"\n\n"+usage)
		return 2
	}
	if *lang != "auto" && *lang != "en" && *lang != "zh" {
		ui.ShowError("BlueLatch", fmt.Sprintf("-lang %q is not auto, en or zh.\n\n%s", *lang, usage))
		return 2
	}
	if *demoConnect != "" && !*demo {
		ui.ShowError("BlueLatch", "-demo-connect needs -demo.\n\n"+usage)
		return 2
	}

	// The library of the window writes its problems with the log package.
	log, closeLog, _ := logging.Open(*verbose, nil)
	defer closeLog()
	slog.SetDefault(log)

	title, mutex := "BlueLatch", `Local\BlueLatch-Window`
	if *demo {
		title, mutex = "BlueLatch (demo)", `Local\BlueLatch-Window-Demo`
	}
	release, ok := ui.SingleInstance(mutex, title)
	if !ok {
		return 0 // the window that is open has been brought to the front
	}
	defer release()

	var adapter bt.Adapter
	if *demo {
		adapter = app.NewDemo()
	} else {
		windowsAdapter, err := bt.New(log)
		if err != nil {
			log.Error("the Bluetooth API is not available", "err", err)
			ui.ShowError("BlueLatch", err.Error())
			return 1
		}
		adapter = windowsAdapter
		switchOnForgottenServices(windowsAdapter, log)
	}

	application := app.New(app.Config{
		Adapter: adapter,
		Policy:  latch.DefaultPolicy(),
		Log:     log,
		Version: version,
	})
	application.Start()
	if *demoConnect != "" {
		go startDemoConnect(application, adapter, *demoConnect, log)
	}

	dataDir := ""
	if dir, err := appdir.Dir(); err == nil {
		dataDir = filepath.Join(dir, "WebView2")
	}
	log.Info("starting the window", "version", version, "demo", *demo)
	err := ui.Run(application, ui.Options{Title: title, Lang: *lang, Debug: *debug, DataDir: dataDir, Log: log})
	application.Shutdown(30 * time.Second)
	if err != nil {
		log.Error("the window failed", "err", err)
		ui.ShowError("BlueLatch", err.Error()+"\n\nhttps://developer.microsoft.com/microsoft-edge/webview2/")
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

// startDemoConnect starts connecting a demo device a moment after the window is
// up, so the progress can be seen without a click.
func startDemoConnect(a *app.App, adapter bt.Adapter, name string, log *slog.Logger) {
	time.Sleep(1500 * time.Millisecond)
	devices, err := adapter.Devices()
	if err == nil {
		var dev bt.Device
		if dev, err = bt.Find(devices, name); err == nil {
			err = a.Connect(dev.Address.String())
		}
	}
	if err != nil {
		log.Error("-demo-connect failed", "err", err)
	}
}
