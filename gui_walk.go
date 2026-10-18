//go:build windows

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/lxn/walk"
	. "github.com/lxn/walk/declarative"

	"github.com/yss161/steam-gacha-launcher/internal/config"
	"github.com/yss161/steam-gacha-launcher/internal/games"
	"github.com/yss161/steam-gacha-launcher/internal/launch"
	"github.com/yss161/steam-gacha-launcher/internal/steam"
	"github.com/yss161/steam-gacha-launcher/internal/update"
)

// gameRow 为列表中的一行。
type gameRow struct {
	id     string
	name   string
	status string
	path   string
}

// gameModel 为 TableView 的数据模型。
type gameModel struct {
	walk.TableModelBase
	rows []gameRow
}

func (m *gameModel) RowCount() int { return len(m.rows) }

func (m *gameModel) Value(row, col int) interface{} {
	r := m.rows[row]
	switch col {
	case 0:
		return r.name
	case 1:
		return r.status
	case 2:
		return r.path
	}
	return ""
}

// guiApp 承载界面状态。
type guiApp struct {
	cfg             *config.Config
	cfgPath         string
	model           *gameModel
	mw              *walk.MainWindow
	tv              *walk.TableView
	detailLbl       *walk.TextLabel
	statusItem      *walk.StatusBarItem
	steamStatusItem *walk.StatusBarItem
}

// openNativeGUI 打开原生 Win32 图形界面（Walk）。
func openNativeGUI(c *cli) error {
	cfgPath := c.configPath
	if cfgPath == "" {
		if exe, err := os.Executable(); err == nil {
			cfgPath = filepath.Join(filepath.Dir(exe), "config.yaml")
		}
	}
	cfg, err := config.Load(cfgPath)
	if err != nil {
		cfg = config.Default()
	}
	a := &guiApp{cfg: cfg, cfgPath: cfgPath, model: &gameModel{}}
	a.scan()

	err = (MainWindow{
		AssignTo: &a.mw,
		Title:    "Steam 国服二游启动器",
		Size:     Size{Width: 1040, Height: 660},
		MinSize:  Size{Width: 860, Height: 520},
		Layout:   VBox{MarginsZero: true},

		MenuItems: []MenuItem{
			Menu{Text: "文件(&F)", Items: []MenuItem{
				Action{Text: "重新扫描(&R)", Shortcut: Shortcut{Modifiers: walk.ModControl, Key: walk.KeyR}, OnTriggered: a.onRescan},
				Action{Text: "打开配置文件(&C)", OnTriggered: a.onOpenConfig},
				Separator{},
				Action{Text: "退出(&X)", OnTriggered: func() { a.mw.Close() }},
			}},
			Menu{Text: "帮助(&H)", Items: []MenuItem{
				Action{Text: "检查更新(&U)", OnTriggered: a.onCheckUpdate},
				Separator{},
				Action{Text: "关于(&A)", OnTriggered: a.onAbout},
			}},
		},

		ToolBar: ToolBar{
			ButtonStyle: ToolBarButtonTextOnly,
			Items: []MenuItem{
				Action{Text: "直接启动", OnTriggered: func() { a.launch("direct") }},
				Action{Text: "经启动器", OnTriggered: func() { a.launch("launcher") }},
				Action{Text: "经 Steam", OnTriggered: func() { a.launch("steam") }},
				Separator{},
				Action{Text: "加入 Steam", OnTriggered: a.onAdd},
				Action{Text: "移除", OnTriggered: a.onRemove},
			},
		},

		Children: []Widget{
			HSplitter{
				HandleWidth: 6,
				Children: []Widget{
					TableView{
						AssignTo:            &a.tv,
						AlternatingRowBG:    true,
						LastColumnStretched: true,
						ColumnsOrderable:    true,
						Columns: []TableViewColumn{
							{Title: "游戏", Width: 200},
							{Title: "状态", Width: 80},
							{Title: "主程序路径", Width: 440},
						},
						Model:                 a.model,
						OnCurrentIndexChanged: a.onSelChanged,
						OnItemActivated:       a.onEdit,
						StyleCell:             a.styleCell,
						ContextMenuItems: []MenuItem{
							Action{Text: "直接启动", OnTriggered: func() { a.launch("direct") }},
							Action{Text: "经官方启动器", OnTriggered: func() { a.launch("launcher") }},
							Action{Text: "经 Steam 启动", OnTriggered: func() { a.launch("steam") }},
							Separator{},
							Action{Text: "加入 Steam 库", OnTriggered: a.onAdd},
							Action{Text: "移除", OnTriggered: a.onRemove},
							Separator{},
							Action{Text: "编辑路径/参数", OnTriggered: a.onEdit},
						},
					},
					GroupBox{
						Title:   "详情",
						MinSize: Size{Width: 300},
						Layout:  VBox{Margins: Margins{Left: 10, Top: 10, Right: 10, Bottom: 10}, Spacing: 8},
						Children: []Widget{
							TextLabel{AssignTo: &a.detailLbl, Text: "", MinSize: Size{Width: 260}},
							VSpacer{},
							PushButton{Text: "启动国服（经 Steam 库）", OnClicked: a.onLaunchSteamApp},
							PushButton{Text: "一键假入库 / 刷新", OnClicked: a.onInstall},
							PushButton{Text: "取消假入库", OnClicked: a.onUninstall},
							PushButton{Text: "编辑路径/参数", OnClicked: a.onEdit},
							PushButton{Text: "设置", OnClicked: a.onSettings},
							PushButton{Text: "重新扫描", OnClicked: a.onRescan},
							PushButton{Text: "打开配置文件", OnClicked: a.onOpenConfig},
						},
					},
				},
			},
		},

		StatusBarItems: []StatusBarItem{
			{AssignTo: &a.statusItem, Text: "就绪"},
			{AssignTo: &a.steamStatusItem, Text: a.steamStatus()},
		},
	}).Create()
	if err != nil {
		return err
	}

	// 后置初始化：状态栏第一项伸缩（Steam 信息右对齐）、默认选中首行。
	if a.statusItem != nil {
		_ = a.statusItem.SetWidth(-1)
	}
	if len(a.model.rows) > 0 {
		_ = a.tv.SetCurrentIndex(0)
	}
	a.refreshDetail()

	a.mw.Run()
	return nil
}

