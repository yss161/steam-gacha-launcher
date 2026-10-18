package games

// init 注册内置游戏画像。
//
// 说明：国服（CN）客户端与海外客户端往往共用同一主程序名，区别在于：
//   - 主程序本身即为 CN 版（如原神国服 YuanShen.exe）；
//   - 或需要平台参数（如 -platform_type CN）由启动器/命令行区分大区。
//
// 这里的 DefaultArgs 均保持保守（多数国服客户端无需额外参数即可正常启动与登录），
// 用户可在 config.yaml 中用 launch_args 覆盖。登录流程统一为「游戏内登录」，即
// 直接拉起主程序后由游戏自身弹出登录界面。
func init() {
	register(&Profile{
		ID:   "genshin-cn",
		Name: "原神 (Genshin Impact 国服)",
		DefaultPaths: []string{
			`C:\Program Files\Genshin Impact`,
			`C:\Program Files (x86)\Genshin Impact`,
			`D:\Genshin Impact`,
		},
		ExeNames: []string{
			`Genshin Impact Game\YuanShen.exe`,
			`YuanShen.exe`,
		},
		LauncherExe: `launcher.exe`,
		Tags:        []string{"原神", "HoYoverse"},
		RegHints: []RegHint{
			{Root: "HKLM", Match: []string{"原神", "Genshin Impact"}, Value: "InstallLocation"},
		},
	})

	register(&Profile{
		ID:   "hsr-cn",
		Name: "崩坏：星穹铁道 (Honkai: Star Rail 国服)",
		DefaultPaths: []string{
			`C:\Program Files\Star Rail`,
			`C:\Program Files (x86)\Star Rail`,
			`D:\Star Rail`,
		},
		ExeNames: []string{
			`Games\StarRail.exe`,
			`StarRail.exe`,
		},
		LauncherExe: `launcher.exe`,
		Tags:        []string{"星穹铁道", "HoYoverse"},
		RegHints: []RegHint{
			{Root: "HKLM", Match: []string{"崩坏", "星穹铁道", "Star Rail"}, Value: "InstallLocation"},
		},
	})

	register(&Profile{
		ID:   "zzz-cn",
		Name: "绝区零 (Zenless Zone Zero 国服)",
		DefaultPaths: []string{
			`C:\Program Files\Zenless Zone Zero`,
			`C:\Program Files (x86)\Zenless Zone Zero`,
			`D:\Zenless Zone Zero`,
		},
		ExeNames: []string{
			`Games\ZenlessZoneZero.exe`,
			`ZenlessZoneZero.exe`,
		},
		LauncherExe: `launcher.exe`,
		Tags:        []string{"绝区零", "HoYoverse"},
		RegHints: []RegHint{
			{Root: "HKLM", Match: []string{"绝区零", "Zenless Zone Zero"}, Value: "InstallLocation"},
		},
		SteamAppID:      "4162040",
		SteamInstallDir: "ZenlessZoneZero",
	})

	register(&Profile{
		ID:   "wuthering-waves-cn",
		Name: "鸣潮 (Wuthering Waves 国服)",
		DefaultPaths: []string{
			`C:\Program Files\Wuthering Waves`,
			`C:\Program Files (x86)\Wuthering Waves`,
			`D:\Wuthering Waves`,
		},
		ExeNames: []string{
			`Wuthering Waves Game\Client\Binaries\Win64\Client-Win64-Shipping.exe`,
			`Client-Win64-Shipping.exe`,
		},
		LauncherExe: `launcher.exe`,
		Tags:        []string{"鸣潮", "库洛"},
		RegHints: []RegHint{
			{Root: "HKLM", Match: []string{"鸣潮", "Wuthering Waves"}, Value: "InstallLocation"},
		},
		SteamAppID:      "3513350",
		SteamInstallDir: "Wuthering Waves",
	})

	register(&Profile{
		ID:   "endfield-cn",
		Name: "明日方舟：终末地 (Arknights: Endfield 国服)",
		DefaultPaths: []string{
			`C:\Program Files\GRYPHLINE`,
			`C:\Program Files (x86)\GRYPHLINE`,
			`D:\GRYPHLINE`,
			`C:\Program Files\Arknights Endfield`,
		},
		ExeNames: []string{
			`Endfield.exe`,
			`Game\Endfield.exe`,
		},
		LauncherExe: `Launcher.exe`,
		Tags:        []string{"终末地", "鹰角"},
		RegHints: []RegHint{
			{Root: "HKLM", Match: []string{"终末地", "Endfield"}, Value: "InstallLocation"},
		},
		SteamAppID:      "4732690",
		SteamInstallDir: "Arknights Endfield",
	})
}
