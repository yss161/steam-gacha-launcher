package steam

import (
	"bytes"
	"testing"
)

func TestGenerateAppIdHighBit(t *testing.T) {
	id := GenerateAppId(`C:\Games\Genshin Impact Game\YuanShen.exe`, "原神")
	if id&0x80000000 == 0 {
		t.Fatalf("appid 最高位应为 1, got %#x", id)
	}
	// 确定性：相同输入得到相同结果
	if GenerateAppId(`C:\Games\Genshin Impact Game\YuanShen.exe`, "原神") != id {
		t.Fatal("appid 应具有确定性")
	}
	// 不同输入应不同
	if GenerateAppId(`C:\Games\YuanShen.exe`, "原神") == id {
		t.Fatal("不同输入不应产生相同 appid")
	}
}

func TestVDFRoundTrip(t *testing.T) {
	appID := GenerateAppId(`C:\foo\bar.exe`, "测试")
	e := buildEntry(appID, `C:\foo\bar.exe`, `C:\foo`, "测试", "-platform_type CN", []string{"标签", "二次元"}, "")

	root := newObject()
	appendField(root, "shortcuts", newObject())
	sc := root.children["shortcuts"]
	appendField(sc, "0", e)

	data := root.serialize()

	// 校验结尾标记：对象结束符 0x08（array 与 root 各一个）
	if len(data) < 2 || data[len(data)-1] != 0x08 || data[len(data)-2] != 0x08 {
		t.Fatalf("文件应以 0x08 0x08 结束, 实际末尾: %x", data[len(data)-min(len(data),4):])
	}

	parsed, err := parseVDF(data)
	if err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	sc2 := parsed.children["shortcuts"]
	if sc2 == nil || len(sc2.keys) != 1 {
		t.Fatalf("shortcuts 应含 1 项")
	}
	entry := sc2.children["0"]
	get := func(k string) *vdfNode { return entry.children[k] }

	if got := get("AppName"); got == nil || got.str != "测试" {
		t.Fatalf("AppName 往返错误: %+v", got)
	}
	if got := get("Exe"); got == nil || got.str != `C:\foo\bar.exe` {
		t.Fatalf("Exe 往返错误: %+v", got)
	}
	if got := get("LaunchOptions"); got == nil || got.str != "-platform_type CN" {
		t.Fatalf("LaunchOptions 往返错误: %+v", got)
	}
	if got := get("appid"); got == nil || got.typ != vdfInt || uint32(got.i32) != appID {
		t.Fatalf("appid 往返错误: %+v (want %d)", got, appID)
	}
	if got := get("AllowOverlay"); got == nil || got.i32 != 1 {
		t.Fatalf("AllowOverlay 往返错误: %+v", got)
	}
	tags := get("tags")
	if tags == nil || tags.children["0"] == nil || tags.children["0"].str != "标签" || tags.children["1"].str != "二次元" {
		t.Fatalf("tags 往返错误: %+v", tags)
	}
}

func TestVDFRoundTripEmptyAndExisting(t *testing.T) {
	// 模拟：已有 1 条快捷方式，再追加 1 条
	a := buildEntry(1, `C:\a.exe`, `C:\`, "A", "", nil, "")
	b := buildEntry(2, `C:\b.exe`, `C:\`, "B", "", nil, "")
	root := newObject()
	appendField(root, "shortcuts", newObject())
	sc := root.children["shortcuts"]
	appendField(sc, "0", a)
	appendField(sc, "1", b)
	data := root.serialize()

	parsed, err := parseVDF(data)
	if err != nil {
		t.Fatal(err)
	}
	if got := len(parsed.children["shortcuts"].keys); got != 2 {
		t.Fatalf("期望 2 条, got %d", got)
	}
	// 再次序列化应字节级一致
	if !bytes.Equal(parsed.serialize(), data) {
		t.Fatal("解析后再序列化应一致")
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