// scan 重新检测游戏并刷新列表。
func (a *guiApp) scan() {
	rows := []gameRow{}
	seen := map[string]bool{}
	for _, p := range games.Builtin() {
		seen[p.ID] = true
		rows = append(rows, a.buildRow(p.ID))
	}
	var custom []string
	for id := range a.cfg.Games {
		if !seen[id] {
			custom = append(custom, id)
		}
	}
	sort.Strings(custom)
	for _, id := range custom {
		rows = append(rows, a.buildRow(id))
	}
	a.model.rows = rows
	a.model.PublishRowsReset()
}

func (a *guiApp) buildRow(id string) gameRow {
	row := gameRow{id: id, name: id, status: "未检测到"}
	if p := games.ByName(id); p != nil {
		row.name = p.Name
	}
	if gc, ok := a.cfg.Games[id]; ok && gc.Name != "" {
		row.name = gc.Name
	}
	if spec, err := resolveSpec(a.cfg, &cli{}, id); err == nil {
		row.status = "已定位"
		row.path = spec.ExePath
	}
	return row
}

func (a *guiApp) steamStatus() string {
	sp, err := steam.FindSteamPath(a.cfg.Steam.Path)
	if err != nil {
		return "Steam：未找到（可在「设置」中指定）"
	}
	s := "Steam：" + sp
	if steam.IsSteamRunning() {
		s += " · 运行中"
	} else {
		s += " · 未运行"
	}
	return s
}

func (a *guiApp) setStatus(s string) {
	if a.statusItem != nil {
		_ = a.statusItem.SetText(s)
	}
}

func (a *guiApp) err(title string, err error) {
	a.setStatus(title + "：" + err.Error())
	walk.MsgBox(a.mw, title, err.Error(), walk.MsgBoxIconError)
}

func (a *guiApp) selected() (string, *gameSpec, bool) {
	idx := a.tv.CurrentIndex()
	if idx < 0 || idx >= len(a.model.rows) {
		a.setStatus("请先选择一款游戏")
		return "", nil, false
	}
	id := a.model.rows[idx].id
	spec, err := resolveSpec(a.cfg, &cli{}, id)
	if err != nil {
		a.setStatus("该游戏未定位：" + err.Error())
		return id, nil, false
	}
	return id, spec, true
}

