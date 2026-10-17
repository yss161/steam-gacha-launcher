# Steam 国服二游启动器（steam-gacha-launcher）

> 一个用于**通过 Steam 启动国服二次元游戏（二游）**的 Windows 命令行小工具。
> 参考 [Better-Endfield](https://github.com/Dr-hydra/Better-Endfield) 的 Steam 启动思路，但**只做「启动器」、不做任何注入/魔改**。

## 为什么需要它

国服二游（原神、星穹铁道、绝区零、鸣潮、终末地等）通常只能从各自的官方启动器启动，无法直接享受 Steam 的能力：

- 🎮 **Steam 覆盖层 / F12 截图 / Shift+Tab**
- 🕹️ **Steam Input 手柄映射**（含陀螺仪、背键、摇杆曲线）
- 📺 **大屏模式（Big Picture）/ Steam Deck 串流**
- ⏱️ **游戏时长统计与成就墙**
- 📡 **Steam Remote Play 远程游玩**

本工具把「定位客户端 → 注册为 Steam 非 Steam 游戏 → 通过 Steam 拉起」整条链路自动化，同时也能**直接启动游戏**或**经官方启动器启动**。

---

## 核心功能

### 1. 检测并定位游戏客户端

对每个内置游戏维护一份「画像」（Profile），按以下顺序探测安装位置：

1. **注册表 Uninstall 键**：枚举 `HKLM`（含 `WOW6432Node` 32 位视图）与 `HKCU` 下的卸载项，按 `DisplayName` 子串匹配（如「原神」「鸣潮」），读取 `InstallLocation`，必要时从 `DisplayIcon` 解析路径；
2. **常见安装目录**：逐个尝试默认路径；
3. **有限深度目录搜索**：在候选根目录下按主程序文件名做最多 5 层的递归搜索（大小写不敏感），兼容 HoYoPlay 等聚合启动器的深层安装布局。

也可通过 `-path` 或配置文件 `games.<id>.path` 手动指定，跳过自动检测。

```bash
steam-gacha-launcher scan   # 列出内置游戏并检测本机安装
```

### 2. 通过 Steam 启动

两种互斥、可叠加的方式：

- **快捷方式（shortcuts.vdf）**：把游戏主程序写入 Steam 的非 Steam 游戏库（`userdata/<账号>/config/shortcuts.vdf`），`steam-add <id>`。写入前自动备份为 `.bak`、去重、并在 Steam 运行时给出安全提示。
- **启动参数 / URI**：`steam-uri <id>` 输出 `steam://rungameid/<appid>`；`launch <id> -mode steam` 一键「确保已入库 + 通过 Steam 拉起」。

> **appid 算法**与 Steam ROM Manager 一致：`crc32(exe + 名称) | 0x80000000`（写入 vdf 为 int32 小端；封面用长 id 为 `(id << 32) | 0x02000000`）。

### 3. 国服特有的启动参数与登录流程

- 国服/海外服往往**共用同一主程序名**，区别在于大区参数（如 `-platform_type CN`）或主程序本身就是 CN 版（如原神国服 `YuanShen.exe`）。内置画像为保守默认，用户可用 `launch_args` 覆盖。
- **登录流程统一为「游戏内登录」**：直接拉起主程序，由游戏自身弹出登录界面，不涉及账号/令牌透传。
- 若某游戏要求**必须经官方启动器**（例如启动器下发临时令牌/校验），使用 `launch <id> -mode launcher` 走官方启动器。

### 4. 反作弊 / 完整性校验 —— 兼容而非绕过

> ⚠️ 重要声明：本工具**不注入进程、不修改游戏文件、不绕过任何反作弊或完整性校验**。

- 内核级反作弊（如腾讯 ACE、ACE Expert 等）能正常通过的关键在于：**我们以与官方启动器完全相同的方式拉起主程序**——同一个 exe、同一个工作目录、同一套启动参数、继承同一环境变量。对反作弊而言这是一次「合法启动」。
- 直接启动为何可行：多数国服二游的主程序自身即完成登录与反作弊加载，官方启动器只是「下载器 + 拉起器」；只要不篡改 `GameAssembly` / 资源文件，完整性校验自然通过。
- 需注意：部分内核反作弊会**按设计禁用 Steam 覆盖层注入**，这属于游戏方策略，本工具不会去「对抗」它。若覆盖层不可用，改用 Steam 手柄/截图请以游戏实际表现为准。

### 5. 清晰的配置方式

- **配置文件**（YAML，`config.yaml`）：`launch_mode`、`steam.path`、`steam.account_id`、`games.<id>`（覆盖内置 + 自定义游戏）。
- **命令行参数**：`-config` `-mode` `-path` `-args` `-wait` `-dry-run` `-force`。
- `config-init` 一键生成带注释的示例配置。

---

## 安装与构建

### 从 Release 下载（推荐）

在 [Releases](https://github.com/yss161/steam-gacha-launcher/releases) 下载 `steam-gacha-launcher-windows-amd64.exe`。

### 从源码构建（Go ≥ 1.22）

```bash
go build -trimpath -ldflags "-s -w" -o steam-gacha-launcher.exe .
```

### 云端编译（GitHub Actions）

本项目自带 `.github/workflows/build.yml`：推送或打 tag `v*` 即自动在云端构建 Windows amd64/arm64/386 三架构产物，tag 推送还会自动发布 Release。手动触发：仓库 **Actions → build → Run workflow**。

---

## 使用示例

```bash
# 1. 查看支持的游戏 + 检测本机安装
steam-gacha-launcher scan

# 2. 直接启动（不经过 Steam，无覆盖层）
steam-gacha-launcher launch genshin-cn

# 3. 经官方启动器启动（需要启动器令牌的游戏）
steam-gacha-launcher launch wuthering-waves-cn -mode launcher

# 4. 加入 Steam 库（请先完全退出 Steam，或加 -force）
steam-gacha-launcher steam-add zzz-cn

# 5. 重启 Steam 后，通过 Steam 一键拉起
steam-gacha-launcher launch zzz-cn -mode steam

# 6. 覆盖路径与启动参数
steam-gacha-launcher launch hsr-cn -path "D:/Star Rail" -args "-platform_type CN"

# 7. 生成示例配置
steam-gacha-launcher config-init
```

---

## 配置示例

```yaml
launch_mode: direct          # direct | launcher | steam

steam:
  path: ""                   # Steam 目录，留空自动检测
  account_id: ""             # userdata 账号目录，留空自动选择

games:
  genshin-cn:                # 覆盖内置游戏
    path: "D:/Games/Genshin Impact"
    launch_args: ["-platform_type", "CN"]
    tags: ["原神", "二次元"]

  my-game:                   # 自定义游戏
    name: "我的自定义游戏"
    path: "D:/Games/MyGame"
    exe: "MyGame.exe"
    launcher_exe: "Launcher.exe"
```

---

## 内置支持的游戏

| id | 名称 | 主程序 | 备注 |
|----|------|--------|------|
| `genshin-cn` | 原神 | `YuanShen.exe` | CN 主程序即国服 |
| `hsr-cn` | 崩坏：星穹铁道 | `StarRail.exe` | 可用 `-platform_type CN` 区分 |
| `zzz-cn` | 绝区零 | `ZenlessZoneZero.exe` | HoYoPlay 布局已兼容 |
| `wuthering-waves-cn` | 鸣潮 | `Client-Win64-Shipping.exe` | 深度目录搜索定位 |
| `endfield-cn` | 明日方舟：终末地 | `Endfield.exe` | 参考项目同款游戏 |

所有画像均为保守默认，实际以你本机安装为准；新增游戏只需在配置里写一条 `games.<id>`。

---

## 与 Better-Endfield 的关系

| 维度 | Better-Endfield | 本项目 |
|------|-----------------|--------|
| 定位 | 为终末地注入 Mod/Hook（IL2CPP 解析、XInput 代理、覆盖层注入等） | 纯启动器，**零注入** |
| 借鉴点 | 「多启动器识别」「免启动器直接拉起」「启动参数配置」等**启动路径**思路 | 复用上述启动思路 |
| Steam 集成 | XInput 代理 / 覆盖层注入 | 标准非 Steam 游戏快捷方式（shortcuts.vdf） |
| 风险 | 涉及游戏运行时注入 | 仅操作 Steam 库与进程启动，风险低 |

---

## 目录结构

```
steam-gacha-launcher/
├── main.go                      # CLI 入口
├── internal/
│   ├── config/                  # 配置加载/生成
│   ├── games/                   # 游戏画像 + 内置列表
│   ├── detect/                  # 注册表/目录/搜索定位
│   ├── launch/                  # 进程启动与 URI 打开
│   └── steam/                   # Steam 目录检测 + shortcuts.vdf 读写
├── config.example.yaml
├── .github/workflows/build.yml  # 云端编译
└── LICENSE
```

---

## 免责声明

本工具为独立的非官方项目，与上述任何游戏厂商、发行商及 Valve 均无关联。使用前请了解并遵守各游戏的服务条款与账号安全规范；因违反相关条款产生的账号/客户端风险由使用者自行承担。
