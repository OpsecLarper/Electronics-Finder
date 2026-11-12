//go:build windows

package main

import (
	"path/filepath"
	"unsafe"

	webview2 "github.com/jchv/go-webview2"
	"golang.org/x/sys/windows"
)

var (
	dwmapi               = windows.NewLazySystemDLL("dwmapi.dll")
	procDwmSetWindowAttr = dwmapi.NewProc("DwmSetWindowAttribute")
	user32               = windows.NewLazySystemDLL("user32.dll")
	procMessageBoxW      = user32.NewProc("MessageBoxW")
)

// runWindow shows the UI in a native WebView2 window. Returns false if
// WebView2 isn't available so the caller can fall back to a browser.
func runWindow(addr string) (ok bool) {
	defer func() {
		if r := recover(); r != nil {
			ok = false
		}
	}()
	w := webview2.NewWithOptions(webview2.WebViewOptions{
		AutoFocus: true,
		DataPath:  filepath.Join(appDir(), "webview"),
		WindowOptions: webview2.WindowOptions{
			Title:  "Electronics Finder",
			Width:  1320,
			Height: 860,
			IconId: 1,
			Center: true,
		},
	})
	if w == nil {
		return false
	}
	defer w.Destroy()
	w.SetSize(980, 640, webview2.HintMin)
	if hwnd := uintptr(w.Window()); hwnd != 0 {
		on := int32(1)
		// DWMWA_USE_IMMERSIVE_DARK_MODE (20) — dark title bar on Win10 20H1+/Win11.
		procDwmSetWindowAttr.Call(hwnd, 20, uintptr(unsafe.Pointer(&on)), 4)
		// DWMWA_SYSTEMBACKDROP_TYPE (38) = 2 (Mica) on Windows 11 for the frame.
		mica := int32(2)
		procDwmSetWindowAttr.Call(hwnd, 38, uintptr(unsafe.Pointer(&mica)), 4)
	}
	w.Navigate(addr)
	w.Run()
	return true
}

func showError(msg string) {
	t, _ := windows.UTF16PtrFromString("Electronics Finder")
	m, _ := windows.UTF16PtrFromString(msg)
	procMessageBoxW.Call(0, uintptr(unsafe.Pointer(m)), uintptr(unsafe.Pointer(t)), 0x10)
}