// styleCell 为表格单元格着色（状态列：已定位=绿，未检测到=灰）。
func (a *guiApp) styleCell(style *walk.CellStyle) {
	if style.Col() != 1 {
		return
	}
	row := style.Row()
	if row < 0 || row >= len(a.model.rows) {
		return
	}
	if a.model.rows[row].status == "已定位" {
		style.TextColor = walk.RGB(0, 140, 0)
	} else {
		style.TextColor = walk.RGB(150, 150, 150)
	}
}

// detailText 生成当前选中游戏的详情文本。
func (a *guiApp) detailText() string {
	idx := a.tv.CurrentIndex()
	if idx < 0 || idx >= len(a.model.rows) {
		return "未选择游戏\n\n在左侧列表中选择一款游戏，或双击行编辑路径/参数。"
	}
	r := a.model.rows[idx]
	gc := a.cfg.Games[r.id]
	var b strings.Builder
	fmt.Fprintf(&b, "名称：%s\n", r.name)
	fmt.Fprintf(&b, "状态：%s\n", r.status)
	if r.path != "" {
		fmt.Fprintf(&b, "路径：%s\n", r.path)
	}
	if len(gc.LaunchArgs) > 0 {
		fmt.Fprintf(&b, "参数：%s\n", strings.Join(gc.LaunchArgs, " "))
	}
	if p := games.ByName(r.id); p != nil && p.LauncherExe != "" {
		fmt.Fprintf(&b, "启动器：%s\n", p.LauncherExe)
	}
	// Steam 假入库状态
	if appID := a.appIDOf(r.id); appID != "" {
		st := "未入库"
		if sp, err := steam.FindSteamPath(a.cfg.Steam.Path); err == nil && steam.IsFakeInstalled(sp, appID) {
			st = "已入库（Steam 库显示为已安装）"
		}
		fmt.Fprintf(&b, "Steam 库：%s（AppId %s）\n", st, appID)
	} else {
		fmt.Fprintf(&b, "Steam 库：无对应 Steam 版本\n")
	}
	return strings.TrimRight(b.String(), "\n")
}

// appIDOf 返回游戏的 Steam appid（配置覆盖优先于内置）。
func (a *guiApp) appIDOf(id string) string {
	if gc, ok := a.cfg.Games[id]; ok && gc.SteamAppID != "" {
		return gc.SteamAppID
	}
	if p := games.ByName(id); p != nil {
		return p.SteamAppID
	}
	return ""
}

// onLaunchSteamApp 通过 Steam 库启动国服（steam://rungameid/<appid>，需已假入库）。
func (a *guiApp) onLaunchSteamApp() {
	_, spec, ok := a.selected()
	if !ok {
		return
	}
	if spec.SteamAppID == "" {
		walk.MsgBox(a.mw, "无法启动", "该游戏无对应 Steam 版本（未提供 appid），无法经 Steam 库启动。\n可改用「直接启动」或「经 Steam（快捷方式）」。", walk.MsgBoxIconWarning)
		return
	}
	sp, err := steam.FindSteamPath(a.cfg.Steam.Path)
	if err != nil {
		a.err("启动失败", err)
		return
	}
	if !steam.IsFakeInstalled(sp, spec.SteamAppID) {
		if walk.MsgBox(a.mw, "尚未假入库",
			"该游戏尚未在 Steam 库中显示为已安装。\n是否现在「假入库」？",
			walk.MsgBoxYesNo|walk.MsgBoxIconQuestion) != walk.DlgCmdYes {
			return
		}
		a.onInstall()
		if !steam.IsFakeInstalled(sp, spec.SteamAppID) {
			return
		}
	}
	uri := "steam://rungameid/" + spec.SteamAppID
	if err := launch.OpenURI(uri); err != nil {
		a.err("启动失败", err)
		return
	}
	a.setStatus("已通过 Steam 库启动：" + spec.Name)
}

