// steam-gacha-launcher —— 通过 Steam 启动国服二游的小工具。
//
// 核心能力：
//   - 检测并定位游戏客户端（注册表 / 常见路径 / 目录搜索）
//   - 直接启动游戏，或经官方启动器启动
//   - 将游戏注册为 Steam 非 Steam 游戏快捷方式（写入 shortcuts.vdf）
//   - 输出 steam:// 启动 URI
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/yss161/steam-gacha-launcher/internal/config"
	"github.com/yss161/steam-gacha-launcher/internal/detect"
	"github.com/yss161/steam-gacha-launcher/internal/games"
	"github.com/yss161/steam-gacha-launcher/internal/launch"
	"github.com/yss161/steam-gacha-launcher/internal/steam"
	"github.com/yss161/steam-gacha-launcher/internal/update"
)

var version = "dev"

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "\n[错误] %v\n", err)
		os.Exit(1)
	}
}

// parseArgs 解析命令行参数，支持「位置参数与选项穿插」的写法（如 launch yi-huan -path ...）。
func parseArgs(args []string) (*cli, []string, error) {
	c := &cli{}
	var pos []string
	i := 0
	for i < len(args) {
		a := args[i]
		switch a {
		case "-config":
			if i+1 >= len(args) {
				return nil, nil, fmt.Errorf("%s 需要一个值", a)
			}
			c.configPath = args[i+1]
			i += 2
		case "-mode":
			if i+1 >= len(args) {
				return nil, nil, fmt.Errorf("%s 需要一个值", a)
			}
			c.mode = args[i+1]
			i += 2
		case "-path":
			if i+1 >= len(args) {
				return nil, nil, fmt.Errorf("%s 需要一个值", a)
			}
			c.pathOverride = args[i+1]
			i += 2
		case "-args":
			if i+1 >= len(args) {
				return nil, nil, fmt.Errorf("%s 需要一个值", a)
			}
			c.argsOverride = args[i+1]
			i += 2
		case "-wait":
			c.wait = true
			i++
		case "-dry-run":
			c.dryRun = true
			i++
		case "-force":
			c.force = true
			i++
		default:
			pos = append(pos, a)
			i++
		}
	}
	return c, pos, nil
}

type cli struct {
	configPath   string
	mode         string
	pathOverride string
	argsOverride string
	wait         bool
	dryRun       bool
	force        bool
}

func run(args []string) error {
	if len(args) == 0 {
		// 双击运行 / 无参数：直接打开图形界面
		if err := openNativeGUI(&cli{}); err != nil {
			fmt.Fprintln(os.Stderr, "启动图形界面失败:", err)
			printUsage()
		}
		return nil
	}
	cmd := args[0]
	rest := args[1:]

	c, pos, err := parseArgs(rest)
	if err != nil {
		return err
	}

	// 无需配置的命令
	switch cmd {
	case "config-init", "init":
		return cmdConfigInit(c)
	case "gui":
		return openNativeGUI(c)
	case "version", "-v", "--version":
		fmt.Println(version)
		return nil
	case "update":
		return cmdUpdate()
	case "help", "-h", "--help":
		printUsage()
		return nil
	}

	cfg, err := config.Load(c.configPath)
	if err != nil {
		return err
	}

	switch cmd {
	case "list", "scan":
		return cmdList(cfg, cmd == "scan")
	case "launch":
		if len(pos) < 1 {
			return fmt.Errorf("用法: launch <游戏id>")
		}
		return cmdLaunch(cfg, c, pos[0])
	case "install":
		if len(pos) < 1 {
			return fmt.Errorf("用法: install <游戏id>")
		}
		return cmdInstall(cfg, c, pos[0])
	case "uninstall":
		if len(pos) < 1 {
			return fmt.Errorf("用法: uninstall <游戏id>")
		}
		return cmdUninstall(cfg, c, pos[0])
	default:
		return fmt.Errorf("未知命令 %q\n\n%s", cmd, usageText)
	}
}

