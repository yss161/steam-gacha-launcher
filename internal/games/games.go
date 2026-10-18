// Package games 定义游戏画像（Profile）与内置游戏列表。
package games

// RegHint 描述一条注册表探测线索。
type RegHint struct {
	// Root 为注册表根：HKLM 或 HKCU。
	Root string
	// Path 为要枚举的键路径（通常是 Uninstall 键），留空使用默认 Uninstall 路径。
	Path string
	// Match 为 DisplayName 的子串匹配列表（命中其一即可）。
	Match []string
	// Value 为取值字段，默认 InstallLocation。
	Value string
}

// Profile 描述一个内置游戏画像。
type Profile struct {
	ID           string
	Name         string
	DefaultPaths []string // 常见安装根目录
	ExeNames     []string // 主程序相对路径候选（相对安装根目录）
	LauncherExe  string   // 官方启动器文件名
	DefaultArgs  []string
	LauncherArgs []string
	Tags         []string
	RegHints     []RegHint
	// 假入库（Steam 库显示为已安装）相关
	SteamAppID         string // 该游戏在 Steam 的真实 appid（无 Steam 版本则为空）
	SteamInstallDir    string // steamapps/common 下的目录名
	SteamBuildID       string // 当前公开分支 buildid（随更新变化）
	SteamDepotID       string // 主 depot id
	SteamDepotManifest string // depot 当前 manifest GID
	SteamExeRel        string // 主程序相对游戏内容根目录的路径（用于定位 junction 目标）
	SteamSize          int64  // depot 磁盘大小（字节）
}

var builtin []*Profile

func register(p *Profile) {
	builtin = append(builtin, p)
}

// Builtin 返回全部内置游戏画像。
func Builtin() []*Profile {
	return builtin
}

// ByName 按 id 返回内置画像，未找到返回 nil。
func ByName(id string) *Profile {
	for _, p := range builtin {
		if p.ID == id {
			return p
		}
	}
	return nil
}
