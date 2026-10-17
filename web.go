// web.go —— 图形化界面（本地 Web UI）。
//
// `steam-gacha-launcher web` 启动一个仅监听 127.0.0.1 的本地 HTTP 服务，
// 提供深色主题的单页操作界面，并自动在默认浏览器中打开。
package main

import (
	"embed"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/yss161/steam-gacha-launcher/internal/config"
	"github.com/yss161/steam-gacha-launcher/internal/detect"
	"github.com/yss161/steam-gacha-launcher/internal/games"
	"github.com/yss161/steam-gacha-launcher/internal/launch"
	"github.com/yss161/steam-gacha-launcher/internal/steam"
)

//go:embed web/index.html
var webFS embed.FS

type webServer struct {
	cfgPath string
	cfg     *config.Config
	mu      sync.Mutex
}

func newWebServer(cfgPath string) *webServer {
	if cfgPath == "" {
		if exe, err := os.Executable(); err == nil {
			cfgPath = filepath.Join(filepath.Dir(exe), "config.yaml")
		}
	}
	cfg, err := config.Load(cfgPath)
	if err != nil {
		cfg = config.Default()
	}
	return &webServer{cfgPath: cfgPath, cfg: cfg}
}

func cmdWeb(c *cli) error {
	ln, err := net.Listen("tcp", "127.0.0.1:8080")
	if err != nil {
		ln, err = net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			return fmt.Errorf("启动服务失败: %w", err)
		}
	}
	srv := newWebServer(c.configPath)
	url := "http://" + ln.Addr().String()
	fmt.Printf("图形界面已启动: %s\n", url)
	fmt.Println("按 Ctrl+C 退出")
	_ = launch.OpenURI(url)
	return http.Serve(ln, srv.handler())
}

func (s *webServer) handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		data, err := webFS.ReadFile("web/index.html")
		if err != nil {
			http.Error(w, "index.html not found", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write(data)
	})
	mux.HandleFunc("/api/state", s.handleState)
	mux.HandleFunc("/api/launch", s.handleLaunch)
	mux.HandleFunc("/api/steam-add", s.handleSteamAdd)
	mux.HandleFunc("/api/steam-remove", s.handleSteamRemove)
	mux.HandleFunc("/api/steam-uri", s.handleSteamURI)
	mux.HandleFunc("/api/save-config", s.handleSaveConfig)
	return mux
}

// ---------- 通用 ----------

type actionReq struct {
	ID    string `json:"id"`
	Mode  string `json:"mode"`
	Path  string `json:"path"`
	Args  string `json:"args"`
	Force bool   `json:"force"`
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, code int, err error) {
	writeJSON(w, code, map[string]string{"error": err.Error()})
}

func decodeBody(w http.ResponseWriter, r *http.Request, v any) bool {
	defer r.Body.Close()
	if err := json.NewDecoder(r.Body).Decode(v); err != nil {
		writeErr(w, http.StatusBadRequest, fmt.Errorf("请求解析失败: %w", err))
		return false
	}
	return true
}

func (s *webServer) snapshot() *config.Config {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.cfg
}

// ---------- 状态 ----------

type gameState struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Builtin     bool     `json:"builtin"`
	Detected    bool     `json:"detected"`
	ExePath     string   `json:"exe_path,omitempty"`
	Path        string   `json:"path"`
	Args        []string `json:"args"`
	ArgsStr     string   `json:"args_str"`
	LauncherExe string   `json:"launcher_exe"`
}

type stateResp struct {
	LaunchMode   string      `json:"launch_mode"`
	SteamPath    string      `json:"steam_path"`
	SteamAccount string      `json:"steam_account"`
	SteamFound   bool        `json:"steam_found"`
	SteamRunning bool        `json:"steam_running"`
	Games        []gameState `json:"games"`
}