// gameSpec 为解析后的「待启动游戏」描述。
type gameSpec struct {
	ID              string
	Name            string
	ExePath         string
	WorkDir         string
	RootDir         string
	LauncherExe     string
	Args            []string
	LauncherArgs    []string
	Tags            []string
	Icon            string
	SteamAppID      string
	SteamInstallDir string
	SteamBuildID    string
	SteamDepotID    string
	SteamDepotManifest string
	SteamExeRel     string
	SteamSize       int64
}

func resolveSpec(cfg *config.Config, c *cli, id string) (*gameSpec, error) {
	p := games.ByName(id)
	gc, hasCfg := cfg.Games[id]
	if p == nil && !hasCfg {
		return nil, fmt.Errorf("未知游戏 id %q。用 `list` 查看内置游戏，或在配置文件中自定义", id)
	}

	spec := &gameSpec{ID: id}
	if p != nil {
		spec.Name = p.Name
		spec.LauncherExe = p.LauncherExe
		spec.Args = append([]string{}, p.DefaultArgs...)
		spec.LauncherArgs = append([]string{}, p.LauncherArgs...)
		spec.Tags = append([]string{}, p.Tags...)
		spec.SteamAppID = p.SteamAppID
		spec.SteamInstallDir = p.SteamInstallDir
		spec.SteamBuildID = p.SteamBuildID
		spec.SteamDepotID = p.SteamDepotID
		spec.SteamDepotManifest = p.SteamDepotManifest
		spec.SteamExeRel = p.SteamExeRel
		spec.SteamSize = p.SteamSize
	}
	if hasCfg {
		if gc.Name != "" {
			spec.Name = gc.Name
		}
		if gc.LauncherExe != "" {
			spec.LauncherExe = gc.LauncherExe
		}
		if len(gc.LaunchArgs) > 0 {
			spec.Args = append([]string{}, gc.LaunchArgs...)
		}
		if len(gc.LauncherArgs) > 0 {
			spec.LauncherArgs = append([]string{}, gc.LauncherArgs...)
		}
		if len(gc.Tags) > 0 {
			spec.Tags = append([]string{}, gc.Tags...)
		}
		if gc.Icon != "" {
			spec.Icon = gc.Icon
		}
		if gc.SteamAppID != "" {
			spec.SteamAppID = gc.SteamAppID
		}
		if gc.SteamInstallDir != "" {
			spec.SteamInstallDir = gc.SteamInstallDir
		}
		if gc.SteamBuildID != "" {
			spec.SteamBuildID = gc.SteamBuildID
		}
		if gc.SteamDepotManifest != "" {
			spec.SteamDepotManifest = gc.SteamDepotManifest
		}
	}

	if p == nil {
		// 自定义游戏：必须显式提供 path + exe
		if !hasCfg || gc.Path == "" || gc.Exe == "" {
			return nil, fmt.Errorf("自定义游戏 %q 需在配置中提供 path 与 exe", id)
		}
		exe := gc.Exe
		if !filepath.IsAbs(exe) {
			exe = filepath.Join(gc.Path, exe)
		}
		if _, err := os.Stat(exe); err != nil {
			return nil, fmt.Errorf("未找到可执行文件 %s", exe)
		}
		spec.ExePath = exe
		spec.RootDir = gc.Path
		spec.WorkDir = filepath.Dir(exe)
		if gc.WorkDir != "" {
			spec.WorkDir = gc.WorkDir
		}
	} else {
		overridePath := c.pathOverride
		overrideExe := ""
		if hasCfg {
			if overridePath == "" {
				overridePath = gc.Path
			}
			overrideExe = gc.Exe
		}
		res, err := detect.Locate(p, overridePath, overrideExe)
		if err != nil {
			return nil, err
		}
		spec.ExePath = res.ExePath
		spec.WorkDir = res.WorkDir
		spec.RootDir = res.RootDir
		if hasCfg && gc.WorkDir != "" {
			spec.WorkDir = gc.WorkDir
		}
	}

	if c.argsOverride != "" {
		spec.Args = strings.Fields(c.argsOverride)
	}
	return spec, nil
}

