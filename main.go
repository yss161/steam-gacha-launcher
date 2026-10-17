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
)

var version = "dev"

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "\n[错误] %v\n", err)
		os.Exit(1)
	}
}

// parseArgs 解析命令行参数，支持「位置参数与选项穿插」的写法（如 launch genshin-cn -path ...）。
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
	case "web", "gui":
		if cmd == "gui" {
			return openNativeGUI(c)
		}
		return cmdWeb(c)
	case "version", "-v", "--version":
		fmt.Println(version)
		return nil
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
	case "steam-add", "add":
		if len(pos) < 1 {
			return fmt.Errorf("用法: steam-add <游戏id>")
		}
		return cmdSteamAdd(cfg, c, pos[0])
	case "steam-uri", "uri":
		if len(pos) < 1 {
			return fmt.Errorf("用法: steam-uri <游戏id>")
		}
		return cmdSteamURI(cfg, c, pos[0])
	case "remove":
		if len(pos) < 1 {
			return fmt.Errorf("用法: remove <游戏id>")
		}
		return cmdRemove(cfg, c, pos[0])
	default:
		return fmt.Errorf("未知命令 %q\n\n%s", cmd, usageText)
	}
}

// gameSpec 为解析后的「待启动游戏」描述。
type gameSpec struct {
	ID           string
	Name         string
	ExePath      string
	WorkDir      string
	RootDir      string
	LauncherExe  string
	Args         []string
	LauncherArgs []string
	Tags         []string
	Icon         string
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
		appID, err := ensureShortcut(cfg, c, spec)
		if err != nil {
			return err
		}
		uri := steam.LaunchURI(appID)
		fmt.Printf("经 Steam 启动: %s\n", uri)
		if c.dryRun {
			return nil
		}
		return launch.OpenURI(uri)
	default:
		return fmt.Errorf("未知启动方式 %q（可选 direct|launcher|steam）", mode)
	}
}

func cmdSteamAdd(cfg *config.Config, c *cli, id string) error {
	spec, err := resolveSpec(cfg, c, id)
	if err != nil {
		return err
	}
	appID, err := ensureShortcut(cfg, c, spec)
	if err != nil {
		return err
	}
	fmt.Println()
	fmt.Printf("已加入 Steam 库。请重启 Steam 后在库中找到「%s」。\n", spec.Name)
	fmt.Printf("  AppId: %d\n", uint32(appID))
	fmt.Printf("  URI:   %s\n", steam.LaunchURI(appID))
	fmt.Printf("  封面用长 AppId: %d\n", steam.GenerateLongAppId(spec.ExePath, spec.Name))
	return nil
}

func cmdSteamURI(cfg *config.Config, c *cli, id string) error {
	spec, err := resolveSpec(cfg, c, id)
	if err != nil {
		return err
	}
	appID := steam.GenerateAppId(spec.ExePath, spec.Name)
	fmt.Printf("游戏: %s\n", spec.Name)
	fmt.Printf("Exe:  %s\n", spec.ExePath)
	fmt.Println()
	fmt.Printf("启动 URI:  %s\n", steam.LaunchURI(appID))
	fmt.Printf("AppId:     %d\n", uint32(appID))
	fmt.Printf("长 AppId:  %d\n", steam.GenerateLongAppId(spec.ExePath, spec.Name))
	fmt.Println()
	fmt.Println("提示：只有在 `steam-add` 将游戏加入 Steam 库后，上述 URI 才能启动。")
	return nil
}

func cmdRemove(cfg *config.Config, c *cli, id string) error {
	spec, err := resolveSpec(cfg, c, id)
	if err != nil {
		return err
	}
	if !c.force && steam.IsSteamRunning() {
		return fmt.Errorf("Steam 正在运行，请先完全退出 Steam（可用 -force 忽略）")
	}
	steamPath, err := steam.FindSteamPath(cfg.Steam.Path)
	if err != nil {
		return err
	}
	ud, err := steam.FindUserDataDir(steamPath, cfg.Steam.AccountID)
	if err != nil {
		return err
	}
	vdf := steam.ShortcutsPath(ud)
	ok, err := steam.RemoveShortcut(vdf, spec.ExePath)
	if err != nil {
		return err
	}
	if !ok {
		fmt.Println("未在 Steam 库中找到该游戏。")
		return nil
	}
	fmt.Println("已从 Steam 库移除该游戏，请重启 Steam 生效。")
	return nil
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

func ensureShortcut(cfg *config.Config, c *cli, spec *gameSpec) (uint32, error) {
	steamPath, err := steam.FindSteamPath(cfg.Steam.Path)
	if err != nil {
		return 0, err
	}
	ud, err := steam.FindUserDataDir(steamPath, cfg.Steam.AccountID)
	if err != nil {
		return 0, err
	}
	vdf := steam.ShortcutsPath(ud)
	launchOptions := strings.Join(spec.Args, " ")
	fmt.Printf("Steam 目录: %s\n", steamPath)
	fmt.Printf("shortcuts.vdf: %s\n", vdf)
	fmt.Printf("写入快捷方式: %s → %s\n", spec.Name, spec.ExePath)
	if c.dryRun {
		fmt.Println("[dry-run] 未实际写入")
		return steam.GenerateAppId(spec.ExePath, spec.Name), nil
	}
	if !c.force && steam.IsSteamRunning() {
		return 0, fmt.Errorf("Steam 正在运行，直接写入会被覆盖。请先完全退出 Steam（可用 -force 忽略，或用 `steam-uri` 仅生成 URI）")
	}
	return steam.AddShortcut(vdf, spec.ExePath, spec.WorkDir, spec.Name, launchOptions, spec.Tags, spec.Icon)
}

func printUsage() {
	fmt.Print(usageText)
}

const usageText = `steam-gacha-launcher —— 通过 Steam 启动国服二游

用法:
  steam-gacha-launcher <命令> [选项]

命令:
  (无参数)              双击运行：直接打开图形界面
  gui                    打开图形界面（原生窗口）
  web                    以浏览器方式打开图形界面
  list                   列出内置支持的游戏
  scan                   列出内置游戏并检测本机安装位置
  launch <id>            启动游戏（默认 direct 模式）
  steam-add <id>         将游戏加入 Steam 库（非 Steam 游戏快捷方式）
  steam-uri <id>         仅输出 steam:// 启动 URI 与 appid
  remove <id>            从 Steam 库移除该游戏
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
  steam-gacha-launcher launch genshin-cn
  steam-gacha-launcher launch genshin-cn -mode launcher
  steam-gacha-launcher launch hsr-cn -mode steam
  steam-gacha-launcher steam-add zzz-cn
  steam-gacha-launcher launch genshin-cn -path "D:/Games/Genshin Impact" -args "-platform_type CN"
`
