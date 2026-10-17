package steam

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
)

// 本文件实现了 Steam shortcuts.vdf 的最小 VDF（Valve Data Format）读写。
//
// 字节格式（与 steam-shortcut-editor / Steam ROM Manager 一致）：
//   - 值类型字节：object=0x00, string=0x01, int=0x02
//   - 属性名与字符串值均以 0x00 结束；整数为 4 字节小端（int32）
//   - 对象以 0x08 结束
//   - 顶层为单对象 {shortcuts: [ ... ]}，shortcuts 为数字键数组
//
// 每个快捷方式条目的字段（顺序与 Steam 一致）：
//   appid(int32) AppName(string) Exe(string) StartDir(string) icon(string)
//   ShortcutPath(string) LaunchOptions(string) IsHidden(int) AllowDesktopConfig(int)
//   AllowOverlay(int) OpenVR(int) Devkit(int) DevkitGameID(string)
//   DevkitOverrideAppID(int) LastPlayTime(int) FlatpakAppID(string) tags(object)

const (
	vdfObject = 0x00
	vdfString = 0x01
	vdfInt    = 0x02
	vdfEnd    = 0x08
	vdfNul    = 0x00
)

// vdfNode 为 VDF 节点。
type vdfNode struct {
	typ      byte
	str      string
	i32      int32
	keys     []string // 对象的有序键
	children map[string]*vdfNode
}

func newObject() *vdfNode {
	return &vdfNode{typ: vdfObject, children: map[string]*vdfNode{}}
}

func newString(s string) *vdfNode { return &vdfNode{typ: vdfString, str: s} }
func newInt(v int32) *vdfNode    { return &vdfNode{typ: vdfInt, i32: v} }

func appendField(parent *vdfNode, key string, child *vdfNode) {
	parent.keys = append(parent.keys, key)
	parent.children[key] = child
}

// --- 解析 ---

type vdfParser struct {
	data []byte
	pos  int
}

func parseVDF(data []byte) (*vdfNode, error) {
	p := &vdfParser{data: data}
	n, err := p.parseObject()
	if err != nil {
		return nil, err
	}
	return n, nil
}

func (p *vdfParser) parseObject() (*vdfNode, error) {
	n := newObject()
	for {
		if p.pos >= len(p.data) {
			break
		}
		t := p.data[p.pos]
		p.pos++
		if t == vdfEnd {
			break
		}
		name, err := p.parseString()
		if err != nil {
			return nil, err
		}
		child, err := p.parseValue(t)
		if err != nil {
			return nil, err
		}
		appendField(n, name, child)
	}
	return n, nil
}

func (p *vdfParser) parseString() (string, error) {
	start := p.pos
	for p.pos < len(p.data) && p.data[p.pos] != vdfNul {
		p.pos++
	}
	if p.pos >= len(p.data) {
		return "", errors.New("未终止的字符串")
	}
	s := string(p.data[start:p.pos])
	p.pos++ // 跳过 0x00
	return s, nil
}

func (p *vdfParser) parseValue(t byte) (*vdfNode, error) {
	switch t {
	case vdfObject:
		return p.parseObject()
	case vdfString:
		s, err := p.parseString()
		if err != nil {
			return nil, err
		}
		return newString(s), nil
	case vdfInt:
		if p.pos+4 > len(p.data) {
			return nil, errors.New("整数被截断")
		}
		v := int32(binary.LittleEndian.Uint32(p.data[p.pos : p.pos+4]))
		p.pos += 4
		return newInt(v), nil
	default:
		return nil, fmt.Errorf("未知类型字节 0x%02x", t)
	}
}

// --- 序列化 ---

func (n *vdfNode) serialize() []byte {
	var buf bytes.Buffer
	n.writeTo(&buf)
	return buf.Bytes()
}

func (n *vdfNode) writeTo(buf *bytes.Buffer) {
	switch n.typ {
	case vdfObject:
		for _, k := range n.keys {
			child := n.children[k]
			buf.WriteByte(child.typ)
			buf.WriteString(k)
			buf.WriteByte(vdfNul)
			child.writeTo(buf)
		}
		buf.WriteByte(vdfEnd)
	case vdfString:
		buf.WriteString(n.str)
		buf.WriteByte(vdfNul)
	case vdfInt:
		var b [4]byte
		binary.LittleEndian.PutUint32(b[:], uint32(n.i32))
		buf.Write(b[:])
	}
}
