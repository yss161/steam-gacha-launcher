package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestStateEndpoint(t *testing.T) {
	srv := newWebServer("")
	ts := httptest.NewServer(srv.handler())
	defer ts.Close()

	res, err := http.Get(ts.URL + "/api/state")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("状态码 = %d", res.StatusCode)
	}
	var st stateResp
	if err := json.NewDecoder(res.Body).Decode(&st); err != nil {
		t.Fatal(err)
	}
	if len(st.Games) < 5 {
		t.Fatalf("期望至少 5 个内置游戏, got %d", len(st.Games))
	}
	seen := map[string]bool{}
	for _, g := range st.Games {
		if g.Name == "" {
			t.Fatalf("游戏 %q 名称为空", g.ID)
		}
		seen[g.ID] = true
	}
	for _, want := range []string{"genshin-cn", "hsr-cn", "zzz-cn", "wuthering-waves-cn", "endfield-cn"} {
		if !seen[want] {
			t.Fatalf("缺少内置游戏 %s", want)
		}
	}
}

func TestIndexServed(t *testing.T) {
	srv := newWebServer("")
	ts := httptest.NewServer(srv.handler())
	defer ts.Close()

	res, err := http.Get(ts.URL + "/")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("状态码 = %d", res.StatusCode)
	}
	ct := res.Header.Get("Content-Type")
	if ct != "text/html; charset=utf-8" {
		t.Fatalf("Content-Type = %q", ct)
	}
}

func TestSaveConfigRoundTrip(t *testing.T) {
	dir := t.TempDir()
	srv := newWebServer(dir + "/config.yaml")

	// 通过 API 保存配置
	body := `{"launch_mode":"steam","steam":{"path":"","account_id":"abc"},"games":{"genshin-cn":{"path":"D:/Test","args":"-platform_type CN"}}}`
	req := httptest.NewRequest(http.MethodPost, "/api/save-config", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	srv.handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("保存失败: %d %s", rec.Code, rec.Body.String())
	}

	// 重新加载验证
	srv2 := newWebServer(dir + "/config.yaml")
	if srv2.cfg.LaunchMode != "steam" {
		t.Fatalf("LaunchMode = %q, want steam", srv2.cfg.LaunchMode)
	}
	if srv2.cfg.Steam.AccountID != "abc" {
		t.Fatalf("AccountID = %q", srv2.cfg.Steam.AccountID)
	}
	gc, ok := srv2.cfg.Games["genshin-cn"]
	if !ok || gc.Path != "D:/Test" {
		t.Fatalf("genshin-cn 配置未保存: %+v", srv2.cfg.Games)
	}
}
