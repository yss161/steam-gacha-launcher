//go:build !windows

package main

import "fmt"

// openNativeGUI 的降级实现：原生界面仅支持 Windows。
func openNativeGUI(c *cli) error {
	return fmt.Errorf("原生图形界面仅支持 Windows")
}