// onInstall 假入库：让游戏在 Steam 库中显示为已安装。
func (a *guiApp) onInstall() {
	_, spec, ok := a.selected()
	if !ok {
		return
	}
	if spec.SteamAppID == "" {
		walk.MsgBox(a.mw, "无法假入库",
			"游戏「"+spec.Name+"」未提供 Steam appid（需在 Steam 有对应版本）。\n可在 config.yaml 中设置 steam_appid。",
			walk.MsgBoxIconWarning)
		return
	}
	if steam.IsSteamRunning() {
		if walk.MsgBox(a.mw, "Steam 正在运行",
			"假入库后需完全重启 Steam，库才会刷新显示。\n是否继续？",
			walk.MsgBoxYesNo|walk.MsgBoxIconWarning) != walk.DlgCmdYes {
			return
		}
	}
	sp, err := steam.FindSteamPath(a.cfg.Steam.Path)
	if err != nil {
		a.err("假入库失败", err)
		return
	}
	installdir := steamInstallDirOf(spec)
	contentDir := steamContentDir(spec)
	// 幂等：已入库则先移除再重装（一键刷新到最新数据）
	refreshing := steam.IsFakeInstalled(sp, spec.SteamAppID)
	if refreshing {
		_ = steam.RemoveFakeInstall(sp, spec.SteamAppID, installdir)
	} else if _, err := os.Lstat(filepath.Join(sp, "steamapps", "common", installdir)); err == nil {
		refreshing = true
		_ = steam.RemoveFakeInstall(sp, spec.SteamAppID, installdir)
	}
	linkDir, err := steam.FakeInstall(sp, spec.SteamAppID, spec.Name, installdir, contentDir, steamDepotOf(spec))
	if err != nil {
		a.err("假入库失败", err)
		return
	}
	a.refreshDetail()
	title := "假入库完成"
	if refreshing {
		title = "已刷新"
	}
	a.setStatus(spec.Name + "：" + title)
	walk.MsgBox(a.mw, title,
		fmt.Sprintf("AppId：%s（buildid %s）\n库内目录：%s\n联接至：%s\n\n请重启 Steam，「%s」将显示为已安装，可从库中启动国服。",
			spec.SteamAppID, spec.SteamBuildID, linkDir, contentDir, spec.Name),
		walk.MsgBoxIconInformation)
}

// onCheckUpdate 检查并应用自更新。
func (a *guiApp) onCheckUpdate() {
	a.setStatus("正在检查更新…")
	rel, err := update.LatestRelease()
	if err != nil {
		a.err("检查更新失败", err)
		return
	}
	if !update.IsNewer(version, rel.Tag) {
		walk.MsgBox(a.mw, "检查更新", fmt.Sprintf("已是最新版本（%s）。", version), walk.MsgBoxIconInformation)
		return
	}
	if walk.MsgBox(a.mw, "发现新版本",
		fmt.Sprintf("当前：%s\n最新：%s\n\n是否现在下载并更新？", version, rel.Tag),
		walk.MsgBoxYesNo|walk.MsgBoxIconQuestion) != walk.DlgCmdYes {
		return
	}
	tmp := filepath.Join(os.TempDir(), "steam-gacha-launcher-update.exe")
	a.setStatus("正在下载更新…")
	if err := update.Download(rel.URL, tmp); err != nil {
		a.err("下载失败", err)
		return
	}
	if err := update.SelfReplace(tmp); err != nil {
		a.err("更新失败", err)
		return
	}
	os.Exit(0)
}

// onUninstall 取消假入库（不影响游戏文件）。
func (a *guiApp) onUninstall() {
	_, spec, ok := a.selected()
	if !ok {
		return
	}
	if spec.SteamAppID == "" {
		a.setStatus("该游戏未提供 Steam appid")
		return
	}
	sp, err := steam.FindSteamPath(a.cfg.Steam.Path)
	if err != nil {
		a.err("取消失败", err)
		return
	}
	if !steam.IsFakeInstalled(sp, spec.SteamAppID) {
		a.setStatus("该游戏未假入库")
		return
	}
	if walk.MsgBox(a.mw, "取消假入库",
		"将删除 Steam 库中的安装记录与目录联接（不影响游戏文件）。\n继续？",
		walk.MsgBoxYesNo|walk.MsgBoxIconQuestion) != walk.DlgCmdYes {
		return
	}
	installdir := steamInstallDirOf(spec)
	if err := steam.RemoveFakeInstall(sp, spec.SteamAppID, installdir); err != nil {
		a.err("取消失败", err)
		return
	}
	a.refreshDetail()
	a.setStatus("已取消假入库：" + spec.Name)
}