func cmdList(cfg *config.Config, scan bool) error {
	fmt.Println("内置支持的游戏：")
	fmt.Println()
	for _, p := range games.Builtin() {
		line := fmt.Sprintf("  %-20s %s", p.ID, p.Name)
		if scan {
			res, err := detect.Locate(p, "", "")
			if err == nil {
				line += fmt.Sprintf("\n    已定位: %s", res.ExePath)
			} else {
				line += "\n    未检测到安装"
			}
		}
		fmt.Println(line)
	}
	if len(cfg.Games) > 0 {
		fmt.Println()
		fmt.Println("配置文件中的自定义/覆盖游戏：")
		ids := make([]string, 0, len(cfg.Games))
		for id := range cfg.Games {
			ids = append(ids, id)
		}
		sort.Strings(ids)
		for _, id := range ids {
			gc := cfg.Games[id]
			fmt.Printf("  %-20s %s\n", id, gc.Name)
		}
	}
	return nil
}

func cmdLaunch(cfg *config.Config, c *cli, id string) error {
	spec, err := resolveSpec(cfg, c, id)
	if err != nil {
		return err
	}
	mode := c.mode
	if mode == "" {
		mode = cfg.LaunchMode
	}
	if mode == "" {
		mode = "direct"
	}

	switch mode {
	case "direct":
		o := launch.Options{ExePath: spec.ExePath, Args: spec.Args, WorkDir: spec.WorkDir, Wait: c.wait}
		fmt.Printf("启动: %s\n", launch.Describe(o))
		if c.dryRun {
			return nil
		}
		return launch.Run(o)
	case "launcher":
		launcherPath, err := resolveLauncher(spec)
		if err != nil {
			return err
		}
		o := launch.Options{ExePath: launcherPath, Args: spec.LauncherArgs, WorkDir: spec.RootDir, Wait: c.wait}
		fmt.Printf("经官方启动器启动: %s\n", launch.Describe(o))
		if c.dryRun {
			return nil
		}
		return launch.Run(o)
	case "steam":
		// 经 Steam 启动 = 确保已假入库（让 Steam 以为已下载），再用 steam://rungameid/<appid> 拉起国服
		if spec.SteamAppID == "" {
			return fmt.Errorf("游戏「%s」无对应 Steam 版本（未提供 appid），无法经 Steam 启动。可改用 direct/launcher 模式", spec.Name)
		}
		sp, err := steam.FindSteamPath(cfg.Steam.Path)
		if err != nil {
			return err
		}
		installdir := steamInstallDirOf(spec)
		if !steam.IsFakeInstalled(sp, spec.SteamAppID) {
			if c.dryRun {
				fmt.Printf("[dry-run] 将假入库「%s」（appid %s）\n", spec.Name, spec.SteamAppID)
			} else {
				contentDir := steamContentDir(spec)
				if _, err := steam.FakeInstall(sp, spec.SteamAppID, spec.Name, installdir, contentDir, steamDepotOf(spec)); err != nil {
					return fmt.Errorf("假入库失败: %w（可先运行 install %s）", err, spec.ID)
				}
				fmt.Printf("已自动假入库（重启 Steam 后生效）。\n")
			}
		}
		uri := steam.LaunchURI(spec.SteamAppID)
		fmt.Printf("经 Steam 启动: %s\n", uri)
		if c.dryRun {
			return nil
		}
		return launch.OpenURI(uri)
	default:
		return fmt.Errorf("未知启动方式 %q（可选 direct|launcher|steam）", mode)
	}
}

func cmdConfigInit(c *cli) error {
	path := c.configPath
	if path == "" {
		path = "config.yaml"
	}
	if err := config.SaveDefault(path); err != nil {
		return err
	}
	fmt.Printf("已生成示例配置: %s\n", path)
	return nil
}

