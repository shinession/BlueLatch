//go:build windows

package ui

import (
	"errors"
	"log/slog"
	"sync/atomic"
	"time"
	"unsafe"

	webview2 "github.com/jchv/go-webview2"
	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"

	"github.com/shinession/bluelatch/internal/app"
)

var (
	user32 = windows.NewLazySystemDLL("user32.dll")
	dwmapi = windows.NewLazySystemDLL("dwmapi.dll")

	procSetProcessDpiAwarenessContext = user32.NewProc("SetProcessDpiAwarenessContext")
	procGetDpiForSystem               = user32.NewProc("GetDpiForSystem")
	procGetDpiForWindow               = user32.NewProc("GetDpiForWindow")
	procGetSystemMetrics              = user32.NewProc("GetSystemMetrics")
	procGetSystemMetricsForDpi        = user32.NewProc("GetSystemMetricsForDpi")
	procLoadImage                     = user32.NewProc("LoadImageW")
	procSendMessage                   = user32.NewProc("SendMessageW")
	procSetWindowLongPtr              = user32.NewProc("SetWindowLongPtrW")
	procCallWindowProc                = user32.NewProc("CallWindowProcW")
	procFindWindow                    = user32.NewProc("FindWindowW")
	procShowWindow                    = user32.NewProc("ShowWindow")
	procSetForegroundWindow           = user32.NewProc("SetForegroundWindow")
	procDwmSetWindowAttribute         = dwmapi.NewProc("DwmSetWindowAttribute")
)

const (
	wmClose     = 0x0010
	wmSetIcon   = 0x0080
	gwlpWndProc = ^uintptr(3) // -4
	swRestore   = 9

	iconSmall  = 0  // ICON_SMALL: the icon of the title bar
	iconBig    = 1  // ICON_BIG: the icon of the taskbar and of Alt+Tab
	imageIcon  = 1  // IMAGE_ICON
	smCxIcon   = 11 // SM_CXICON
	smCxSmIcon = 49 // SM_CXSMICON

	// programIconID is the ID of the icon group that tools/mkrsrc writes into the
	// resources of the program.
	programIconID = 1

	// dpiAwarenessPerMonitorV2 is DPI_AWARENESS_CONTEXT_PER_MONITOR_AWARE_V2, -4.
	dpiAwarenessPerMonitorV2 = ^uintptr(3)

	// shutdownTimeout is how long closing the window waits for a request that
	// Windows is working on. One request takes about seven seconds.
	shutdownTimeout = 30 * time.Second
)

// enableDPIAwareness makes the window sharp on a display that is scaled. It has
// to run before the window is created, and does nothing on a Windows that does
// not have the call.
func enableDPIAwareness() {
	if procSetProcessDpiAwarenessContext.Find() == nil {
		procSetProcessDpiAwarenessContext.Call(dpiAwarenessPerMonitorV2)
	}
}

// systemDPIScale is the scale of the display: 1 at 96 DPI, 1.5 at 144 DPI.
func systemDPIScale() float64 {
	if procGetDpiForSystem.Find() != nil {
		return 1
	}
	dpi, _, _ := procGetDpiForSystem.Call()
	if dpi == 0 {
		return 1
	}
	return float64(dpi) / 96
}

// setWindowIcons gives the window the icon of the program in the two sizes that
// Windows shows at the DPI of the window: the small one for the title bar and the
// big one for the taskbar. The binding sets one size, which Windows then scales.
// A program built without the resources has no icon to load, and keeps the
// default one.
func setWindowIcons(hwnd uintptr) {
	var module windows.Handle
	if err := windows.GetModuleHandleEx(0, nil, &module); err != nil {
		return
	}
	dpi := uintptr(96)
	if procGetDpiForWindow.Find() == nil {
		if d, _, _ := procGetDpiForWindow.Call(hwnd); d != 0 {
			dpi = d
		}
	}
	for _, icon := range []struct{ which, metric uintptr }{{iconSmall, smCxSmIcon}, {iconBig, smCxIcon}} {
		var size uintptr
		if procGetSystemMetricsForDpi.Find() == nil {
			size, _, _ = procGetSystemMetricsForDpi.Call(icon.metric, dpi)
		} else {
			size, _, _ = procGetSystemMetrics.Call(icon.metric)
		}
		if size == 0 {
			continue
		}
		if handle, _, _ := procLoadImage.Call(uintptr(module), programIconID, imageIcon, size, size, 0); handle != 0 {
			procSendMessage.Call(hwnd, wmSetIcon, icon.which, handle)
		}
	}
}

