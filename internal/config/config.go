// Package config 负责加载与生成配置文件。
package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// SteamConfig 描述 Steam 相关配置。
type SteamConfig struct {
	// Path 为 Steam 安装目录，留空则自动检测（注册表 + 常见路径）。
	Path string `yaml:"path"`
	// AccountID 为 Steam 用户账号目录名（userdata/<AccountID>），留空则自动选择最近使用的账号。
	AccountID string `yaml:"account_id"`
}

// GameConfig 描述单个游戏的自定义配置。既可覆盖内置游戏，也可新增自定义游戏。
type GameConfig struct {
	Name         string   `yaml:"name"`          // 显示名称（用于 Steam 库名称）
	Path         string   `yaml:"path"`          // 游戏安装根目录；内置游戏留空则自动检测
	Exe          string   `yaml:"exe"`           // 游戏主程序，相对 path 或绝对路径（内置游戏留空自动检测）
	LauncherExe  string   `yaml:"launcher_exe"`  // 官方启动器（用于 launcher 模式）
	LaunchArgs   []string `yaml:"launch_args"`   // 游戏启动参数（覆盖内置默认）
	LauncherArgs []string `yaml:"launcher_args"` // 启动器参数
	WorkDir      string   `yaml:"work_dir"`      // 工作目录，留空 = 主程序所在目录
	Tags         []string `yaml:"tags"`          // Steam 分类标签
	Icon         string   `yaml:"icon"`          // Steam 图标路径（.ico/.exe），留空用主程序

	// 假入库（让游戏在 Steam 库中显示为已安装）
	SteamAppID      string `yaml:"steam_appid"`       // 该游戏的 Steam appid
	SteamInstallDir string `yaml:"steam_installdir"`  // steamapps/common 下的目录名
}

// Config 为顶层配置结构。
type Config struct {
	// LaunchMode 默认启动方式：direct（直接启动游戏）/ launcher（经官方启动器）/ steam（经 Steam 快捷方式）。
	LaunchMode string                `yaml:"launch_mode"`
	Steam      SteamConfig           `yaml:"steam"`
	Games      map[string]GameConfig `yaml:"games"`
}

// Default 返回默认配置。
func Default() *Config {
	return &Config{
		LaunchMode: "direct",
		Steam:      SteamConfig{},
		Games:      map[string]GameConfig{},
	}
}

// Load 从指定路径（或默认位置）加载配置。路径为空时尝试可执行文件旁的 config.yaml / config.yml。
func Load(path string) (*Config, error) {
	cfg := Default()
	if path == "" {
		path = findDefault()
	}
	if path == "" {
		return cfg, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("读取配置失败: %w", err)
	}
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("解析配置失败: %w", err)
	}
	return cfg, nil
}

func findDefault() string {
	exe, err := os.Executable()
	if err != nil {
		return ""
	}
	dir := filepath.Dir(exe)
	for _, name := range []string{"config.yaml", "config.yml"} {
		p := filepath.Join(dir, name)
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return ""
}

// SaveDefault 生成一份带注释的示例配置到指定路径。
func SaveDefault(path string) error {
	return os.WriteFile(path, []byte(ExampleYAML), 0o644)
}

// Save 将配置序列化为 YAML 并写入 path。
func Save(path string, cfg *Config) error {
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("序列化配置失败: %w", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("写入配置失败: %w", err)
	}
	return nil
}

// ExampleYAML 为示例配置模板。
const ExampleYAML = `# Steam 国服二游启动器 配置文件
# 说明见 README.md

# 默认启动方式: direct(直接启动游戏) | launcher(经官方启动器) | steam(经 Steam 快捷方式)
launch_mode: direct

steam:
  path: ""          # Steam 安装目录，留空自动检测
  account_id: ""    # userdata 下的账号目录名，留空自动选择最近使用的账号

games:
  # 可覆盖内置游戏（键名与内置 id 一致），也可新增自定义游戏。
  # 内置 id 见: steam-gacha-launcher list
  #
  # 覆盖示例（原神，手动指定路径与参数）:
  # genshin-cn:
  #   path: "D:/Games/Genshin Impact"
  #   launch_args: ["-platform_type", "CN"]
  #   tags: ["原神", "二次元"]
  #
  # 自定义游戏示例:
  # my-game:
  #   name: "我的自定义游戏"
  #   path: "D:/Games/MyGame"
  #   exe: "MyGame.exe"
  #   launch_args: ["-cn"]
  #   launcher_exe: "Launcher.exe"
`
