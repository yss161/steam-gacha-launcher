package steam

import (
	"fmt"
	"hash/crc32"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// GenerateAppId 计算非 Steam 游戏快捷方式的 appid（32 位，最高位为 1）。
// 算法与 Steam ROM Manager 一致：crc32(exe + appname) | 0x80000000。
func GenerateAppId(exePath, appName string) uint32 {
	crc := crc32.ChecksumIEEE([]byte(exePath + appName))
	return crc | 0x80000000
}

// GenerateLongAppId 计算用于 SteamGridDB 封面命名的长 appid（64 位）。
func GenerateLongAppId(exePath, appName string) uint64 {
	top := uint64(GenerateAppId(exePath, appName))
	return (top << 32) | 0x02000000
}

// LaunchURI 返回启动用 steam:// URI。
func LaunchURI(appID uint32) string {
	return fmt.Sprintf("steam://rungameid/%d", uint32(appID))
}

// ShortcutsPath 返回 shortcuts.vdf 路径。
func ShortcutsPath(userDataDir string) string {
	return filepath.Join(userDataDir, "config", "shortcuts.vdf")
}

// AddShortcut 向 shortcuts.vdf 追加一条非 Steam 游戏快捷方式，返回 appid。
// 会先备份原文件（<path>.bak），并在解析失败时拒绝写入。
func AddShortcut(vdfPath, exe, startDir, appName, launchOptions string, tags []string, icon string) (uint32, error) {
	appID := GenerateAppId(exe, appName)

	data, err := os.ReadFile(vdfPath)
	notExist := os.IsNotExist(err)
	if err != nil && !notExist {
		return 0, fmt.Errorf("读取 %s 失败: %w", vdfPath, err)
	}

	var root *vdfNode
	if notExist || len(data) == 0 {
		root = newObject()
		appendField(root, "shortcuts", newObject())
	} else {
		root, err = parseVDF(data)
		if err != nil {
			return 0, fmt.Errorf("解析 %s 失败（文件可能损坏，已不写入）: %w", vdfPath, err)
		}
		if root.children["shortcuts"] == nil {
			appendField(root, "shortcuts", newObject())
		}
	}

	sc := root.children["shortcuts"]
	// 去重：按 Exe（大小写不敏感）
	for _, k := range sc.keys {
		c := sc.children[k]
		if c == nil {
			continue
		}
		if e := c.children["Exe"]; e != nil && e.typ == vdfString && strings.EqualFold(e.str, exe) {
			return 0, fmt.Errorf("该游戏已存在于 Steam 库（Exe: %s），无需重复添加", exe)
		}
	}

	entry := buildEntry(appID, exe, startDir, appName, launchOptions, tags, icon)
	idx := strconv.Itoa(len(sc.keys))
	appendField(sc, idx, entry)

	out := root.serialize()

	// 备份
	if !notExist {
		if berr := os.WriteFile(vdfPath+".bak", data, 0o644); berr != nil {
			return 0, fmt.Errorf("备份 %s 失败: %w", vdfPath, berr)
		}
	}
	if werr := os.WriteFile(vdfPath, out, 0o644); werr != nil {
		return 0, fmt.Errorf("写入 %s 失败: %w", vdfPath, werr)
	}
	return appID, nil
}

// buildEntry 构造单个快捷方式条目。
func buildEntry(appID uint32, exe, startDir, appName, launchOptions string, tags []string, icon string) *vdfNode {
	if icon == "" {
		icon = exe
	}
	e := newObject()
	appendField(e, "appid", newInt(int32(appID)))
	appendField(e, "AppName", newString(appName))
	appendField(e, "Exe", newString(exe))
	appendField(e, "StartDir", newString(startDir))
	appendField(e, "icon", newString(icon))
	appendField(e, "ShortcutPath", newString(""))
	appendField(e, "LaunchOptions", newString(launchOptions))
	appendField(e, "IsHidden", newInt(0))
	appendField(e, "AllowDesktopConfig", newInt(1))
	appendField(e, "AllowOverlay", newInt(1))
	appendField(e, "OpenVR", newInt(0))
	appendField(e, "Devkit", newInt(0))
	appendField(e, "DevkitGameID", newString(""))
	appendField(e, "DevkitOverrideAppID", newInt(0))
	appendField(e, "LastPlayTime", newInt(0))
	appendField(e, "FlatpakAppID", newString(""))

	tagsNode := newObject()
	for i, t := range tags {
		appendField(tagsNode, strconv.Itoa(i), newString(t))
	}
	appendField(e, "tags", tagsNode)
	return e
}

// RemoveShortcut 从 shortcuts.vdf 移除匹配 Exe 的快捷方式，返回是否移除成功。
func RemoveShortcut(vdfPath, exe string) (bool, error) {
	data, err := os.ReadFile(vdfPath)
	if err != nil {
		return false, fmt.Errorf("读取 %s 失败: %w", vdfPath, err)
	}
	root, err := parseVDF(data)
	if err != nil {
		return false, fmt.Errorf("解析 %s 失败: %w", vdfPath, err)
	}
	sc := root.children["shortcuts"]
	if sc == nil {
		return false, nil
	}
	var keep []string
	removed := false
	for _, k := range sc.keys {
		c := sc.children[k]
		if c == nil {
			continue
		}
		if e := c.children["Exe"]; e != nil && e.typ == vdfString && strings.EqualFold(e.str, exe) {
			removed = true
			continue
		}
		keep = append(keep, k)
	}
	if !removed {
		return false, nil
	}
	// 重新规范化索引
	newChildren := map[string]*vdfNode{}
	sc.keys = sc.keys[:0]
	for i, old := range keep {
		ni := strconv.Itoa(i)
		sc.keys = append(sc.keys, ni)
		newChildren[ni] = sc.children[old]
	}
	sc.children = newChildren

	out := root.serialize()
	if berr := os.WriteFile(vdfPath+".bak", data, 0o644); berr != nil {
		return false, fmt.Errorf("备份失败: %w", berr)
	}
	if werr := os.WriteFile(vdfPath, out, 0o644); werr != nil {
		return false, fmt.Errorf("写入失败: %w", werr)
	}
	return true, nil
}

// ListShortcuts 返回 vdf 中已有的快捷方式（appid + 名称 + Exe）。
func ListShortcuts(vdfPath string) ([][3]string, error) {
	data, err := os.ReadFile(vdfPath)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	root, err := parseVDF(data)
	if err != nil {
		return nil, err
	}
	sc := root.children["shortcuts"]
	if sc == nil {
		return nil, nil
	}
	var out [][3]string
	for _, k := range sc.keys {
		c := sc.children[k]
		if c == nil {
			continue
		}
		id := ""
		if a := c.children["appid"]; a != nil && a.typ == vdfInt {
			id = strconv.FormatUint(uint64(uint32(a.i32)), 10)
		}
		name := ""
		if n := c.children["AppName"]; n != nil && n.typ == vdfString {
			name = n.str
		}
		exe := ""
		if e := c.children["Exe"]; e != nil && e.typ == vdfString {
			exe = e.str
		}
		out = append(out, [3]string{id, name, exe})
	}
	return out, nil
}