func (a *guiApp) refreshDetail() {
	if a.detailLbl != nil {
		_ = a.detailLbl.SetText(a.detailText())
	}
}

func (a *guiApp) onSelChanged() {
	a.refreshDetail()
}

func (a *guiApp) launch(mode string) {
	_, spec, ok := a.selected()
	if !ok {
		return
	}
	switch mode {
	case "direct":
		o := launch.Options{ExePath: spec.ExePath, Args: spec.Args, WorkDir: spec.WorkDir}
		if err := launch.Run(o); err != nil {
			a.err("启动失败", err)
			return
		}
		a.setStatus("已启动：" + spec.Name)
	case "launcher":
		lp, err := resolveLauncher(spec)
		if err != nil {
			a.err("无法使用官方启动器", err)
			return
		}
		o := launch.Options{ExePath: lp, Args: spec.LauncherArgs, WorkDir: spec.RootDir}
		if err := launch.Run(o); err != nil {
			a.err("启动失败", err)
			return
		}
		a.setStatus("已通过官方启动器启动：" + spec.Name)
	case "steam":
		appID, err := ensureShortcut(a.cfg, &cli{}, spec)
		if err != nil {
			a.err("经 Steam 启动失败", err)
			return
		}
		if err := launch.OpenURI(steam.LaunchURI(appID)); err != nil {
			a.err("打开 Steam 失败", err)
			return
		}
		a.setStatus("已通过 Steam 启动：" + spec.Name)
	}
}

func (a *guiApp) onAdd() {
	_, spec, ok := a.selected()
	if !ok {
		return
	}
	force := false
	if steam.IsSteamRunning() {
		if walk.MsgBox(a.mw, "Steam 正在运行",
			"Steam 正在运行，直接写入会在退出时被覆盖。\n是否强制写入？",
			walk.MsgBoxYesNo|walk.MsgBoxIconWarning) != walk.DlgCmdYes {
			return
		}
		force = true
	}
	appID, err := ensureShortcut(a.cfg, &cli{force: force}, spec)
	if err != nil {
		a.err("加入 Steam 失败", err)
		return
	}
	msg := fmt.Sprintf("AppId：%d\nURI：%s\n\n请重启 Steam 后在库中找到「%s」。",
		uint32(appID), steam.LaunchURI(appID), spec.Name)
	a.setStatus("已加入 Steam 库：" + spec.Name)
	walk.MsgBox(a.mw, "已加入 Steam 库", msg, walk.MsgBoxIconInformation)
}

func (a *guiApp) onRemove() {
	_, spec, ok := a.selected()
	if !ok {
		return
	}
	if steam.IsSteamRunning() {
		if walk.MsgBox(a.mw, "Steam 正在运行",
			"请先完全退出 Steam 再移除，否则可能被覆盖。\n是否仍要尝试移除？",
			walk.MsgBoxYesNo|walk.MsgBoxIconWarning) != walk.DlgCmdYes {
			return
		}
	}
	steamPath, err := steam.FindSteamPath(a.cfg.Steam.Path)
	if err != nil {
		a.err("移除失败", err)
		return
	}
	ud, err := steam.FindUserDataDir(steamPath, a.cfg.Steam.AccountID)
	if err != nil {
		a.err("移除失败", err)
		return
	}
	removed, err := steam.RemoveShortcut(steam.ShortcutsPath(ud), spec.ExePath)
	if err != nil {
		a.err("移除失败", err)
		return
	}
	if !removed {
		a.setStatus("未在 Steam 库中找到该游戏")
		return
	}
	a.setStatus("已从 Steam 库移除：" + spec.Name)
}