func (s *webServer) handleState(w http.ResponseWriter, r *http.Request) {
	cfg := s.snapshot()
	resp := stateResp{
		LaunchMode:   cfg.LaunchMode,
		SteamPath:    cfg.Steam.Path,
		SteamAccount: cfg.Steam.AccountID,
		SteamRunning: steam.IsSteamRunning(),
		Games:        []gameState{},
	}
	if _, err := steam.FindSteamPath(cfg.Steam.Path); err == nil {
		resp.SteamFound = true
	}

	seen := map[string]bool{}
	for _, p := range games.Builtin() {
		seen[p.ID] = true
		resp.Games = append(resp.Games, s.buildGameState(cfg, p.ID, p, false))
	}
	var custom []string
	for id := range cfg.Games {
		if !seen[id] {
			custom = append(custom, id)
		}
	}
	sort.Strings(custom)
	for _, id := range custom {
		resp.Games = append(resp.Games, s.buildGameState(cfg, id, nil, true))
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *webServer) buildGameState(cfg *config.Config, id string, p *games.Profile, custom bool) gameState {
	gc, hasCfg := cfg.Games[id]
	g := gameState{ID: id, Builtin: !custom, Args: []string{}}
	if p != nil {
		g.Name = p.Name
		g.LauncherExe = p.LauncherExe
		g.Args = append([]string{}, p.DefaultArgs...)
	}
	if hasCfg {
		if gc.Name != "" {
			g.Name = gc.Name
		}
		if gc.LauncherExe != "" {
			g.LauncherExe = gc.LauncherExe
		}
		if len(gc.LaunchArgs) > 0 {
			g.Args = append([]string{}, gc.LaunchArgs...)
		}
		g.Path = gc.Path
	}
	if g.Name == "" {
		g.Name = id
	}
	g.ArgsStr = strings.Join(g.Args, " ")

	if p != nil {
		overridePath, overrideExe := "", ""
		if hasCfg {
			overridePath, overrideExe = gc.Path, gc.Exe
		}
		if res, err := detect.Locate(p, overridePath, overrideExe); err == nil {
			g.Detected = true
			g.ExePath = res.ExePath
		}
	} else if hasCfg && gc.Path != "" && gc.Exe != "" {
		exe := gc.Exe
		if !filepath.IsAbs(exe) {
			exe = filepath.Join(gc.Path, exe)
		}
		if _, err := os.Stat(exe); err == nil {
			g.Detected = true
			g.ExePath = exe
		}
	}
	return g
}

// ---------- 启动 ----------

func (s *webServer) handleLaunch(w http.ResponseWriter, r *http.Request) {
	var req actionReq
	if !decodeBody(w, r, &req) {
		return
	}
	cfg := s.snapshot()
	spec, err := resolveSpec(cfg, &cli{pathOverride: req.Path, argsOverride: req.Args}, req.ID)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	mode := req.Mode
	if mode == "" {
		mode = cfg.LaunchMode
	}
	if mode == "" {
		mode = "direct"
	}
	switch mode {
	case "direct":
		o := launch.Options{ExePath: spec.ExePath, Args: spec.Args, WorkDir: spec.WorkDir}
		if err := launch.Run(o); err != nil {
			writeErr(w, http.StatusInternalServerError, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"message": "已启动 " + spec.Name})
	case "launcher":
		lp, err := resolveLauncher(spec)
		if err != nil {
			writeErr(w, http.StatusBadRequest, err)
			return
		}
		o := launch.Options{ExePath: lp, Args: spec.LauncherArgs, WorkDir: spec.RootDir}
		if err := launch.Run(o); err != nil {
			writeErr(w, http.StatusInternalServerError, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"message": "已通过官方启动器启动 " + spec.Name})
	case "steam":
		appID, err := ensureShortcut(cfg, &cli{force: req.Force}, spec)
		if err != nil {
			writeErr(w, http.StatusBadRequest, err)
			return
		}
		if err := launch.OpenURI(steam.LaunchURI(appID)); err != nil {
			writeErr(w, http.StatusInternalServerError, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"message": "已通过 Steam 启动 " + spec.Name})
	default:
		writeErr(w, http.StatusBadRequest, fmt.Errorf("未知启动方式 %q", mode))
	}
}

// ---------- Steam 快捷方式 ----------

func (s *webServer) handleSteamAdd(w http.ResponseWriter, r *http.Request) {
	var req actionReq
	if !decodeBody(w, r, &req) {
		return
	}
	cfg := s.snapshot()
	spec, err := resolveSpec(cfg, &cli{pathOverride: req.Path}, req.ID)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	appID, err := ensureShortcut(cfg, &cli{force: req.Force}, spec)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"appid":     uint32(appID),
		"uri":       steam.LaunchURI(appID),
		"long_id":   steam.GenerateLongAppId(spec.ExePath, spec.Name),
		"name":      spec.Name,
		"exe":       spec.ExePath,
		"message":   "已加入 Steam 库，请重启 Steam 后在库中找到「" + spec.Name + "」",
	})
}