// steamInstallDirOf 返回假入库用的库内目录名（未配置时用游戏根目录名兜底）。
func steamInstallDirOf(spec *gameSpec) string {
	if spec.SteamInstallDir != "" {
		return spec.SteamInstallDir
	}
	d := filepath.Base(spec.RootDir)
	if d == "" || d == "." || d == "\\" || d == "/" {
		d = strings.TrimSuffix(strings.TrimSuffix(filepath.Base(spec.ExePath), ".exe"), ".EXE")
	}
	if d == "" || d == "." || d == "\\" || d == "/" {
		d = strings.ReplaceAll(spec.Name, " ", "")
	}
	return d
}

// steamDepotOf 由 spec 组装 DepotInfo。
func steamDepotOf(spec *gameSpec) steam.DepotInfo {
	return steam.DepotInfo{
		BuildID:  spec.SteamBuildID,
		DepotID:  spec.SteamDepotID,
		Manifest: spec.SteamDepotManifest,
		Size:     spec.SteamSize,
	}
}

// steamContentDir 计算 junction 目标（游戏内容根目录）：从检测到的主程序绝对路径
// 中去掉 SteamExeRel（主程序相对内容根目录的路径）。
func steamContentDir(spec *gameSpec) string {
	if spec.SteamExeRel == "" {
		return spec.WorkDir
	}
	exe := filepath.ToSlash(spec.ExePath)
	rel := filepath.ToSlash(spec.SteamExeRel)
	if strings.HasSuffix(exe, rel) {
		return filepath.FromSlash(strings.TrimSuffix(exe, rel))
	}
	return spec.WorkDir
}

// cmdUpdate 检查并应用自更新。
func cmdUpdate() error {
	rel, err := update.LatestRelease()
	if err != nil {
		return err
	}
	if !update.IsNewer(version, rel.Tag) {
		fmt.Printf("✅ 已是最新版本（当前 %s，最新 %s）。\n", version, rel.Tag)
		return nil
	}
	fmt.Printf("发现新版本：%s（当前 %s）\n%s\n\n正在下载更新…\n", rel.Tag, version, rel.Name)
	tmp := filepath.Join(os.TempDir(), "steam-gacha-launcher-update.exe")
	if err := update.Download(rel.URL, tmp); err != nil {
		return err
	}
	if err := update.SelfReplace(tmp); err != nil {
		return err
	}
	fmt.Println("✅ 下载完成，程序将在数秒后自动替换并重启。")
	os.Exit(0)
	return nil
}

// cmdInstall 假入库：让游戏在 Steam 库中显示为已安装（伪造完整 appmanifest + 目录联接）。
func cmdInstall(cfg *config.Config, c *cli, id string) error {
	spec, err := resolveSpec(cfg, c, id)
	if err != nil {
		return err
	}
	if spec.SteamAppID == "" {
		return fmt.Errorf("游戏「%s」未提供 Steam appid（需在 Steam 有对应版本）。可在 config.yaml 为该游戏设置 steam_appid", spec.Name)
	}
	steamPath, err := steam.FindSteamPath(cfg.Steam.Path)
	if err != nil {
		return err
	}
	if steam.IsSteamRunning() && !c.force {
		fmt.Println("提示：Steam 正在运行。假入库后需完全重启 Steam，库才会刷新显示。")
	}
	installdir := steamInstallDirOf(spec)
	contentDir := steamContentDir(spec)
	// 幂等：若已假入库（清单或 junction 存在），先移除再重装，实现「一键刷新」
	if steam.IsFakeInstalled(steamPath, spec.SteamAppID) {
		fmt.Println("检测到已假入库，正在刷新为最新数据…")
		_ = steam.RemoveFakeInstall(steamPath, spec.SteamAppID, installdir)
	} else if _, err := os.Lstat(filepath.Join(steamPath, "steamapps", "common", installdir)); err == nil {
		_ = steam.RemoveFakeInstall(steamPath, spec.SteamAppID, installdir)
	}
	linkDir, err := steam.FakeInstall(steamPath, spec.SteamAppID, spec.Name, installdir, contentDir, steamDepotOf(spec))
	if err != nil {
		return err
	}
	fmt.Printf("✅ 假入库完成\n")
	fmt.Printf("  AppId:      %s (buildid %s)\n", spec.SteamAppID, spec.SteamBuildID)
	fmt.Printf("  库内目录:    %s\n", linkDir)
	fmt.Printf("  联接至(国服): %s\n", contentDir)
	fmt.Println("\n请重启 Steam，「" + spec.Name + "」将显示为已安装，可从库中启动国服。")
	fmt.Printf("经 Steam 启动: steam://rungameid/%s\n", spec.SteamAppID)
	return nil
}

