// Package shell runs the user's login shell on a PTY and mirrors it
// into a virtual terminal.
package shell

import (
	"os"
	"os/exec"
	"strings"
	"sync"
	"syscall"

	"github.com/charmbracelet/x/vt"
	"github.com/creack/pty"
)

// Resolve returns the shell binary to launch.
func Resolve() string {
	if sh := os.Getenv("SHELL"); sh != "" {
		if p, err := exec.LookPath(sh); err == nil {
			return p
		}
		if _, err := os.Stat(sh); err == nil {
			return sh
		}
	}
	for _, c := range []string{"/bin/bash", "/usr/bin/bash", "/bin/zsh", "/bin/sh"} {
		if _, err := os.Stat(c); err == nil {
			return c
		}
	}
	return "/bin/sh"
}

// Cwd reads a process's working directory from /proc.
func Cwd(pid int) string {
	if pid <= 0 {
		return ""
	}
	dir, err := os.Readlink("/proc/" + itoa(pid) + "/cwd")
	if err != nil {
		return ""
	}
	return dir
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b [16]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}

// Session is one running shell.
type Session struct {
	Cmd   *exec.Cmd
	PTY   *os.File
	Emu   *vt.Emulator
	Dir   string
	Shell string

	in   *asyncWriter
	once sync.Once
}

// Start launches the shell on a PTY sized cols×rows.
func Start(cols, rows int) (*Session, error) {
	if cols < 2 {
		cols = 80
	}
	if rows < 1 {
		rows = 24
	}
	shellPath := Resolve()
	cmd := exec.Command(shellPath)
	cmd.Env = envWithTerm(os.Environ())
	dir, err := os.Getwd()
	if err != nil {
		dir, _ = os.UserHomeDir()
	}
	cmd.Dir = dir
	cmd.SysProcAttr = &syscall.SysProcAttr{Pdeathsig: syscall.SIGTERM}

	ptmx, err := pty.StartWithSize(cmd, &pty.Winsize{Cols: uint16(cols), Rows: uint16(rows)})
	if err != nil {
		return nil, err
	}
	emu := vt.NewEmulator(cols, rows)
	emu.SetScrollbackSize(2000)

	s := &Session{
		Cmd:   cmd,
		PTY:   ptmx,
		Emu:   emu,
		Dir:   dir,
		Shell: shellPath,
		in:    newAsyncWriter(ptmx),
	}
	go s.in.loop()
	go s.pumpInput()
	return s, nil
}

func envWithTerm(env []string) []string {
	out := make([]string, 0, len(env)+3)
	for _, e := range env {
		if strings.HasPrefix(e, "TERM=") || strings.HasPrefix(e, "COLORTERM=") {
			continue
		}
		out = append(out, e)
	}
	out = append(out, "TERM=xterm-256color", "COLORTERM=truecolor", "BANDIT_TERMINAL=1")
	return out
}

// SendKey encodes a key for the shell, honoring application cursor mode.
func (s *Session) SendKey(ev vt.KeyPressEvent) {
	if s == nil || s.Emu == nil {
		return
	}
	s.Emu.SendKey(ev)
}

// SendText writes raw bytes to the shell, as if they were typed.
func (s *Session) SendText(text string) {
	if s == nil || s.Emu == nil || text == "" {
		return
	}
	s.Emu.SendText(text)
}

// Paste inserts text, using bracketed paste when the shell asked for it.
func (s *Session) Paste(text string) {
	if s == nil || s.Emu == nil || text == "" {
		return
	}
	s.Emu.Paste(text)
}

// Resize updates the virtual terminal and the PTY, which signals the shell.
func (s *Session) Resize(cols, rows int) {
	if s == nil {
		return
	}
	if cols < 2 {
		cols = 2
	}
	if rows < 1 {
		rows = 1
	}
	if s.Emu != nil {
		s.Emu.Resize(cols, rows)
	}
	if s.PTY != nil {
		_ = pty.Setsize(s.PTY, &pty.Winsize{Rows: uint16(rows), Cols: uint16(cols)})
	}
}

// Close hangs up the shell and releases the PTY. It is safe to call twice.
func (s *Session) Close() {
	if s == nil {
		return
	}
	s.once.Do(func() {
		if s.Cmd != nil && s.Cmd.Process != nil && s.Cmd.Process.Pid > 1 {
			_ = syscall.Kill(-s.Cmd.Process.Pid, syscall.SIGTERM)
		}
		if s.in != nil {
			s.in.Close()
		}
		if s.PTY != nil {
			_ = s.PTY.Close()
		}
		// Leave the emulator open. vt.Emulator.Close writes an unsynchronized
		// flag while pumpInput is blocked in Emulator.Read, which the race
		// detector flags. The pipe is reclaimed when the process exits.
	})
}

// pumpInput copies keystrokes from the emulator's input pipe to the PTY.
// The emulator writes that pipe from SendKey/SendText/Paste, including
// replies to device queries, so this goroutine has to stay running.
func (s *Session) pumpInput() {
	buf := make([]byte, 32*1024)
	for {
		n, err := s.Emu.Read(buf)
		if n > 0 {
			b := make([]byte, n)
			copy(b, buf[:n])
			s.in.Enqueue(b)
		}
		if err != nil {
			return
		}
	}
}

// asyncWriter copies bytes onto the PTY without blocking the UI thread.
type asyncWriter struct {
	mu     sync.Mutex
	cond   *sync.Cond
	buf    []byte
	file   *os.File
	closed bool
}

func newAsyncWriter(f *os.File) *asyncWriter {
	w := &asyncWriter{file: f}
	w.cond = sync.NewCond(&w.mu)
	return w
}

func (w *asyncWriter) Enqueue(p []byte) {
	if len(p) == 0 {
		return
	}
	w.mu.Lock()
	w.buf = append(w.buf, p...)
	w.cond.Signal()
	w.mu.Unlock()
}

func (w *asyncWriter) Close() {
	w.mu.Lock()
	w.closed = true
	w.cond.Signal()
	w.mu.Unlock()
}

func (w *asyncWriter) loop() {
	var pending []byte
	for {
		w.mu.Lock()
		for len(w.buf) == 0 && !w.closed {
			w.cond.Wait()
		}
		pending = append(pending[:0], w.buf...)
		w.buf = w.buf[:0]
		closed := w.closed
		w.mu.Unlock()
		if len(pending) == 0 {
			if closed {
				return
			}
			continue
		}
		if _, err := w.file.Write(pending); err != nil {
			return
		}
	}
}
