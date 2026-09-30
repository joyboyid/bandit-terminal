package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/creack/pty"
)

func TestTerminalArgs(t *testing.T) {
	if got := strings.Join(terminalArgs("wezterm"), " "); got != "start --" {
		t.Fatal(got)
	}
	if got := strings.Join(terminalArgs("gnome-terminal"), " "); got != "--" {
		t.Fatal(got)
	}
	if got := strings.Join(terminalArgs("/usr/bin/foot"), " "); got != "-e" {
		t.Fatal(got)
	}
}

func TestSmoke(t *testing.T) {
	if _, err := os.Stat("/bin/bash"); err != nil {
		t.Skip("bash is not installed")
	}
	bin := filepath.Join(t.TempDir(), "bandit")
	build := exec.Command("go", "build", "-o", bin, ".")
	build.Dir = "."
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build: %v\n%s", err, out)
	}

	home := t.TempDir()
	if err := os.WriteFile(filepath.Join(home, "marker.dat"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	cmd := exec.Command(bin)
	cmd.Dir = home
	cmd.Env = replaceEnv(os.Environ(), map[string]string{
		"SHELL": "/bin/bash",
		"HOME":  home,
		"TERM":  "xterm-256color",
	})
	ptmx, err := pty.StartWithSize(cmd, &pty.Winsize{Rows: 36, Cols: 110})
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		_ = ptmx.Close()
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
	}()

	var screen safeBuf
	go func() { _, _ = copyUntil(&screen, ptmx) }()

	if !waitFor(&screen, 8*time.Second, "BANDIT", "marker.dat") {
		t.Fatalf("screen never showed the chrome and file list:\n%s", strip(screen.String()))
	}
	if _, err := ptmx.Write([]byte("echo SMOKE_OK\r")); err != nil {
		t.Fatal(err)
	}
	if !waitFor(&screen, 8*time.Second, "SMOKE_OK") {
		t.Fatalf("shell did not echo:\n%s", strip(screen.String()))
	}
	if _, err := ptmx.Write([]byte{0x11}); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("process did not exit on ctrl+q")
	}
}

type safeBuf struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (s *safeBuf) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.Write(p)
}

func (s *safeBuf) String() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.String()
}

func waitFor(buf *safeBuf, d time.Duration, needles ...string) bool {
	deadline := time.Now().Add(d)
	for {
		plain := strip(buf.String())
		ok := true
		for _, n := range needles {
			if !strings.Contains(plain, n) {
				ok = false
				break
			}
		}
		if ok {
			return true
		}
		if !time.Now().Before(deadline) {
			return false
		}
		time.Sleep(40 * time.Millisecond)
	}
}

func copyUntil(dst *safeBuf, src interface{ Read([]byte) (int, error) }) (int64, error) {
	buf := make([]byte, 4096)
	var n int64
	for {
		r, err := src.Read(buf)
		if r > 0 {
			_, _ = dst.Write(buf[:r])
			n += int64(r)
		}
		if err != nil {
			return n, err
		}
	}
}

func replaceEnv(env []string, kv map[string]string) []string {
	out := make([]string, 0, len(env)+len(kv))
	seen := map[string]bool{}
	for _, e := range env {
		key, _, _ := strings.Cut(e, "=")
		if v, ok := kv[key]; ok {
			out = append(out, key+"="+v)
			seen[key] = true
			continue
		}
		out = append(out, e)
	}
	for k, v := range kv {
		if !seen[k] {
			out = append(out, k+"="+v)
		}
	}
	return out
}

func strip(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] != 0x1b {
			b.WriteByte(s[i])
			continue
		}
		if i+1 >= len(s) {
			break
		}
		switch s[i+1] {
		case '[':
			i += 2
			for i < len(s) && (s[i] < 0x40 || s[i] > 0x7e) {
				i++
			}
		case ']':
			i += 2
			for i < len(s) && s[i] != 0x07 {
				if s[i] == 0x1b && i+1 < len(s) && s[i+1] == '\\' {
					i++
					break
				}
				i++
			}
		default:
			i++
		}
	}
	return b.String()
}
