package ui

import (
	"strings"

	uv "github.com/charmbracelet/ultraviolet"
	"github.com/charmbracelet/x/vt"
)

func renderTerm(emu *vt.Emulator, scroll int, cursor bool) string {
	if emu == nil {
		return ""
	}
	if scroll <= 0 || emu.IsAltScreen() {
		return renderLive(emu, cursor)
	}
	h := emu.Height()
	if h <= 0 {
		return ""
	}
	sb := emu.Scrollback()
	sbLen := 0
	if sb != nil {
		sbLen = sb.Len()
	}
	if scroll > sbLen {
		scroll = sbLen
	}
	start := sbLen - scroll
	if start < 0 {
		start = 0
	}
	lines := make([]string, h)
	for row := 0; row < h; row++ {
		idx := start + row
		var rendered string
		if idx < sbLen && sb != nil {
			if line := sb.Line(idx); line != nil {
				rendered = line.Render()
			}
		} else {
			y := idx - sbLen
			if y >= 0 && y < h {
				rendered = screenLine(emu, y).Render()
			}
		}
		lines[row] = rendered
	}
	return strings.Join(lines, "\n")
}

func renderLive(emu *vt.Emulator, cursor bool) (out string) {
	if !cursor {
		return emu.Render()
	}
	pos := emu.CursorPosition()
	cell := emu.CellAt(pos.X, pos.Y)
	if cell == nil {
		return emu.Render()
	}
	saved := *cell
	next := saved
	if next.Content == "" || next.Width == 0 {
		next.Content = " "
		next.Width = 1
	}
	next.Style.Attrs |= uv.AttrReverse
	emu.SetCell(pos.X, pos.Y, &next)
	defer emu.SetCell(pos.X, pos.Y, &saved)
	out = emu.Render()
	return out
}

func screenLine(emu *vt.Emulator, y int) uv.Line {
	w := emu.Width()
	line := make(uv.Line, 0, w)
	for x := 0; x < w; {
		c := emu.CellAt(x, y)
		if c == nil || c.Width == 0 {
			if c == nil {
				line = append(line, uv.EmptyCell)
			}
			x++
			continue
		}
		cp := *c
		if cp.Width < 1 {
			cp.Width = 1
		}
		line = append(line, cp)
		x += cp.Width
	}
	return line
}
