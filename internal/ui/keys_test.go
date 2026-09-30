package ui

import (
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/vt"
)

func TestKeyEvent(t *testing.T) {
	ev, ok := keyEvent(tea.KeyMsg{Type: tea.KeyCtrlC})
	if !ok || ev.Code != 'c' || ev.Mod&vt.ModCtrl == 0 {
		t.Fatalf("ctrl+c: %+v ok=%v", ev, ok)
	}
	ev, ok = keyEvent(tea.KeyMsg{Type: tea.KeyEnter})
	if !ok || ev.Code != vt.KeyEnter {
		t.Fatalf("enter: %+v", ev)
	}
	ev, ok = keyEvent(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}, Alt: true})
	if !ok || ev.Code != 'a' || ev.Mod&vt.ModAlt == 0 {
		t.Fatalf("alt+a: %+v", ev)
	}
	if _, ok := keyEvent(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a', 'b'}}); ok {
		t.Fatal("multi-rune should not map to one key")
	}
}

func TestKeyBytes(t *testing.T) {
	emu := vt.NewEmulator(8, 2)
	got := make(chan []byte, 1)
	go func() {
		buf := make([]byte, 8)
		n, _ := emu.Read(buf)
		got <- append([]byte(nil), buf[:n]...)
	}()
	ev, ok := keyEvent(tea.KeyMsg{Type: tea.KeyCtrlC})
	if !ok {
		t.Fatal("map")
	}
	emu.SendKey(ev)
	select {
	case b := <-got:
		if string(b) != "\x03" {
			t.Fatalf("ctrl+c bytes = %q", b)
		}
	case <-time.After(time.Second):
		t.Fatal("timeout")
	}
}
