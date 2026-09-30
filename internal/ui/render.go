package ui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

const (
	minW = 48
	minH = 12

	colGreen    = "#3DDC97"
	colGreenHot = "#B8FFD9"
	colDim      = "#1E6B45"
	colInk      = "#D7FBE8"
	colMuted    = "#6F917F"
	colAmber    = "#E7FF6A"
	colCyan     = "#8BE9FF"
	colRed      = "#FF6B81"
	colSel      = "#145C38"
	colBrandFg  = "#042015"
)

type layout struct {
	tooSmall   bool
	showRight  bool
	termX      int
	termY      int
	termOuterW int
	termOuterH int
	termInnerW int
	termInnerH int
	rightW     int
	fileOuterH int
	sysOuterH  int
	fileInnerW int
	fileInnerH int
	sysInnerW  int
	sysInnerH  int
	fileX      int
	fileY      int
	fileW      int
	fileH      int
	sysX       int
	sysY       int
	sysW       int
	sysH       int
	fileHeader int
}

func computeLayout(width, height int, wantRight bool) layout {
	l := layout{}
	if width < minW || height < minH {
		l.tooSmall = true
		return l
	}
	bodyH := height - 2
	l.termX = 0
	l.termY = 1

	right := 0
	if wantRight {
		right = width * 32 / 100
		if right < 28 {
			right = 28
		}
		if right > 40 {
			right = 40
		}
		if width-right < 40 {
			right = width - 40
		}
		if right < 24 {
			right = 0
		}
	}
	l.showRight = right > 0
	l.rightW = right
	l.termOuterW = width - right
	l.termOuterH = bodyH
	l.termInnerW = max(1, l.termOuterW-2)
	l.termInnerH = max(1, bodyH-2)

	if !l.showRight {
		return l
	}

	sysH := 11
	if bodyH < 18 {
		sysH = 9
	}
	if sysH > bodyH-5 {
		sysH = bodyH / 3
	}
	if sysH < 5 {
		sysH = 5
	}
	if sysH >= bodyH {
		sysH = bodyH / 2
	}
	fileH := bodyH - sysH
	l.fileOuterH = fileH
	l.sysOuterH = sysH
	l.fileInnerW = max(1, right-2)
	l.sysInnerW = max(1, right-2)
	l.fileInnerH = max(1, fileH-2)
	l.sysInnerH = max(1, sysH-2)
	l.fileX = l.termOuterW
	l.fileY = 1
	l.fileW = right
	l.fileH = fileH
	l.sysX = l.termOuterW
	l.sysY = 1 + fileH
	l.sysW = right
	l.sysH = sysH
	l.fileHeader = fileHeaderLines(l.fileInnerH)
	return l
}

func fileHeaderLines(innerH int) int {
	switch {
	case innerH >= 5:
		return 2
	case innerH >= 1:
		return 1
	default:
		return 0
	}
}

func paint(s, color string) string {
	return lipgloss.NewStyle().Foreground(lipgloss.Color(color)).Render(s)
}

func paintWidth(text string, w int, st lipgloss.Style) string {
	if w <= 0 {
		return ""
	}
	text = fitPlain(text, w)
	out := st.Width(w).MaxHeight(1).Render(text)
	return padANSI(out, w)
}

func truncPlain(s string, w int) string {
	if w <= 0 {
		return ""
	}
	if lipgloss.Width(s) <= w {
		return s
	}
	if w == 1 {
		return "…"
	}
	target := w - 1
	r := []rune(s)
	for len(r) > 0 && lipgloss.Width(string(r)) > target {
		r = r[:len(r)-1]
	}
	out := string(r) + "…"
	for lipgloss.Width(out) > w && out != "" {
		rr := []rune(out)
		out = string(rr[:len(rr)-1])
	}
	return out
}

func fitPlain(s string, w int) string {
	if w <= 0 {
		return ""
	}
	if lipgloss.Width(s) > w {
		s = truncPlain(s, w)
	}
	if d := w - lipgloss.Width(s); d > 0 {
		s += strings.Repeat(" ", d)
	}
	return s
}

func padANSI(s string, w int) string {
	if w <= 0 {
		return ""
	}
	sw := lipgloss.Width(s)
	switch {
	case sw == w:
		return s
	case sw < w:
		return s + strings.Repeat(" ", w-sw)
	default:
		out := lipgloss.NewStyle().MaxWidth(w).Render(s)
		if extra := lipgloss.Width(out) - w; extra > 0 {
			out = fitPlain(stripANSI(out), w)
		}
		if lipgloss.Width(out) < w {
			out += strings.Repeat(" ", w-lipgloss.Width(out))
		}
		return out
	}
}

