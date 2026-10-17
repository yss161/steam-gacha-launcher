//go:build windows && cgo

package main

import (
	"fmt"
	"net"
	"net/http"
	"unsafe"

	"golang.org/x/sys/windows"

	webview "github.com/webview/webview_go"
)

// openNativeGUI 启动本地 HTTP 服务，并在原生窗口（Edge WebView2）中显示界面。
// 仅在 Windows + CGO 构建下编译；其它构建见 gui_stub.go。
func openNativeGUI(c *cli) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("界面异常: %v", r)
		}
		if err != nil {
			showErrorBox("启动失败\n" + err.Error())
		}
	}()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return fmt.Errorf("启动本地服务失败: %w", err)
	}
	srv := newWebServer(c.configPath)
	url := "http://" + ln.Addr().String()
	go func() { _ = http.Serve(ln, srv.handler()) }()

	w := webview.New(false)
	if w == nil {
		return fmt.Errorf("创建窗口失败（可能缺少 WebView2 运行时，Windows 10 请安装 Edge WebView2）")
	}
	defer w.Destroy()
	w.SetTitle("Steam 国服二游启动器")
	w.SetSize(1080, 720, webview.HintMin)
	w.Navigate(url)
	w.Run()
	return nil
}

// showErrorBox 弹出原生错误提示框（GUI 子系统下双击启动时没有控制台，靠它提示）。
func showErrorBox(msg string) {
	user32 := windows.NewLazySystemDLL("user32.dll")
	msgBox := user32.NewProc("MessageBoxW")
	text, _ := windows.UTF16PtrFromString(msg)
	title, _ := windows.UTF16PtrFromString("Steam 国服二游启动器")
	_, _, _ = msgBox.Call(0, uintptr(unsafe.Pointer(text)), uintptr(unsafe.Pointer(title)), 0x10)
}