// cmdUninstall 取消假入库（删除 appmanifest 与 junction，不影响游戏文件）。
func cmdUninstall(cfg *config.Config, c *cli, id string) error {
	spec, err := resolveSpec(cfg, c, id)
	if err != nil {
		return err
	}
	if spec.SteamAppID == "" {
		return fmt.Errorf("游戏「%s」未提供 Steam appid", spec.Name)
	}
	steamPath, err := steam.FindSteamPath(cfg.Steam.Path)
	if err != nil {
		return err
	}
	installdir := steamInstallDirOf(spec)
	if err := steam.RemoveFakeInstall(steamPath, spec.SteamAppID, installdir); err != nil {
		return err
	}
	fmt.Println("✅ 已取消假入库，请重启 Steam 生效。")
	return nil
}

func resolveLauncher(spec *gameSpec) (string, error) {
	if spec.LauncherExe == "" {
		return "", fmt.Errorf("该游戏未配置官方启动器（launcher_exe），无法使用 launcher 模式")
	}
	cands := []string{}
	if spec.RootDir != "" {
		cands = append(cands, filepath.Join(spec.RootDir, spec.LauncherExe))
	}
	cands = append(cands, filepath.Join(spec.WorkDir, spec.LauncherExe))
	for _, cand := range cands {
		if fi, err := os.Stat(cand); err == nil && !fi.IsDir() {
			return cand, nil
		}
	}
	return "", fmt.Errorf("未找到官方启动器 %s（已尝试 %v）", spec.LauncherExe, cands)
}

func printUsage() {
	fmt.Print(usageText)
}

const usageText = `steam-gacha-launcher —— 通过 Steam 启动国服二游

用法:
  steam-gacha-launcher <命令> [选项]

命令:
  (无参数)              双击运行：直接打开图形界面
  gui                    打开图形界面（原生 Win32 窗口）
  list                   列出内置支持的游戏
  scan                   列出内置游戏并检测本机安装位置
  launch <id>            启动游戏（默认 direct；steam 模式=假入库后经 Steam 启动国服）
  install <id>           假入库：一键让 Steam 以为游戏已下载（重复执行即刷新）
  uninstall <id>         取消假入库
  update                 检查并自更新到最新版本
  config-init            生成示例配置文件 config.yaml
  version                显示版本

选项:
  -config <path>        配置文件路径（默认 exe 旁 config.yaml）
  -mode <m>             启动方式: direct | launcher | steam
  -path <dir>           覆盖游戏安装路径
  -args "<...>"         覆盖启动参数
  -wait                 等待游戏进程结束
  -dry-run              只打印命令，不实际执行
  -force                忽略安全提示（如 Steam 正在运行）

示例:
  steam-gacha-launcher                      # 双击 / 无参数：直接打开图形界面
  steam-gacha-launcher scan
  steam-gacha-launcher launch yi-huan                    # 直接启动
  steam-gacha-launcher launch yi-huan -mode launcher     # 经官方启动器
  steam-gacha-launcher install yi-huan                   # 一键假入库（让 Steam 以为已下载）
  steam-gacha-launcher launch yi-huan -mode steam        # 经 Steam 启动国服（自动假入库）
  steam-gacha-launcher launch yi-huan -path "D:/NTE" -args "-platform_type CN"
  steam-gacha-launcher update                            # 检查自更新
`
