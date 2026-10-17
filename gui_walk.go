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
	cfg        *config.Config
	cfgPath    string
	model      *gameModel
	mw         *walk.MainWindow
	tv         *walk.TableView
	steamLbl   *walk.Label
	statusItem *walk.StatusBarItem
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

	_, err = (MainWindow{
		AssignTo: &a.mw,
		Title:    "Steam 国服二游启动器",
		Size:     Size{Width: 940, Height: 620},
		MinSize:  Size{Width: 760, Height: 460},
		Layout:   VBox{Margins: Margins{Left: 12, Top: 12, Right: 12, Bottom: 12}, Spacing: 8},
		Children: []Widget{
			Label{AssignTo: &a.steamLbl, Text: a.steamStatus()},
			TableView{
				AssignTo:            &a.tv,
				AlternatingRowBG:    true,
				LastColumnStretched: true,
				Columns: []TableViewColumn{
					{Title: "游戏", Width: 220},
					{Title: "状态", Width: 90},
					{Title: "主程序路径", Width: 520},
				},
				Model:           a.model,
				OnItemActivated: a.onEdit,
			},
			Composite{Layout: HBox{Spacing: 8}, Children: []Widget{
				PushButton{Text: "直接启动", OnClicked: func() { a.launch("direct") }},
				PushButton{Text: "经官方启动器", OnClicked: func() { a.launch("launcher") }},
				PushButton{Text: "经 Steam 启动", OnClicked: func() { a.launch("steam") }},
			}},
			Composite{Layout: HBox{Spacing: 8}, Children: []Widget{
				PushButton{Text: "编辑路径/参数", OnClicked: a.onEdit},
				PushButton{Text: "加入 Steam 库", OnClicked: a.onAdd},
				PushButton{Text: "移除", OnClicked: a.onRemove},
				PushButton{Text: "重新扫描", OnClicked: a.onRescan},
				PushButton{Text: "设置", OnClicked: a.onSettings},
				PushButton{Text: "打开配置文件", OnClicked: a.onOpenConfig},
			}},
		},
		StatusBarItems: []StatusBarItem{
			{AssignTo: &a.statusItem, Text: "就绪（双击游戏行可编辑路径/参数）"},
		},
	}).Run()
	return err
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
		return "Steam：未找到（可在「设置」中指定路径）"
	}
	s := "Steam：" + sp
	if steam.IsSteamRunning() {
		s += "　·　运行中"
	} else {
		s += "　·　未运行"
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
	walk.MsgBox(nil, title, err.Error(), walk.MsgBoxIconError)
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
		if walk.MsgBox(nil, "Steam 正在运行",
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
	walk.MsgBox(nil, "已加入 Steam 库", msg, walk.MsgBoxIconInformation)
}

func (a *guiApp) onRemove() {
	_, spec, ok := a.selected()
	if !ok {
		return
	}
	if steam.IsSteamRunning() {
		if walk.MsgBox(nil, "Steam 正在运行",
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
		MinSize:  Size{Width: 460},
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
	a.setStatus("已保存配置")
}

func (a *guiApp) onSettings() {
	var pathLe, accLe *walk.LineEdit
	var dlg *walk.Dialog

	result, err := (Dialog{
		AssignTo: &dlg,
		Title:    "设置",
		MinSize:  Size{Width: 480},
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
	if a.steamLbl != nil {
		_ = a.steamLbl.SetText(a.steamStatus())
	}
	a.setStatus("设置已保存")
}

func (a *guiApp) onRescan() {
	a.scan()
	if a.steamLbl != nil {
		_ = a.steamLbl.SetText(a.steamStatus())
	}
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