// applyDarkTitleBar gives the title bar the dark theme when the apps of the user
// use it, so that it matches the page. A window does not do this by itself.
func applyDarkTitleBar(hwnd uintptr) {
	key, err := registry.OpenKey(registry.CURRENT_USER, `Software\Microsoft\Windows\CurrentVersion\Themes\Personalize`, registry.QUERY_VALUE)
	if err != nil {
		return
	}
	defer key.Close()
	if light, _, err := key.GetIntegerValue("AppsUseLightTheme"); err != nil || light != 0 {
		return
	}
	if procDwmSetWindowAttribute.Find() != nil {
		return
	}
	on := int32(1)
	// DWMWA_USE_IMMERSIVE_DARK_MODE is 20; before Windows 10 20H1 it was 19.
	for _, attribute := range []uintptr{20, 19} {
		if r, _, _ := procDwmSetWindowAttribute.Call(hwnd, attribute, uintptr(unsafe.Pointer(&on)), unsafe.Sizeof(on)); r == 0 {
			return
		}
	}
}

// closeGuard intercepts WM_CLOSE. The first one starts the shutdown of the
// application, which waits for the request that Windows may be working on, and
// is ignored; when the shutdown is done the window is closed for real.
type closeGuard struct {
	app      *app.App
	window   webview2.WebView
	original uintptr
	log      *slog.Logger
	started  atomic.Bool
	finished atomic.Bool
}

func installCloseGuard(hwnd uintptr, a *app.App, w webview2.WebView, log *slog.Logger) {
	g := &closeGuard{app: a, window: w, log: log}
	g.original, _, _ = procSetWindowLongPtr.Call(hwnd, gwlpWndProc, windows.NewCallback(g.proc))
}

func (g *closeGuard) proc(hwnd, msg, wparam, lparam uintptr) uintptr {
	if msg == wmClose && !g.finished.Load() {
		if g.started.CompareAndSwap(false, true) {
			go func() {
				if !g.app.Shutdown(shutdownTimeout) {
					g.log.Error("a request did not finish in time, closing anyway")
				}
				g.finished.Store(true)
				g.window.Destroy() // posts WM_CLOSE again, which now goes through
			}()
		}
		return 0
	}
	r, _, _ := procCallWindowProc.Call(g.original, hwnd, msg, wparam, lparam)
	return r
}

// SingleInstance makes sure that only one window runs. When another one does, it
// is brought to the front and ok is false. The mutex is released by the returned
// function, or when the process ends.
func SingleInstance(mutexName, windowTitle string) (release func(), ok bool) {
	name, err := windows.UTF16PtrFromString(mutexName)
	if err != nil {
		return func() {}, true
	}
	h, err := windows.CreateMutex(nil, false, name)
	if errors.Is(err, windows.ERROR_ALREADY_EXISTS) {
		if h != 0 {
			windows.CloseHandle(h)
		}
		bringToFront(windowTitle)
		return nil, false
	}
	if err != nil {
		return func() {}, true // no guard is better than no window
	}
	return func() { windows.CloseHandle(h) }, true
}

func bringToFront(title string) {
	t, err := windows.UTF16PtrFromString(title)
	if err != nil {
		return
	}
	hwnd, _, _ := procFindWindow.Call(0, uintptr(unsafe.Pointer(t)))
	if hwnd == 0 {
		return
	}
	procShowWindow.Call(hwnd, swRestore)
	procSetForegroundWindow.Call(hwnd)
}

// ShowError tells the user something in a message box. The window build has no
// console to write to.
func ShowError(title, text string) {
	t, err1 := windows.UTF16PtrFromString(title)
	m, err2 := windows.UTF16PtrFromString(text)
	if err1 != nil || err2 != nil {
		return
	}
	windows.MessageBox(0, m, t, windows.MB_OK|windows.MB_ICONERROR)
}
