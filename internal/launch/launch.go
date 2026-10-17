// Package launch 负责启动进程与打开 steam:// URI。
package launch

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// Options 描述一次进程启动。
type Options struct {
	ExePath string
	Args    []string
	WorkDir string
	Env     []string
	Wait    bool
}

// Run 启动进程。Wait=false 时进程独立运行。
func Run(o Options) error {
	if o.ExePath == "" {
		return fmt.Errorf("未指定可执行文件")
	}
	cmd := exec.Command(o.ExePath, o.Args...)
	if o.WorkDir != "" {
		cmd.Dir = o.WorkDir
	}
	if len(o.Env) > 0 {
		cmd.Env = append(os.Environ(), o.Env...)
	}
	if o.Wait {
		return cmd.Run()
	}
	return cmd.Start()
}

// Describe 返回可读的命令行描述（用于 -dry-run）。
func Describe(o Options) string {
	parts := append([]string{quote(o.ExePath)}, quoteAll(o.Args)...)
	return strings.Join(parts, " ")
}

func quote(s string) string {
	if s == "" {
		return `""`
	}
	if strings.ContainsAny(s, " \t\"") {
		return `"` + strings.ReplaceAll(s, `"`, `\"`) + `"`
	}
	return s
}

func quoteAll(ss []string) []string {
	out := make([]string, len(ss))
	for i, s := range ss {
		out[i] = quote(s)
	}
	return out
}

// OpenURI 打开一个 URI（如 steam://rungameid/xxx）。
func OpenURI(uri string) error {
	if uri == "" {
		return fmt.Errorf("空 URI")
	}
	return exec.Command("rundll32", "url.dll,FileProtocolHandler", uri).Start()
}
