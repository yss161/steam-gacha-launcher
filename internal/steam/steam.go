// Package steam 负责 Steam 安装目录/用户目录检测与运行状态判断。
package steam

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"golang.org/x/sys/windows/registry"
)

// FindSteamPath 返回 Steam 安装目录。cfgPath 非空且有效时优先使用。
func FindSteamPath(cfgPath string) (string, error) {
	if cfgPath != "" {
		if fi, err := os.Stat(cfgPath); err == nil && fi.IsDir() {
			return cfgPath, nil
		}
		return "", fmt.Errorf("配置的 Steam 目录无效: %s", cfgPath)
	}
	if k, err := registry.OpenKey(registry.CURRENT_USER, `Software\Valve\Steam`, registry.QUERY_VALUE); err == nil {
		if p, _, err := k.GetStringValue("SteamPath"); err == nil && p != "" {
			if fi, err := os.Stat(p); err == nil && fi.IsDir() {
				k.Close()
				return p, nil
			}
		}
		k.Close()
	}
	for _, p := range []string{
		`C:\Program Files (x86)\Steam`,
		`C:\Program Files\Steam`,
		`D:\Steam`,
		`E:\Steam`,
	} {
		if fi, err := os.Stat(p); err == nil && fi.IsDir() {
			return p, nil
		}
	}
	return "", fmt.Errorf("未找到 Steam 安装目录，请通过配置 steam.path 指定")
}

// FindUserDataDir 返回 Steam 用户数据目录（userdata/<accountID>）。
// accountID 非空时直接使用；否则选择最近使用（config 目录修改时间最新）的账号。
func FindUserDataDir(steamPath, accountID string) (string, error) {
	base := filepath.Join(steamPath, "userdata")
	if accountID != "" {
		d := filepath.Join(base, accountID)
		if fi, err := os.Stat(d); err == nil && fi.IsDir() {
			return d, nil
		}
		return "", fmt.Errorf("未找到 Steam 账号目录: %s", d)
	}
	entries, err := os.ReadDir(base)
	if err != nil {
		return "", fmt.Errorf("未找到 userdata 目录: %s", base)
	}
	best := ""
	var bestTime int64 = -1
	for _, e := range entries {
		if !e.IsDir() || e.Name() == "anonymous" || e.Name() == "0" {
			continue
		}
		cfgDir := filepath.Join(base, e.Name(), "config")
		info, err := os.Stat(cfgDir)
		if err != nil {
			continue
		}
		if info.ModTime().Unix() > bestTime {
			bestTime = info.ModTime().Unix()
			best = filepath.Join(base, e.Name())
		}
	}
	if best == "" {
		return "", fmt.Errorf("未找到有效的 Steam 用户数据目录，请通过配置 steam.account_id 指定")
	}
	return best, nil
}

// IsSteamRunning 判断 Steam 客户端是否正在运行。
func IsSteamRunning() bool {
	out, err := exec.Command("tasklist", "/FI", "IMAGENAME eq steam.exe", "/NH").Output()
	if err != nil {
		return false
	}
	return strings.Contains(strings.ToLower(string(out)), "steam.exe")
}
