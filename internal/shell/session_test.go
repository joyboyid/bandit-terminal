package shell

import (
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/x/vt"
)

func TestEcho(t *testing.T) {
	if _, err := os.Stat("/bin/bash"); err != nil {
		t.Skip("bash is not installed")
	}
	home := t.TempDir()
	t.Setenv("SHELL", "/bin/bash")
	t.Setenv("HOME", home)

	s, err := Start(80, 24)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	s.SendText("echo SMOKE_OK\r")

	deadline := time.Now().Add(4 * time.Second)
	buf := make([]byte, 4096)
	var got []byte
	for time.Now().Before(deadline) {
		_ = s.PTY.SetReadDeadline(time.Now().Add(200 * time.Millisecond))
		n, rerr := s.PTY.Read(buf)
		if n > 0 {
			got = append(got, buf[:n]...)
			if strings.Contains(string(got), "SMOKE_OK") {
				return
			}
		}
		if rerr != nil && !errors.Is(rerr, os.ErrDeadlineExceeded) {
			t.Fatalf("read: %v\n%s", rerr, got)
		}
	}
	t.Fatalf("shell output missing SMOKE_OK:\n%s", got)
}

func TestResolveRespectsEnv(t *testing.T) {
	t.Setenv("SHELL", "/bin/bash")
	if got := Resolve(); got != "/bin/bash" && !strings.HasSuffix(got, "/bash") {
		t.Fatal(got)
	}
}

func TestCursorKey(t *testing.T) {
	emu := vt.NewEmulator(8, 2)
	got := make(chan []byte, 1)
	go func() {
		buf := make([]byte, 16)
		n, _ := emu.Read(buf)
		got <- append([]byte(nil), buf[:n]...)
	}()
	emu.SendKey(vt.KeyPressEvent{Code: vt.KeyUp})
	select {
	case b := <-got:
		if string(b) != "\x1b[A" && string(b) != "\x1bOA" {
			t.Fatalf("up = %q", b)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for a key")
	}
}
