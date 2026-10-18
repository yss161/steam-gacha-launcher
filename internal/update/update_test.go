package update

import "testing"

func TestIsNewer(t *testing.T) {
	cases := []struct {
		current, latest string
		want            bool
	}{
		{"v1.0.0", "v1.0.0", false},
		{"v1.0.0", "v1.1.0", true},
		{"v1.2.0", "v1.1.0", false},
		{"v1.0.0", "v2.0.0", true},
		{"1.0.0", "1.0.1", true},
		{"dev-abc123", "v1.0.0", true},  // dev 构建视为有更新
		{"v1.0.0", "dev-abc", false},    // 最新版是 dev 则无法比较
		{"v1.0", "v1.0.1", true},
	}
	for _, c := range cases {
		if got := IsNewer(c.current, c.latest); got != c.want {
			t.Errorf("IsNewer(%q, %q) = %v, want %v", c.current, c.latest, got, c.want)
		}
	}
}

func TestVersionParts(t *testing.T) {
	if p := versionParts("v1.2.3"); p == nil || p[0] != 1 || p[1] != 2 || p[2] != 3 {
		t.Fatalf("v1.2.3 -> %v", p)
	}
	if p := versionParts("v1.2-rc1"); p == nil || p[0] != 1 || p[1] != 2 || p[2] != 0 {
		t.Fatalf("v1.2-rc1 -> %v", p)
	}
	if versionParts("abc") != nil {
		t.Fatal("abc 应解析失败")
	}
}
