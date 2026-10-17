// Package detect 负责定位游戏客户端：注册表、常见路径、有限深度目录搜索。
package detect

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/sys/windows/registry"

	"github.com/yss161/steam-gacha-launcher/internal/games"
)

// Result 为定位结果。
type Result struct {
	ExePath string // 主程序绝对路径
	WorkDir string // 工作目录（主程序所在目录）
	RootDir string // 安装根目录
}

// Locate 定位内置游戏。overridePath/overrideExe 非空时优先使用。
func Locate(p *games.Profile, overridePath, overrideExe string) (*Result, error) {
	if p == nil {
		return nil, fmt.Errorf("空游戏画像")
	}
	if overridePath != "" {
		if r, ok := resolveInDir(overridePath, overrideExe, p); ok {
			return r, nil
		}
		return nil, fmt.Errorf("在指定路径 %s 中未找到游戏主程序", overridePath)
	}
	for _, d := range candidateDirs(p) {
		if r, ok := resolveInDir(d, overrideExe, p); ok {
			return r, nil
		}
	}
	return nil, fmt.Errorf("未找到游戏「%s」(%s)，请用 -path 或配置文件中 games.%s.path 指定安装路径", p.Name, p.ID, p.ID)
}

func resolveInDir(dir, overrideExe string, p *games.Profile) (*Result, bool) {
	if overrideExe != "" {
		cand := overrideExe
		if !filepath.IsAbs(cand) {
			cand = filepath.Join(dir, cand)
		}
		if fi, err := os.Stat(cand); err == nil && !fi.IsDir() {
			return &Result{ExePath: cand, WorkDir: filepath.Dir(cand), RootDir: dir}, true
		}
		return nil, false
	}
	for _, n := range p.ExeNames {
		cand := n
		if !filepath.IsAbs(cand) {
			cand = filepath.Join(dir, n)
		}
		if fi, err := os.Stat(cand); err == nil && !fi.IsDir() {
			return &Result{ExePath: cand, WorkDir: filepath.Dir(cand), RootDir: dir}, true
		}
	}
	// 回退：在根目录下做有限深度搜索（最多 5 层）
	for _, n := range p.ExeNames {
		base := filepath.Base(n)
		if p, ok := findExe(dir, base, 5); ok {
			return &Result{ExePath: p, WorkDir: filepath.Dir(p), RootDir: dir}, true
		}
	}
	return nil, false
}

func candidateDirs(p *games.Profile) []string {
	var dirs []string
	seen := map[string]bool{}
	add := func(d string) {
		d = strings.TrimSpace(d)
		if d == "" || seen[d] {
			return
		}
		seen[d] = true
		dirs = append(dirs, d)
	}
	for _, h := range p.RegHints {
		for _, d := range registryCandidates(h) {
			add(d)
		}
	}
	for _, d := range p.DefaultPaths {
		add(d)
	}
	return dirs
}

func registryCandidates(h games.RegHint) []string {
	var out []string
	base := h.Path
	if base == "" {
		base = `SOFTWARE\Microsoft\Windows\CurrentVersion\Uninstall`
	}
	value := h.Value
	if value == "" {
		value = "InstallLocation"
	}
	enumerate := func(root registry.Key, basePath string) {
		k, err := registry.OpenKey(root, basePath, registry.ENUMERATE_SUB_KEYS|registry.QUERY_VALUE)
		if err != nil {
			return
		}
		defer k.Close()
		names, err := k.ReadSubKeyNames(-1)
		if err != nil {
			return
		}
		for _, name := range names {
			sk, err := registry.OpenKey(k, name, registry.QUERY_VALUE)
			if err != nil {
				continue
			}
			display, _, _ := sk.GetStringValue("DisplayName")
			if matchesAny(display, h.Match) {
				loc, _, _ := sk.GetStringValue(value)
				if loc == "" {
					loc, _, _ = sk.GetStringValue("InstallLocation")
				}
				if loc == "" {
					// 部分游戏仅提供 DisplayIcon，可从中解析路径
					icon, _, _ := sk.GetStringValue("DisplayIcon")
					loc = dirFromIcon(icon)
				}
				if loc != "" {
					out = append(out, loc)
				}
			}
			sk.Close()
		}
	}

	root := registry.LOCAL_MACHINE
	if h.Root == "HKCU" {
		root = registry.CURRENT_USER
	}
	enumerate(root, base)
	if root == registry.LOCAL_MACHINE {
		// 32 位视图（WOW6432Node）
		if strings.HasPrefix(base, "SOFTWARE\\") {
			enumerate(root, "SOFTWARE\\WOW6432Node\\"+strings.TrimPrefix(base, "SOFTWARE\\"))
		}
	}
	return out
}

func matchesAny(s string, subs []string) bool {
	for _, sub := range subs {
		if sub != "" && strings.Contains(s, sub) {
			return true
		}
	}
	return false
}

func dirFromIcon(icon string) string {
	icon = strings.TrimSpace(icon)
	if icon == "" {
		return ""
	}
	// 形如 "C:\path\to.exe,0" 或带引号
	icon = strings.Trim(icon, `"`)
	if i := strings.LastIndex(icon, ","); i >= 0 {
		icon = icon[:i]
	}
	return filepath.Dir(icon)
}

// findExe 在 root 下做深度受限的文件名搜索（大小写不敏感）。
func findExe(root, name string, maxDepth int) (string, bool) {
	var found string
	var walk func(dir string, depth int)
	walk = func(dir string, depth int) {
		if found != "" || depth > maxDepth {
			return
		}
		entries, err := os.ReadDir(dir)
		if err != nil {
			return
		}
		for _, e := range entries {
			if found != "" {
				return
			}
			if e.IsDir() {
				walk(filepath.Join(dir, e.Name()), depth+1)
			} else if strings.EqualFold(e.Name(), name) {
				found = filepath.Join(dir, e.Name())
				return
			}
		}
	}
	walk(root, 0)
	return found, found != ""
}