func (s *webServer) handleSteamRemove(w http.ResponseWriter, r *http.Request) {
	var req actionReq
	if !decodeBody(w, r, &req) {
		return
	}
	cfg := s.snapshot()
	spec, err := resolveSpec(cfg, &cli{pathOverride: req.Path}, req.ID)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	if !req.Force && steam.IsSteamRunning() {
		writeErr(w, http.StatusBadRequest, fmt.Errorf("Steam 正在运行，请先完全退出（或勾选「强制写入 Steam」）"))
		return
	}
	steamPath, err := steam.FindSteamPath(cfg.Steam.Path)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	ud, err := steam.FindUserDataDir(steamPath, cfg.Steam.AccountID)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	ok, err := steam.RemoveShortcut(steam.ShortcutsPath(ud), spec.ExePath)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	if !ok {
		writeJSON(w, http.StatusOK, map[string]string{"message": "未在 Steam 库中找到该游戏"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"message": "已从 Steam 库移除，请重启 Steam 生效"})
}

func (s *webServer) handleSteamURI(w http.ResponseWriter, r *http.Request) {
	var req actionReq
	if !decodeBody(w, r, &req) {
		return
	}
	cfg := s.snapshot()
	spec, err := resolveSpec(cfg, &cli{pathOverride: req.Path}, req.ID)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	appID := steam.GenerateAppId(spec.ExePath, spec.Name)
	writeJSON(w, http.StatusOK, map[string]any{
		"uri":     steam.LaunchURI(appID),
		"appid":   uint32(appID),
		"long_id": steam.GenerateLongAppId(spec.ExePath, spec.Name),
	})
}

// ---------- 保存配置 ----------

type saveReq struct {
	LaunchMode string            `json:"launch_mode"`
	Steam      struct {
		Path      string `json:"path"`
		AccountID string `json:"account_id"`
	} `json:"steam"`
	Games map[string]struct {
		Path string `json:"path"`
		Args string `json:"args"`
	} `json:"games"`
}

func (s *webServer) handleSaveConfig(w http.ResponseWriter, r *http.Request) {
	var req saveReq
	if !decodeBody(w, r, &req) {
		return
	}
	s.mu.Lock()
	cfg := s.cfg
	if req.LaunchMode != "" {
		cfg.LaunchMode = req.LaunchMode
	}
	cfg.Steam.Path = req.Steam.Path
	cfg.Steam.AccountID = req.Steam.AccountID

	for id, g := range req.Games {
		gc := cfg.Games[id]
		if p := games.ByName(id); p != nil && gc.Name == "" {
			gc.Name = p.Name
			gc.LauncherExe = p.LauncherExe
			gc.Tags = append([]string{}, p.Tags...)
		}
		if games.ByName(id) == nil {
			// 自定义游戏：仅在提供 path 时更新
			if g.Path != "" {
				gc.Path = g.Path
			}
		} else {
			gc.Path = g.Path
		}
		gc.LaunchArgs = strings.Fields(g.Args)

		if gc.Path == "" && len(gc.LaunchArgs) == 0 && games.ByName(id) != nil {
			delete(cfg.Games, id)
		} else {
			cfg.Games[id] = gc
		}
	}

	if err := config.Save(s.cfgPath, cfg); err != nil {
		s.mu.Unlock()
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	s.cfg = cfg
	s.mu.Unlock()
	writeJSON(w, http.StatusOK, map[string]string{"message": "已保存到 " + s.cfgPath})
}