func spread(left, right string, w int) string {
	if w <= 0 {
		return ""
	}
	rw := lipgloss.Width(right)
	if rw >= w {
		return fitPlain(right, w)
	}
	return fitPlain(left, w-rw) + right
}

func spreadFill(left, right, fill string, w int) string {
	if w <= 0 {
		return ""
	}
	lw := lipgloss.Width(left)
	rw := lipgloss.Width(right)
	if lw+rw >= w || fill == "" {
		return spread(left, right, w)
	}
	gap := w - lw - rw
	return left + strings.Repeat(fill, gap) + right
}

func centerIn(s string, w int) string {
	if w <= 0 {
		return ""
	}
	if lipgloss.Width(s) >= w {
		return truncPlain(s, w)
	}
	pad := w - lipgloss.Width(s)
	left := pad / 2
	return strings.Repeat(" ", left) + s + strings.Repeat(" ", pad-left)
}

func centerScreen(w, h int, msg string) string {
	if w < 1 {
		w = 1
	}
	if h < 1 {
		h = 1
	}
	parts := strings.Split(msg, "\n")
	top := (h - len(parts)) / 2
	if top < 0 {
		top = 0
	}
	lines := make([]string, h)
	blank := strings.Repeat(" ", w)
	pi := 0
	for i := 0; i < h; i++ {
		if i >= top && pi < len(parts) {
			lines[i] = paint(centerIn(parts[pi], w), colGreen)
			lines[i] = padANSI(lines[i], w)
			pi++
		} else {
			lines[i] = blank
		}
	}
	return strings.Join(lines, "\n")
}

func fitBlock(s string, w, h int) []string {
	if w < 1 {
		w = 1
	}
	if h < 1 {
		h = 1
	}
	raw := strings.Split(s, "\n")
	out := make([]string, h)
	blank := strings.Repeat(" ", w)
	for i := 0; i < h; i++ {
		if i < len(raw) {
			out[i] = padANSI(raw[i], w)
		} else {
			out[i] = blank
		}
	}
	return out
}

func topBorder(w int, title string) string {
	if w < 2 {
		return strings.Repeat("═", max(w, 0))
	}
	label := ""
	if title != "" {
		label = " " + title + " "
		if lipgloss.Width(label) > w-2 {
			label = truncPlain(title, w-2)
		}
	}
	fill := w - 2 - lipgloss.Width(label)
	if fill < 0 {
		fill = 0
	}
	return "╔" + label + strings.Repeat("═", fill) + "╗"
}

func botBorder(w int) string {
	if w < 2 {
		return strings.Repeat("═", max(w, 0))
	}
	return "╚" + strings.Repeat("═", w-2) + "╝"
}

func frameBox(inner string, outerW, outerH int, color, title string) string {
	if outerW < 2 {
		outerW = 2
	}
	if outerH < 2 {
		outerH = 2
	}
	lines := fitBlock(inner, outerW-2, outerH-2)
	side := paint("║", color)
	var b strings.Builder
	b.WriteString(padANSI(paint(topBorder(outerW, title), color), outerW))
	for _, ln := range lines {
		b.WriteByte('\n')
		row := side + ln + side
		b.WriteString(padANSI(row, outerW))
	}
	b.WriteByte('\n')
	b.WriteString(padANSI(paint(botBorder(outerW), color), outerW))
	return b.String()
}

func joinH(left, right string, lw, rw, h int) string {
	ll := fitBlock(left, lw, h)
	rr := fitBlock(right, rw, h)
	var b strings.Builder
	for i := 0; i < h; i++ {
		if i > 0 {
			b.WriteByte('\n')
		}
		b.WriteString(padANSI(ll[i]+rr[i], lw+rw))
	}
	return b.String()
}

func joinV(top, bottom string, w, topH, botH int) string {
	a := fitBlock(top, w, topH)
	b := fitBlock(bottom, w, botH)
	return strings.Join(append(a, b...), "\n")
}

func stack(parts ...string) string {
	var b strings.Builder
	first := true
	for _, p := range parts {
		p = strings.TrimRight(p, "\n")
		if p == "" {
			continue
		}
		if !first {
			b.WriteByte('\n')
		}
		first = false
		b.WriteString(p)
	}
	return b.String()
}

func stripANSI(s string) string {
	var b strings.Builder
	b.Grow(len(s))
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
			for i < len(s) && !isFinal(s[i]) {
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

func isFinal(c byte) bool {
	return c >= 0x40 && c <= 0x7e
}
