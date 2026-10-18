package steam

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func setupFakeSteam(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	sp := filepath.Join(dir, "Steam")
	if err := os.MkdirAll(filepath.Join(sp, "steamapps", "common"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(sp, "config"), 0o755); err != nil {
		t.Fatal(err)
	}
	// libraryfolders.vdf（带转义反斜杠）
	lf := "\"libraryfolders\"\n{\n\t\"0\"\n\t{\n\t\t\"path\"\t\t\"C:\\\\program files (x86)\\\\steam\"\n\t\t\"apps\"\n\t\t{\n\t\t\t\"4732690\"\t\t\"0\"\n\t\t}\n\t}\n}\n"
	if err := os.WriteFile(filepath.Join(sp, "steamapps", "libraryfolders.vdf"), []byte(lf), 0o644); err != nil {
		t.Fatal(err)
	}
	// loginusers.vdf：一个 AutoLogin=1 账号
	lu := "\"users\"\n{\n\t\"76561199799980040\"\n\t{\n\t\t\"AccountName\"\t\t\"test\"\n\t\t\"AutoLogin\"\t\t\"1\"\n\t\t\"timestamp\"\t\t\"1792250169\"\n\t}\n\t\"76561198682695927\"\n\t{\n\t\t\"AccountName\"\t\t\"other\"\n\t\t\"AutoLogin\"\t\t\"0\"\n\t\t\"timestamp\"\t\t\"1788172100\"\n\t}\n}\n"
	if err := os.WriteFile(filepath.Join(sp, "config", "loginusers.vdf"), []byte(lu), 0o644); err != nil {
		t.Fatal(err)
	}
	return sp
}

func TestLibraryFoldersAndOwner(t *testing.T) {
	sp := setupFakeSteam(t)
	folders := LibraryFolders(sp)
	if len(folders) < 1 || folders[0] != filepath.Join(sp, "steamapps") {
		t.Fatalf("主库目录错误: %v", folders)
	}
	owner := CurrentSteamID64(sp)
	if owner != "76561199799980040" {
		t.Fatalf("SteamID64 错误: %s", owner)
	}
}

func TestFakeInstallRoundTrip(t *testing.T) {
	sp := setupFakeSteam(t)
	// 源目录（模拟国服安装）
	src := filepath.Join(t.TempDir(), "ZenlessZoneZero Game")
	if err := os.MkdirAll(src, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "ZenlessZoneZero.exe"), []byte("fake"), 0o644); err != nil {
		t.Fatal(err)
	}

	if IsFakeInstalled(sp, "4162040") {
		t.Fatal("不应已入库")
	}

	linkDir, err := FakeInstall(sp, "4162040", "绝区零", "ZenlessZoneZero", src)
	if err != nil {
		t.Fatalf("假入库失败: %v", err)
	}

	// junction 应指向源目录，且可访问到 exe
	exe := filepath.Join(linkDir, "ZenlessZoneZero.exe")
	if _, err := os.Stat(exe); err != nil {
		t.Fatalf("junction 无法访问 exe: %v (%s)", err, linkDir)
	}

	// acf 应存在且含关键字段
	acf, err := os.ReadFile(filepath.Join(sp, "steamapps", "appmanifest_4162040.acf"))
	if err != nil {
		t.Fatal(err)
	}
	s := string(acf)
	for _, want := range []string{`"appid"`, `"4162040"`, `"StateFlags"`, `"4"`, `"installdir"`, `"ZenlessZoneZero"`, `"LastOwner"`, `"76561199799980040"`} {
		if !strings.Contains(s, want) {
			t.Fatalf("acf 缺少 %q\n%s", want, s)
		}
	}

	if !IsFakeInstalled(sp, "4162040") {
		t.Fatal("应显示已入库")
	}

	// 移除
	if err := RemoveFakeInstall(sp, "4162040", "ZenlessZoneZero"); err != nil {
		t.Fatalf("移除失败: %v", err)
	}
	if IsFakeInstalled(sp, "4162040") {
		t.Fatal("移除后不应已入库")
	}
	// 源目录必须保留（junction 删除不影响目标）
	if _, err := os.Stat(filepath.Join(src, "ZenlessZoneZero.exe")); err != nil {
		t.Fatal("junction 删除误删了源文件！")
	}
}

func TestFakeInstallNoAppID(t *testing.T) {
	sp := setupFakeSteam(t)
	if _, err := FakeInstall(sp, "", "某游戏", "x", sp); err == nil {
		t.Fatal("空 appid 应报错")
	}
}
