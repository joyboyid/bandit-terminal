package ui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/vt"

	"bandit-terminal/internal/files"
	"bandit-terminal/internal/shell"
	"bandit-terminal/internal/sysinfo"
)

func TestViewGeometry(t *testing.T) {
	for _, show := range []bool{true, false} {
		m := sampleModel(t, show)
		view := m.View()
		lines := strings.Split(view, "\n")
		if len(lines) != m.height {
			t.Fatalf("showRight=%v lines=%d want %d\n%s", show, len(lines), m.height, stripANSI(view))
		}
		for i, ln := range lines {
			if w := lipgloss.Width(ln); w != m.width {
				t.Fatalf("showRight=%v line %d width %d\n%s", show, i, w, stripANSI(view))
			}
		}
		plain := stripANSI(view)
		for _, needle := range []string{"BANDIT", "alpha.go", "CPU", "FILES", "SYSTEM"} {
			if show || (needle != "alpha.go" && needle != "FILES" && needle != "SYSTEM" && needle != "CPU") {
				if !strings.Contains(plain, needle) {
					t.Fatalf("showRight=%v missing %q\n%s", show, needle, plain)
				}
			}
		}
	}
}

func TestIPToggleHidesAddress(t *testing.T) {
	const secret = "203.0.113.10"
	m := sampleModel(t, true)
	m.stats.Addrs = []sysinfo.Addr{
		{Iface: "wlan0", IP: secret, IPv4: true},
		{Iface: "docker0", IP: "172.17.0.1", IPv4: true},
	}
	plain := stripANSI(m.View())
	if !strings.Contains(plain, secret) {
		t.Fatalf("address missing:\n%s", plain)
	}
	if !strings.Contains(plain, "wlan0") || !strings.Contains(plain, "+1") {
		t.Fatalf("iface or extra count missing:\n%s", plain)
	}
	m.Update(tea.KeyMsg{Type: tea.KeyF1})
	if _, cmd := m.Update(tea.KeyMsg{Type: tea.KeyF3}); cmd != nil {
		t.Fatal(cmd)
	}
	if m.showIP {
		t.Fatal("F3 should hide the address")
	}
	if m.focus != focusFiles {
		t.Fatal("F3 should leave the file list focused")
	}
	hidden := stripANSI(m.View())
	if strings.Contains(hidden, secret) || strings.Contains(hidden, "172.17.0.1") {
		t.Fatalf("address still visible:\n%s", hidden)
	}
	if !strings.Contains(hidden, "hidden") {
		t.Fatalf("hidden marker missing:\n%s", hidden)
	}
	lines := strings.Split(m.View(), "\n")
	if len(lines) != m.height {
		t.Fatalf("lines %d", len(lines))
	}
	for i, ln := range lines {
		if w := lipgloss.Width(ln); w != m.width {
			t.Fatalf("line %d width %d", i, w)
		}
	}
	m.Update(tea.KeyMsg{Type: tea.KeyF3})
	if !m.showIP || !strings.Contains(stripANSI(m.View()), secret) {
		t.Fatal("F3 should show the address again")
	}
}

func TestPrivacyAndGit(t *testing.T) {
	m := sampleModel(t, true)
	m.user = "mandex"
	m.home = "/home/mandex"
	m.shellCwd = "/home/mandex/src"
	m.files.dir = m.shellCwd
	m.gitDir = m.shellCwd
	m.gitBranch = "main"
	m.gitDirty = true
	m.stats.Addrs = []sysinfo.Addr{{Iface: "wlan0", IP: "203.0.113.10", IPv4: true}}
	m.stats.Power = sysinfo.Power{HasBattery: true, Percent: 72, Status: "Discharging", HasTemp: true, TempC: 61}

	plain := stripANSI(m.View())
	for _, needle := range []string{"main*", "203.0.113.10", "wlan0", "BAT", "72%", "61C"} {
		if !strings.Contains(plain, needle) {
			t.Fatalf("missing %q\n%s", needle, plain)
		}
	}
	m.Update(tea.KeyMsg{Type: tea.KeyF4})
	if !m.privacy {
		t.Fatal("F4 should enable privacy")
	}
	hidden := stripANSI(m.View())
	if strings.Contains(hidden, "mandex") || strings.Contains(hidden, "203.0.113.10") {
		t.Fatalf("identity leaked:\n%s", hidden)
	}
	if !strings.Contains(hidden, "••••") || !strings.Contains(hidden, "~") || !strings.Contains(hidden, "main*") {
		t.Fatalf("mask or branch missing:\n%s", hidden)
	}
	assertGeometry(t, m)
	m.Update(tea.KeyMsg{Type: tea.KeyF4})
	if m.privacy || !strings.Contains(stripANSI(m.View()), "mandex") {
		t.Fatal("F4 should show the identity again")
	}
}

