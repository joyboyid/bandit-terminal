package ui

import (
	"strings"

	uv "github.com/charmbracelet/ultraviolet"
	"github.com/charmbracelet/x/vt"
)

// termPlain returns scrollback followed by the live screen, oldest first.
func termPlain(emu *vt.Emulator) []string {
	if emu == nil {
		return nil
	}
	h := emu.Height()
	sb := emu.Scrollback()
	n := 0
	if sb != nil {
		n = sb.Len()
	}
	out := make([]string, 0, n+h)
	for i := 0; i < n; i++ {
		out = append(out, linePlain(sb.Line(i)))
	}
	for y := 0; y < h; y++ {
		out = append(out, linePlain(screenLine(emu, y)))
	}
	return out
}

func linePlain(line uv.Line) string {
	var b strings.Builder
	for _, c := range line {
		if c.Width == 0 {
			continue
		}
		if c.Content == "" {
			b.WriteByte(' ')
			continue
		}
		b.WriteString(c.Content)
	}
	return strings.TrimRight(b.String(), " ")
}

func matchLines(lines []string, query string) []int {
	q := strings.ToLower(query)
	if q == "" {
		return nil
	}
	var hits []int
	for i, ln := range lines {
		if strings.Contains(strings.ToLower(ln), q) {
			hits = append(hits, i)
		}
	}
	return hits
}

func (m *Model) startFind() {
	m.findOn = true
	m.findQuery = ""
	m.findSaved = m.scroll
	m.findHit = -1
}

func (m *Model) cancelFind() {
	if m.findSaved >= 0 && (m.findOn || m.findHit >= 0) {
		m.scroll = m.findSaved
	}
	m.findOn = false
	m.findQuery = ""
	m.findHit = -1
}

func (m *Model) jumpFind() {
	if m.session == nil || m.session.Emu == nil || m.findQuery == "" {
		m.findHit = -1
		if m.findSaved >= 0 {
			m.scroll = m.findSaved
		}
		return
	}
	hits := matchLines(termPlain(m.session.Emu), m.findQuery)
	if len(hits) == 0 {
		m.findHit = -1
		return
	}
	m.findHit = hits[len(hits)-1]
	m.scrollToLine(m.findHit)
}

func (m *Model) stepFind(dir int) {
	if m.session == nil || m.session.Emu == nil || m.findQuery == "" {
		return
	}
	hits := matchLines(termPlain(m.session.Emu), m.findQuery)
	if len(hits) == 0 {
		m.findHit = -1
		return
	}
	idx := len(hits) - 1
	for i, h := range hits {
		if h == m.findHit {
			idx = i
			break
		}
	}
	idx += dir
	if idx < 0 {
		idx = 0
	}
	if idx >= len(hits) {
		idx = len(hits) - 1
	}
	m.findHit = hits[idx]
	m.scrollToLine(m.findHit)
}

func (m *Model) scrollToLine(i int) {
	if m.session == nil || m.session.Emu == nil {
		return
	}
	scroll := m.session.Emu.ScrollbackLen() - i
	if scroll < 0 {
		scroll = 0
	}
	m.scroll = scroll
	m.clampScroll()
}

func (m *Model) findCount() (at, n int) {
	if m.session == nil || m.findQuery == "" {
		return 0, 0
	}
	hits := matchLines(termPlain(m.session.Emu), m.findQuery)
	n = len(hits)
	for i, h := range hits {
		if h == m.findHit {
			return i + 1, n
		}
	}
	return 0, n
}
