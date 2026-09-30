package ui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
)

func TestFrameBoxGeometry(t *testing.T) {
	got := frameBox("hi", 12, 5, colGreen, "TERM")
	lines := strings.Split(got, "\n")
	if len(lines) != 5 {
		t.Fatalf("lines = %d\n%s", len(lines), stripANSI(got))
	}
	for i, ln := range lines {
		if w := lipgloss.Width(ln); w != 12 {
			t.Fatalf("line %d width %d: %q", i, w, stripANSI(ln))
		}
	}
	plain := stripANSI(got)
	if !strings.Contains(plain, "TERM") || !strings.Contains(plain, "hi") {
		t.Fatalf("missing content:\n%s", plain)
	}
}

func TestFitPlain(t *testing.T) {
	if got := fitPlain("abc", 6); got != "abc   " {
		t.Fatalf("%q", got)
	}
	got := truncPlain("abcdef", 4)
	if lipgloss.Width(got) != 4 {
		t.Fatalf("width %d %q", lipgloss.Width(got), got)
	}
	if !strings.HasSuffix(got, "…") {
		t.Fatalf("%q", got)
	}
}

func TestLayoutFillsScreen(t *testing.T) {
	l := computeLayout(100, 32, true)
	if l.tooSmall || !l.showRight {
		t.Fatalf("%+v", l)
	}
	if l.termOuterW+l.rightW != 100 {
		t.Fatalf("width %d+%d", l.termOuterW, l.rightW)
	}
	if l.fileOuterH+l.sysOuterH != 30 {
		t.Fatalf("body %d+%d", l.fileOuterH, l.sysOuterH)
	}
	narrow := computeLayout(60, 20, true)
	if narrow.showRight {
		t.Fatalf("narrow screen should drop the panel: %+v", narrow)
	}
	if computeLayout(20, 5, true).tooSmall != true {
		t.Fatal("expected too small")
	}
}
