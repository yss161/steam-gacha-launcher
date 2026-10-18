// Package update 实现工具自身的检查更新与自更新（从 GitHub Releases）。
package update

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const repo = "yss161/steam-gacha-launcher"

// Release 描述一次 GitHub Release。
type Release struct {
	Tag     string
	URL     string // exe 资产下载地址
	Name    string
	Body    string
}

type ghRelease struct {
	TagName string `json:"tag_name"`
	Name    string `json:"name"`
	Body    string `json:"body"`
	Assets  []struct {
		Name               string `json:"name"`
		BrowserDownloadURL string `json:"browser_download_url"`
	} `json:"assets"`
}

// LatestRelease 查询仓库最新 Release（取其中的 .exe 资产）。
func LatestRelease() (*Release, error) {
	client := &http.Client{Timeout: 15 * time.Second}
	req, err := http.NewRequest("GET", "https://api.github.com/repos/"+repo+"/releases/latest", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "steam-gacha-launcher")
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("检查更新失败（网络）: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("检查更新失败: HTTP %d", resp.StatusCode)
	}
	var gr ghRelease
	if err := json.NewDecoder(resp.Body).Decode(&gr); err != nil {
		return nil, err
	}
	rel := &Release{Tag: gr.TagName, Name: gr.Name, Body: gr.Body}
	for _, a := range gr.Assets {
		if strings.HasSuffix(strings.ToLower(a.Name), ".exe") {
			rel.URL = a.BrowserDownloadURL
			break
		}
	}
	if rel.URL == "" {
		return nil, fmt.Errorf("最新 Release 中未找到 exe 资产")
	}
	return rel, nil
}

// IsNewer 判断 latest 是否比 current 新（按 vX.Y.Z 语义化版本；current 为 dev 构建时恒认为有更新）。
func IsNewer(current, latest string) bool {
	c := versionParts(current)
	l := versionParts(latest)
	if l == nil {
		return false
	}
	if c == nil {
		return true // 当前是 dev 构建，有新版本即提示
	}
	for i := 0; i < 3; i++ {
		if l[i] > c[i] {
			return true
		}
		if l[i] < c[i] {
			return false
		}
	}
	return false
}

// versionParts 把 "v1.2.3" / "1.2" 解析为 [1,2,3]；无法解析返回 nil。
func versionParts(v string) []int {
	v = strings.TrimSpace(v)
	v = strings.TrimPrefix(v, "v")
	v = strings.TrimPrefix(v, "V")
	if i := strings.IndexAny(v, "-+"); i >= 0 {
		v = v[:i]
	}
	segs := strings.Split(v, ".")
	if len(segs) == 0 || len(segs) > 3 {
		return nil
	}
	out := []int{0, 0, 0}
	for i, s := range segs {
		n, err := strconv.Atoi(strings.TrimSpace(s))
		if err != nil || n < 0 {
			return nil
		}
		out[i] = n
	}
	return out
}

// Download 下载 url 到 dest，返回错误。
func Download(url, dest string) error {
	client := &http.Client{Timeout: 5 * time.Minute}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "steam-gacha-launcher")
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("下载失败: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return fmt.Errorf("下载失败: HTTP %d", resp.StatusCode)
	}
	f, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = io.Copy(f, resp.Body)
	return err
}

// SelfReplace 用 newExe 替换当前正在运行的可执行文件，并在替换后重新启动。
// Windows 下运行中的 exe 不能被直接覆盖：先改名、再由一个分离的 cmd 稍后移动覆盖并重启。
func SelfReplace(newExe string) error {
	self, err := os.Executable()
	if err != nil {
		return err
	}
	self, _ = filepath.Abs(self)
	newExe, _ = filepath.Abs(newExe)

	old := self + ".old"
	_ = os.Remove(old)
	// 运行中的 exe 可改名
	if err := os.Rename(self, old); err != nil {
		return fmt.Errorf("无法重命名当前程序: %w", err)
	}
	// 分离的 cmd：等待 ~2s（等本进程退出）→ 移入新 exe → 删除旧文件 → 启动新版本
	script := fmt.Sprintf(
		`ping 127.0.0.1 -n 3 >nul & move /y "%s" "%s" >nul & del /f /q "%s" >nul & start "" "%s"`,
		newExe, self, old, self,
	)
	cmd := exec.Command("cmd", "/c", script)
	_ = cmd.Start()
	return nil
}