func assertGeometry(t *testing.T, m *Model) {
	t.Helper()
	view := m.View()
	lines := strings.Split(view, "\n")
	if len(lines) != m.height {
		t.Fatalf("lines %d want %d", len(lines), m.height)
	}
	for i, ln := range lines {
		if w := lipgloss.Width(ln); w != m.width {
			t.Fatalf("line %d width %d", i, w)
		}
	}
}

func TestViewTooSmall(t *testing.T) {
	m := New()
	m.width = 20
	m.height = 8
	plain := stripANSI(m.View())
	if !strings.Contains(plain, "too small") {
		t.Fatalf("%s", plain)
	}
	lines := strings.Split(m.View(), "\n")
	if len(lines) != 8 {
		t.Fatalf("lines %d", len(lines))
	}
}

func TestFocusAndQuit(t *testing.T) {
	m := sampleModel(t, true)
	if _, cmd := m.Update(tea.KeyMsg{Type: tea.KeyF1}); cmd != nil {
		t.Fatal("F1 should not return a command")
	}
	if m.focus != focusFiles {
		t.Fatal("expected file focus")
	}
	if _, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEsc}); cmd != nil {
		t.Fatal(cmd)
	}
	if m.focus != focusTerm {
		t.Fatal("expected terminal focus")
	}
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlQ})
	if cmd == nil {
		t.Fatal("expected quit")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Fatalf("quit msg %T", cmd())
	}
}

func TestOpenDirectory(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "alpha.go"), []byte("package a\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	m := New()
	m.files.dir = dir
	m.files.follow = true
	m.shellCwd = dir
	res, err := files.Read(dir, true)
	if err != nil {
		t.Fatal(err)
	}
	m.files.entries = res.Entries
	for i, e := range m.files.entries {
		if e.Name == "sub" {
			m.files.selected = i
		}
	}
	if cmd := m.openSelected(); cmd == nil {
		t.Fatal("expected a list command")
	}
	if m.files.dir != filepath.Join(dir, "sub") {
		t.Fatal(m.files.dir)
	}
	if m.files.follow {
		t.Fatal("browsing away from the shell should leave follow mode")
	}
	msg := m.listNow()().(listedMsg)
	if msg.err != nil {
		t.Fatal(msg.err)
	}
	m.Update(msg)
	if len(m.files.entries) == 0 || m.files.entries[0].Name != ".." {
		t.Fatalf("%+v", m.files.entries)
	}
}

func TestScrollbackShowsEarlierLine(t *testing.T) {
	emu := vt.NewEmulator(20, 5)
	var b strings.Builder
	for i := 0; i < 12; i++ {
		fmt.Fprintf(&b, "line-%02d\r\n", i)
	}
	_, _ = emu.Write([]byte(b.String()))
	live := stripANSI(renderTerm(emu, 0, false))
	if !strings.Contains(live, "line-11") {
		t.Fatalf("live view:\n%s", live)
	}
	if emu.ScrollbackLen() == 0 {
		t.Fatal("expected scrollback")
	}
	older := stripANSI(renderTerm(emu, emu.ScrollbackLen(), false))
	if !strings.Contains(older, "line-00") {
		t.Fatalf("scrolled view:\n%s", older)
	}
}

func sampleModel(t *testing.T, showRight bool) *Model {
	t.Helper()
	m := New()
	m.width = 100
	m.height = 32
	m.ready = true
	m.booted = true
	m.showRight = showRight
	m.user = "mandex"
	m.host = "box"
	m.shellName = "bash"
	m.shellCwd = "/tmp/project"
	m.home = "/home/mandex"
	m.files.dir = "/tmp/project"
	m.files.follow = true
	m.files.entries = []files.Entry{
		{Name: "..", Path: "/tmp", IsDir: true, Mode: os.ModeDir | 0o755},
		{Name: "alpha.go", Path: "/tmp/project/alpha.go", Size: 128, Mode: 0o644},
	}
	m.files.selected = 1
	m.stats = sysinfo.Snapshot{
		CPUPercent: 12,
		MemTotal:   8 << 30,
		MemUsed:    2 << 30,
		Load1:      0.4,
		Uptime:     time.Hour,
		DiskTotal:  100 << 30,
		DiskUsed:   40 << 30,
		Host:       "box",
		User:       "mandex",
	}
	m.session = &shell.Session{Emu: vt.NewEmulator(20, 8)}
	m.applyResize()
	return m
}
