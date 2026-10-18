package games

// init 注册内置游戏画像。
//
// 说明：国服（CN）客户端与海外客户端往往共用同一主程序名，区别在于：
//   - 主程序本身即为 CN 版；
//   - 或需要平台参数（如 -platform_type CN）由启动器/命令行区分大区。
//
// 这里的 DefaultArgs 均保持保守（多数国服客户端无需额外参数即可正常启动与登录），
// 用户可在 config.yaml 中用 launch_args 覆盖。登录流程统一为「游戏内登录」，即
// 直接拉起主程序后由游戏自身弹出登录界面。
func init() {
	register(&Profile{
		ID:   "yi-huan",
		Name: "异环 (Neverness to Everness 国服)",
		DefaultPaths: []string{
			`C:\Program Files\Neverness to Everness`,
			`C:\Program Files (x86)\Neverness to Everness`,
			`D:\Neverness to Everness`,
			`C:\Perfect World\Neverness to Everness`,
			`C:\Program Files\NTE`,
			`D:\NTE`,
		},
		// 虚幻引擎客户端：常见主程序名（含递归搜索兜底）
		ExeNames: []string{
			`NTE.exe`,
			`NTE\Binaries\Win64\NTE-Win64-Shipping.exe`,
			`Neverness to Everness.exe`,
			`NTE-Win64-Shipping.exe`,
			`Client-Win64-Shipping.exe`,
		},
		LauncherExe: `launcher.exe`,
		Tags:        []string{"异环", "完美世界"},
		RegHints: []RegHint{
			{Root: "HKLM", Match: []string{"异环", "Neverness to Everness", "NTE"}, Value: "InstallLocation"},
		},
		SteamAppID:      "4508340",
		SteamInstallDir: "Neverness to Everness",
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
