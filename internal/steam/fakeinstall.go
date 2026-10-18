package steam

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// 「假入库」：伪造 appmanifest + 目录联接，让 Steam 把游戏当作已安装的库内游戏。
//
// 原理（纯配置操作，无注入、不绕过任何校验）：
//  1. Steam 客户端启动时扫描 steamapps/ 下的 appmanifest_<appid>.acf 决定库中显示哪些游戏；
//  2. 完整清单需 StateFlags=4（已完全安装）+ 真实 buildid + InstalledDepots（depot 及 manifest GID），
//     Steam 据此认为「该版本这些内容已安装」，不再触发下载（经终末地实测验证）；
//  3. 用目录联接（junction）把 steamapps/common/<installdir> 指向你的国服安装目录，
//     使 Steam 按其启动配置在联接目录里找到主程序；
//  4. LastOwner 设为当前登录账号的 SteamID64。
//
// 注意：appid 必须是该游戏真实的 Steam appid；buildid/depot manifest 会随游戏更新而变化，
// 过期后 Steam 可能提示更新（可在配置中用 steam_buildid / steam_depot_manifest 覆盖）。

// DepotInfo 描述一个游戏在 Steam 的 depot 信息（用于生成完整清单）。
type DepotInfo struct {
	BuildID  string // 当前公开分支 buildid
	DepotID  string // 主 depot id
	Manifest string // depot 当前 manifest GID
	Size     int64  // depot 磁盘大小（字节）
}

