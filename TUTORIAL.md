# Steam 国服二游启动器 · 使用教程

> 通过 Steam 启动国服二次元游戏的原生 Windows 工具。支持**检测定位**、**直接启动**、**经官方启动器启动**、**经 Steam 启动**、**假入库（让游戏在 Steam 显示为已下载安装）**、**自更新**。
> 本工具为纯 Go 编译的原生 Win32 程序，单文件、无运行时依赖、不含任何浏览器组件。

---

## 目录

1. [下载与安装](#一下载与安装)
2. [快速上手（图形界面）](#二快速上手图形界面)
3. [「假入库」详解（重点）](#三假入库详解重点)
4. [命令行（CLI）用法](#四命令行cli用法)
5. [配置文件](#五配置文件)
6. [自更新](#六自更新)
7. [常见问题（FAQ）](#七常见问题faq)
8. [内置游戏与 Steam appid](#八内置游戏与-steam-appid)

---

## 一、下载与安装

### 方式 A：Release 直装（推荐）

1. 打开 [Releases](https://github.com/yss161/steam-gacha-launcher/releases) 页面；
2. 下载最新版的 `steam-gacha-launcher.exe`；
3. 放到任意目录（建议单独一个文件夹），**双击即可运行**，无需安装。

> 首次双击会直接进入图形界面（无黑色控制台窗口）。

### 方式 B：源码构建

```bash
# 需要 Go ≥ 1.22
go run github.com/akavel/rsrc@latest -manifest app.manifest -o rsrc.syso
go build -trimpath -ldflags "-s -w -H windowsgui" -o steam-gacha-launcher.exe .
```

---

## 二、快速上手（图形界面）

双击 exe 打开主窗口：

```
菜单栏：文件 / 帮助
工具栏：直接启动 | 经启动器 | 经 Steam | 加入 Steam | 移除
左表格：游戏 | 状态 | 主程序路径          右「详情」面板：信息与操作按钮
状态栏：操作反馈（左） + Steam 状态（右）
```

**典型操作：**

| 想做什么 | 怎么做 |
|----------|--------|
| 看本机装了哪些游戏 | 打开即自动扫描；点「重新扫描」或按 `Ctrl+R` 刷新 |
| 直接启动游戏 | 选中游戏 → 工具栏「直接启动」（不经 Steam，无覆盖层） |
| 经官方启动器启动 | 选中游戏 → 工具栏「经启动器」（需要启动器令牌/校验的游戏） |
| 经 Steam 启动（快捷方式方式） | 选中游戏 → 工具栏「经 Steam」 |
| 让游戏在 Steam 显示为「已安装」 | 选中游戏 → 详情面板「一键假入库 / 刷新」（见下节） |
| 改安装路径 / 启动参数 | **双击游戏行** 或点「编辑路径/参数」，保存后自动重扫 |
| 修改 Steam 目录 / 账号 | 详情面板「设置」 |
| 高级编辑 | 详情面板「打开配置文件」（用记事本打开 config.yaml） |

**交互细节：** 表格行右键有快捷菜单；状态列绿=已定位、灰=未检测到；点「帮助 → 检查更新」可自更新。

---

## 三、「假入库」详解（重点）

### 它是什么

普通「非 Steam 快捷方式」在 Steam 库里显示为**外部游戏**。「假入库」则让游戏**像真正从 Steam 下载安装的一样**：出现在「已安装」分类、带绿色对勾、走 Steam 的启动管线（覆盖层 / F12 截图 / 手柄 / 时长统计 / “正在玩”状态）。

### 原理（纯配置操作，无注入、不破解）

1. Steam 客户端启动时扫描 `Steam/steamapps/appmanifest_<appid>.acf`，据此决定库里显示哪些游戏、是否已安装；
2. 本工具生成**完整清单**：`StateFlags=4`（已完全安装）+ **真实 buildid + InstalledDepots**（depot 及 manifest GID）+ `LastOwner`=你的 SteamID64。Steam 据此认为「该版本内容已安装」，不再触发下载；
3. 同时创建**目录联接（junction）**，把 `steamapps/common/<目录名>` 指向你的**国服安装目录**，Steam 启动时按其启动配置在联接目录里找到主程序并拉起（即「用 Steam 启动国服」）。

> 全程不修改游戏文件、不注入进程、不绕过任何授权或反作弊。仅写入一个清单文件 + 一个目录联接。

### 操作步骤（以绝区零为例）

1. 确保游戏已通过官方启动器**安装在电脑上**（本工具会自动定位，定位不到可双击行手动填路径）；
2. 选中游戏 → 详情面板点 **「一键假入库 / 刷新」**（或命令行 `install zzz-cn`）；
3. 提示成功后，**完全重启 Steam**；
4. 库里该游戏显示为「已安装」，点「开始」即可由 Steam 拉起国服客户端。

### 一键刷新（幂等）

- 该按钮可**重复点**：已入库时会自动用最新内置数据**刷新**清单与联接（游戏更新后点一下即可，无需先取消）。
- 游戏大更新后若 Steam 提示更新，先运行「帮助 → 检查更新」把工具升到最新版（新版自带最新 buildid 数据），再点「一键假入库 / 刷新」。

### 取消

点「取消假入库」（或 `uninstall zzz-cn`）→ 重启 Steam。只删除清单与联接，**不影响游戏文件**。

### 注意事项

- **appid 必须是该游戏真实的 Steam appid**（已内置，见第八节）。没有 Steam 版本的游戏无法被 Steam 拉起。
- 目录联接是「指针」，不复制游戏数据，**不占用额外磁盘**。
- 若你手动把游戏文件移动/重命名，需重新「一键假入库」。

---

## 四、命令行（CLI）用法

无参数双击 = 打开图形界面。也可在终端用子命令：

```
steam-gacha-launcher <命令> [选项]

命令:
  (无参数)        双击运行：直接打开图形界面
  gui             打开图形界面（原生 Win32 窗口）
  list            列出内置支持的游戏
  scan            列出内置游戏并检测本机安装位置
  launch <id>     启动游戏（默认 direct）
  steam-add <id>  将游戏加入 Steam 库（非 Steam 快捷方式）
  install <id>    假入库（一键；重复执行即刷新为最新数据）
  uninstall <id>  取消假入库
  update          检查并自更新到最新版本
  steam-uri <id>  输出 steam:// 启动 URI 与 appid
  remove <id>     从 Steam 库移除（非 Steam 快捷方式）
  config-init     生成示例配置 config.yaml
  version         显示版本

选项:
  -config <path>  配置文件路径（默认 exe 旁 config.yaml）
  -mode <m>       启动方式: direct | launcher | steam
  -path <dir>     覆盖游戏安装路径
  -args "<...>"   覆盖启动参数
  -wait           等待游戏进程结束
  -dry-run        只打印命令，不实际执行
  -force          忽略安全提示
```

**示例：**

```bash
steam-gacha-launcher scan                              # 检测本机游戏
steam-gacha-launcher launch yi-huan                    # 直接启动异环
steam-gacha-launcher launch yi-huan -mode launcher     # 经官方启动器
steam-gacha-launcher install yi-huan                   # 一键假入库异环
steam-gacha-launcher launch yi-huan -mode steam        # 经 Steam（快捷方式）启动
steam-gacha-launcher launch yi-huan -path "D:/NTE" -args "-platform_type CN"
steam-gacha-launcher update                            # 检查自更新
```

---

## 五、配置文件

首次运行会在 exe 旁生成/使用 `config.yaml`。`config-init` 生成带注释模板。

```yaml
launch_mode: direct          # 默认启动方式: direct | launcher | steam

steam:
  path: ""                   # Steam 目录，留空自动检测
  account_id: ""             # userdata 账号目录，留空自动选最近使用的账号

games:
  # 覆盖内置游戏（键名 = 内置 id）
  yi-huan:
    path: "D:/NTE"
    launch_args: ["-platform_type", "CN"]
    tags: ["异环"]
    # 假入库相关（一般无需改，游戏更新导致提示更新时才需更新下面两项）
    # steam_appid: "4508340"
    # steam_buildid: "25643250"
    # steam_depot_manifest: "7894482336424447226"

  # 新增自定义游戏
  my-game:
    name: "我的自定义游戏"
    path: "D:/Games/MyGame"
    exe: "MyGame.exe"
    launcher_exe: "Launcher.exe"
    steam_appid: "1234560"   # 若该游戏有 Steam 版本，可假入库
```

| 字段 | 说明 |
|------|------|
| `launch_mode` | 默认启动方式 |
| `steam.path` / `steam.account_id` | Steam 目录 / 账号（留空自动） |
| `games.<id>.path` / `exe` / `work_dir` | 游戏路径 / 主程序 / 工作目录 |
| `games.<id>.launch_args` | 启动参数 |
| `games.<id>.launcher_exe` / `launcher_args` | 官方启动器 |
| `games.<id>.tags` | Steam 分类标签 |
| `games.<id>.steam_appid` / `steam_installdir` | 假入库 appid / 库内目录名 |
| `games.<id>.steam_buildid` / `steam_depot_manifest` | 假入库清单数据（游戏更新后更新） |

---

## 六、自更新

- 图形界面：菜单 **帮助 → 检查更新**。发现新版本会提示下载，确认后自动替换并重启。
- 命令行：`steam-gacha-launcher update`。

> 每次发布新版都会内置各游戏**最新的 buildid / depot 清单**数据，因此「自更新 + 一键假入库/刷新」即可应对游戏版本更新。

---

## 七、常见问题（FAQ）

**Q1：双击没反应 / 想看到报错？**
程序是 GUI 子系统（无控制台）。如需命令行输出，请在 `cmd` / PowerShell 里运行 `steam-gacha-launcher.exe <命令>`。

**Q2：游戏检测不到？**
点「重新扫描」；仍不行则**双击该行**，在弹窗里手动填写「安装路径」（主程序所在目录）保存。若是新游戏，可发我它的主程序名/安装路径，我补进内置画像。

**Q3：「经 Steam 启动」和「假入库启动」有什么区别？**
- 经 Steam（快捷方式）：把游戏加入 Steam「非 Steam 游戏」库，能覆盖层/手柄，但库里显示为外部游戏。
- 假入库启动：游戏显示为**真正的 Steam 已安装游戏**，体验最接近原生（含“正在玩”状态、库内封面等）。

**Q4：假入库后 Steam 仍提示下载/更新？**
通常是游戏刚更新、内置 buildid 过期。步骤：①「帮助 → 检查更新」升级工具；②再点「一键假入库 / 刷新」。仍不行则到 [SteamDB](https://steamdb.info) 查该游戏最新 `buildid` 填进 config 后刷新。

**Q5：假入库会影响游戏文件 / 账号安全吗？**
不会。只写一个清单文件 + 一个目录联接，不碰游戏文件；不涉及账号密码，不绕过反作弊与授权。

**Q6：如何彻底卸载本工具？**
直接删除 exe 与 config.yaml 即可；若做过假入库，先对每个游戏「取消假入库」并重启 Steam。

---

## 八、内置游戏与 Steam appid

| id | 名称 | Steam appid | 主程序 |
|----|------|-------------|--------|
| `yi-huan` | 异环 (Neverness to Everness) | `4508340` | `NTELauncher\NTEGame.exe` |
| `zzz-cn` | 绝区零 (Zenless Zone Zero) | `4162040` | `ZenlessZoneZero.exe` |
| `wuthering-waves-cn` | 鸣潮 (Wuthering Waves) | `3513350` | `Client-Win64-Shipping.exe` |
| `endfield-cn` | 明日方舟：终末地 (Arknights: Endfield) | `4732690` | `Endfield.exe` |

> 新增游戏：在 `config.yaml` 的 `games:` 下加一条即可（见第五节）。欢迎提 Issue/PR 补充更多国服二游画像。

---

## 免责说明

本工具为独立非官方项目，与任何游戏厂商、发行商及 Valve 均无关联。仅供学习与技术研究，请遵守各游戏服务条款；因违反条款产生的风险由使用者自行承担。