func (a *guiApp) onEdit() {
	idx := a.tv.CurrentIndex()
	if idx < 0 || idx >= len(a.model.rows) {
		a.setStatus("请先选择一款游戏")
		return
	}
	id := a.model.rows[idx].id
	gc := a.cfg.Games[id]

	var pathLe, argsLe *walk.LineEdit
	var dlg *walk.Dialog

	result, err := (Dialog{
		AssignTo: &dlg,
		Title:    "编辑「" + a.model.rows[idx].name + "」",
		MinSize:  Size{Width: 480},
		Layout:   VBox{Spacing: 8, Margins: Margins{Left: 10, Top: 10, Right: 10, Bottom: 10}},
		Children: []Widget{
			Label{Text: "安装路径（留空 = 自动检测）："},
			LineEdit{AssignTo: &pathLe, Text: gc.Path},
			Label{Text: "启动参数（空格分隔，如 -platform_type CN）："},
			LineEdit{AssignTo: &argsLe, Text: strings.Join(gc.LaunchArgs, " ")},
			Composite{Layout: HBox{Spacing: 8, MarginsZero: true}, Children: []Widget{
				HSpacer{},
				PushButton{Text: "取消", OnClicked: func() { dlg.Cancel() }},
				PushButton{Text: "保存", OnClicked: func() { dlg.Accept() }},
			}},
		},
	}).Run(a.mw)
	if err != nil {
		a.err("对话框错误", err)
		return
	}
	if result != walk.DlgCmdOK {
		return
	}

	gc = a.cfg.Games[id]
	gc.Path = strings.TrimSpace(pathLe.Text())
	gc.LaunchArgs = strings.Fields(argsLe.Text())
	if p := games.ByName(id); p != nil && gc.Name == "" {
		gc.Name = p.Name
		gc.LauncherExe = p.LauncherExe
		gc.Tags = append([]string{}, p.Tags...)
	}
	if gc.Path == "" && len(gc.LaunchArgs) == 0 && games.ByName(id) != nil {
		delete(a.cfg.Games, id)
	} else {
		a.cfg.Games[id] = gc
	}
	if err := config.Save(a.cfgPath, a.cfg); err != nil {
		a.err("保存失败", err)
		return
	}
	a.scan()
	a.refreshDetail()
	a.setStatus("已保存配置")
}

func (a *guiApp) onSettings() {
	var pathLe, accLe *walk.LineEdit
	var dlg *walk.Dialog

	result, err := (Dialog{
		AssignTo: &dlg,
		Title:    "设置",
		MinSize:  Size{Width: 500},
		Layout:   VBox{Spacing: 8, Margins: Margins{Left: 10, Top: 10, Right: 10, Bottom: 10}},
		Children: []Widget{
			Label{Text: "Steam 安装目录（留空自动检测）："},
			LineEdit{AssignTo: &pathLe, Text: a.cfg.Steam.Path},
			Label{Text: "Steam 账号目录（userdata 下，留空自动选择）："},
			LineEdit{AssignTo: &accLe, Text: a.cfg.Steam.AccountID},
			Composite{Layout: HBox{Spacing: 8, MarginsZero: true}, Children: []Widget{
				HSpacer{},
				PushButton{Text: "取消", OnClicked: func() { dlg.Cancel() }},
				PushButton{Text: "保存", OnClicked: func() { dlg.Accept() }},
			}},
		},
	}).Run(a.mw)
	if err != nil {
		a.err("对话框错误", err)
		return
	}
	if result != walk.DlgCmdOK {
		return
	}

	a.cfg.Steam.Path = strings.TrimSpace(pathLe.Text())
	a.cfg.Steam.AccountID = strings.TrimSpace(accLe.Text())
	if err := config.Save(a.cfgPath, a.cfg); err != nil {
		a.err("保存失败", err)
		return
	}
	if a.steamStatusItem != nil {
		_ = a.steamStatusItem.SetText(a.steamStatus())
	}
	a.setStatus("设置已保存")
}

func (a *guiApp) onRescan() {
	a.scan()
	if a.steamStatusItem != nil {
		_ = a.steamStatusItem.SetText(a.steamStatus())
	}
	a.refreshDetail()
	a.setStatus("已重新扫描")
}

func (a *guiApp) onOpenConfig() {
	if _, err := os.Stat(a.cfgPath); os.IsNotExist(err) {
		if err := config.SaveDefault(a.cfgPath); err != nil {
			a.err("生成配置失败", err)
			return
		}
	}
	if err := launch.Run(launch.Options{ExePath: "notepad.exe", Args: []string{a.cfgPath}}); err != nil {
		a.err("打开配置失败", err)
		return
	}
	a.setStatus("已用记事本打开配置文件：" + a.cfgPath)
}

func (a *guiApp) onAbout() {
	walk.MsgBox(a.mw, "关于", "Steam 国服二游启动器\n\n通过 Steam 启动国服二次元游戏的工具。\n版本："+version+"\n\n仓库：https://github.com/yss161/steam-gacha-launcher", walk.MsgBoxIconInformation)
}