// LibraryFolders 返回所有 Steam 库目录（主目录 + libraryfolders.vdf 中的附加库）。
func LibraryFolders(steamPath string) []string {
	out := []string{filepath.Join(steamPath, "steamapps")}
	data, err := os.ReadFile(filepath.Join(steamPath, "steamapps", "libraryfolders.vdf"))
	if err != nil {
		return out
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, `"path"`) {
			parts := splitVDFLine(line)
			if len(parts) == 2 {
				p := strings.ReplaceAll(parts[1], `\\`, `\`)
				out = append(out, filepath.Join(p, "steamapps"))
			}
		}
	}
	return out
}

// splitVDFLine 把 `"key"\t\t"value"` 形式的行拆成 [key, value]。
func splitVDFLine(line string) []string {
	var parts []string
	inQuote := false
	var cur strings.Builder
	for _, r := range line {
		switch r {
		case '"':
			if inQuote {
				parts = append(parts, cur.String())
				cur.Reset()
				inQuote = false
			} else {
				inQuote = true
			}
		default:
			if inQuote {
				cur.WriteRune(r)
			}
		}
	}
	return parts
}

// CurrentSteamID64 解析 config/loginusers.vdf，返回当前账号（AutoLogin=1 或最近登录）的 SteamID64。
// 未找到返回 "0"。
func CurrentSteamID64(steamPath string) string {
	data, err := os.ReadFile(filepath.Join(steamPath, "config", "loginusers.vdf"))
	if err != nil {
		return "0"
	}
	type acct struct {
		id        string
		autoLogin bool
		ts        int64
	}
	var accts []acct
	var cur *acct
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		parts := splitVDFLine(line)
		if len(parts) == 1 && strings.HasPrefix(parts[0], "7656") {
			accts = append(accts, acct{id: parts[0]})
			cur = &accts[len(accts)-1]
			continue
		}
		if cur == nil || len(parts) != 2 {
			continue
		}
		switch strings.ToLower(parts[0]) {
		case "autologin":
			cur.autoLogin = parts[1] == "1"
		case "timestamp":
			if ts, err := strconv.ParseInt(parts[1], 10, 64); err == nil {
				cur.ts = ts
			}
		}
	}
	best := ""
	var bestTS int64 = -1
	for _, a := range accts {
		if a.autoLogin {
			return a.id
		}
		if a.ts > bestTS {
			bestTS = a.ts
			best = a.id
		}
	}
	if best == "" {
		return "0"
	}
	return best
}

// FakeInstall 让游戏在 Steam 库中显示为已安装（完整清单）。
//   steamPath: Steam 安装目录
//   appID: 真实 Steam appid
//   name: 游戏显示名
//   installdir: steamapps/common 下的目录名
//   contentDir: 游戏内容根目录（junction 目标，其内部结构需与 Steam depot 一致）
//   depot: depot 信息（buildid/depotid/manifest/size）
func FakeInstall(steamPath, appID, name, installdir, contentDir string, depot DepotInfo) (string, error) {
	if appID == "" {
		return "", fmt.Errorf("未提供 Steam appid（该游戏需在 Steam 有对应版本）")
	}
	if _, err := os.Stat(contentDir); err != nil {
		return "", fmt.Errorf("游戏目录不存在: %s", contentDir)
	}
	steamapps := filepath.Join(steamPath, "steamapps")
	linkDir := filepath.Join(steamapps, "common", installdir)
	acfPath := filepath.Join(steamapps, "appmanifest_"+appID+".acf")

	if _, err := os.Lstat(linkDir); err == nil {
		return "", fmt.Errorf("库中目录已存在: %s（若此前假入库过请先「取消假入库」）", linkDir)
	}

	// 1. 创建目录联接（无需管理员）
	if err := createJunction(linkDir, contentDir); err != nil {
		return "", err
	}

	// 2. 生成完整 appmanifest
	owner := CurrentSteamID64(steamPath)
	acf := generateACF(appID, name, installdir, steamPath, owner, depot)
	if err := os.WriteFile(acfPath, []byte(acf), 0o644); err != nil {
		_ = removeJunction(linkDir)
		return "", fmt.Errorf("写入 appmanifest 失败: %w", err)
	}

	return linkDir, nil
}

// RemoveFakeInstall 取消假入库：删除 appmanifest 与 junction（不影响真实游戏文件）。
func RemoveFakeInstall(steamPath, appID, installdir string) error {
	if appID == "" {
		return fmt.Errorf("未提供 Steam appid")
	}
	steamapps := filepath.Join(steamPath, "steamapps")
	linkDir := filepath.Join(steamapps, "common", installdir)
	acfPath := filepath.Join(steamapps, "appmanifest_"+appID+".acf")

	var errs []string
	if err := os.Remove(acfPath); err != nil && !os.IsNotExist(err) {
		errs = append(errs, "删除 appmanifest: "+err.Error())
	}
	if err := removeJunction(linkDir); err != nil && !os.IsNotExist(err) {
		errs = append(errs, "删除目录联接: "+err.Error())
	}
	if len(errs) > 0 {
		return fmt.Errorf(strings.Join(errs, "; "))
	}
	return nil
}

// IsFakeInstalled 判断指定 appid 是否已假入库。
func IsFakeInstalled(steamPath, appID string) bool {
	if appID == "" {
		return false
	}
	_, err := os.Stat(filepath.Join(steamPath, "steamapps", "appmanifest_"+appID+".acf"))
	return err == nil
}

// generateACF 生成完整 appmanifest 文本（KeyValues 格式，含 buildid 与 InstalledDepots）。
func generateACF(appID, name, installdir, steamPath, owner string, depot DepotInfo) string {
	launcherPath := strings.ReplaceAll(filepath.Join(steamPath, "steam.exe"), `\`, `\\`)
	now := time.Now().Unix()
	if owner == "" {
		owner = "0"
	}
	buildid := depot.BuildID
	if buildid == "" {
		buildid = "0"
	}
	depotID := depot.DepotID
	manifest := depot.Manifest
	size := depot.Size
	if depotID == "" {
		// 无 depot 信息时退化为最小结构（仍可能触发 Steam 校验/下载）
		depotID = appID + "1"
		manifest = "0"
		size = 0
	}

	var b strings.Builder
	b.WriteString("\"AppState\"\n{\n")
	kv := func(k, v string, indent int) {
		tabs := strings.Repeat("\t", indent)
		b.WriteString(tabs + "\"" + k + "\"" + tabs + tabs + "\"" + v + "\"\n")
	}
	kv("appid", appID, 1)
	kv("universe", "1", 1)
	kv("LauncherPath", launcherPath, 1)
	kv("name", name, 1)
	kv("StateFlags", "4", 1)
	kv("installdir", installdir, 1)
	kv("LastUpdated", strconv.FormatInt(now, 10), 1)
	kv("LastPlayed", "0", 1)
	kv("SizeOnDisk", strconv.FormatInt(size, 10), 1)
	kv("StagingSize", "0", 1)
	kv("buildid", buildid, 1)
	kv("LastOwner", owner, 1)
	kv("DownloadType", "0", 1)
	kv("UpdateResult", "0", 1)
	kv("BytesToDownload", "0", 1)
	kv("BytesDownloaded", "0", 1)
	kv("BytesToStage", "0", 1)
	kv("BytesStaged", "0", 1)
	kv("TargetBuildID", "0", 1)
	kv("AutoUpdateBehavior", "0", 1)
	kv("AllowOtherDownloadsWhileRunning", "0", 1)
	kv("ScheduledAutoUpdate", "0", 1)
	b.WriteString("\t\"InstalledDepots\"\n\t{\n")
	b.WriteString("\t\t\"" + depotID + "\"\n\t\t{\n")
	kv("manifest", manifest, 3)
	kv("size", strconv.FormatInt(size, 10), 3)
	b.WriteString("\t\t}\n\t}\n")
	b.WriteString("\t\"UserConfig\"\n\t{\n")
	kv("language", "schinese", 2)
	b.WriteString("\t}\n")
	b.WriteString("\t\"MountedConfig\"\n\t{\n")
	kv("language", "schinese", 2)
	b.WriteString("\t}\n")
	b.WriteString("}\n")
	return b.String()
}

// createJunction 用 mklink /J 创建目录联接（无需管理员权限）。
func createJunction(link, target string) error {
	cmd := exec.Command("cmd", "/c", "mklink", "/J", link, target)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("创建目录联接失败: %w (%s)", err, strings.TrimSpace(string(out)))
	}
	return nil
}

// removeJunction 删除目录联接（rmdir 只删除联接本身，不删除目标目录内容）。
func removeJunction(link string) error {
	if _, err := os.Lstat(link); os.IsNotExist(err) {
		return nil
	}
	cmd := exec.Command("cmd", "/c", "rmdir", link)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%w (%s)", err, strings.TrimSpace(string(out)))
	}
	return nil
}
