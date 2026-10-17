//go:build !windows || !cgo

package main

import "fmt"

// openNativeGUI 的降级实现：非 Windows 或未启用 CGO 的构建下不可用。
func openNativeGUI(c *cli) error {
	return fmt.Errorf("当前构建未启用原生窗口（需 Windows + CGO）。请改用 `steam-gacha-launcher web` 打开浏览器界面")
}
