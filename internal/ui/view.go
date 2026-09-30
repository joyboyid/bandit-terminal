package ui

import (
	"fmt"
	"math"
	"os/exec"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"

	"bandit-terminal/internal/files"
	"bandit-terminal/internal/format"
	"bandit-terminal/internal/sysinfo"
)

func (m *Model) View() string {
	if m.width <= 0 || m.height <= 0 {
		return "BANDIT"
	}
	if m.startErr != nil {
		return centerScreen(m.width, m.height, "BANDIT\nshell failed to start\n"+m.startErr.Error()+"\n\npress q")
	}
	m.lay = computeLayout(m.width, m.height, m.showRight)
	if m.lay.tooSmall {
		return centerScreen(m.width, m.height, "BANDIT\nterminal too small\nneed at least 48×12")
	}
	m.files.ensure(max(1, m.lay.fileInnerH-m.lay.fileHeader))

	bodyH := m.height - 2
	termTitle, termBody := m.termInner()
	var body string
	if !m.lay.showRight {
		body = frameBox(termBody, m.width, bodyH, m.termBorder(), termTitle)
	} else {
		fileTitle, fileBody := m.fileInner()
		sysTitle, sysBody := m.sysInner()
		left := frameBox(termBody, m.lay.termOuterW, bodyH, m.termBorder(), termTitle)
		fileBox := frameBox(fileBody, m.lay.rightW, m.lay.fileOuterH, m.fileBorder(), fileTitle)
		sysBox := frameBox(sysBody, m.lay.rightW, m.lay.sysOuterH, colDim, sysTitle)
		right := joinV(fileBox, sysBox, m.lay.rightW, m.lay.fileOuterH, m.lay.sysOuterH)
		body = joinH(left, right, m.lay.termOuterW, m.lay.rightW, bodyH)
	}
	return stack(m.headerLine(), body, m.footerLine())
}

func (m *Model) termBorder() string {
	if m.focus == focusTerm {
		return colGreen
	}
	return colDim
}

func (m *Model) fileBorder() string {
	if m.focus == focusFiles {
		return colGreen
	}
	return colDim
}

func (m *Model) headerLine() string {
	w := m.width
	brand := lipgloss.NewStyle().
		Background(lipgloss.Color("#F07818")).
		Foreground(lipgloss.Color("#140C08")).
		Bold(true).
		Render(" BANDIT ")
	bw := lipgloss.Width(brand)
	if bw >= w {
		return padANSI(brand, w)
	}
	clock := time.Now().Format("15:04:05")
	rest := spreadFill(m.headerMeta(), clock, "─", w-bw)
	return padANSI(brand+paint(rest, colInk), w)
}

func (m *Model) headerMeta() string {
	user := m.user
	if m.privacy {
		user = "••••"
	}
	meta := fmt.Sprintf(" %s@%s", user, m.host)
	if m.addrsVisible() {
		if ip := primaryIP(m.stats.Addrs); ip != "" {
			meta += " · " + ip
		}
	}
	meta += " · " + m.shellName + " · " + format.PrettyPath(m.shellCwd, m.home)
	if m.gitBranch != "" && m.gitDir == m.shellCwd {
		b := m.gitBranch
		if m.gitDirty {
			b += "*"
		}
		meta += " · " + b
	}
	if m.privacy {
		meta = redactUser(meta, m.user)
	}
	return meta
}

func (m *Model) addrsVisible() bool {
	return m.showIP && !m.privacy
}

func redactUser(s, user string) string {
	if len(user) < 2 || user == "••••" {
		return s
	}
	return strings.ReplaceAll(s, user, "••••")
}

func (m *Model) footerLine() string {
	w := m.width
	var left, right string
	switch {
	case m.findOn:
		at, n := m.findCount()
		left = " /" + m.findQuery
		switch {
		case m.findQuery == "":
			right = "type  ↑↓ match  esc"
		case n == 0:
			right = "no match  esc"
		default:
			right = fmt.Sprintf("%d/%d  ↑↓  enter  esc", at, n)
		}
	case m.dead:
		left = " shell ended — q quit "
		if ee, ok := m.deadErr.(*exec.ExitError); ok && ee.ExitCode() != 0 {
			left = fmt.Sprintf(" shell exited %d — q quit ", ee.ExitCode())
		}
	case m.focus == focusFiles:
		left = " ↑↓ move  enter open  c cd  p paste  . hidden  s sync  esc back "
	default:
		left = " F1 files  F2 panel  F3 ip  F4 priv  / find  ^Q quit "
	}
	if m.privacy && !m.findOn && !m.dead {
		left = " PRIV  " + strings.TrimLeft(left, " ")
	}
	if !m.findOn {
		if m.scroll > 0 {
			right = fmt.Sprintf("SCROLL %d", m.scroll)
		} else if m.stats.MemTotal > 0 {
			mem := float64(m.stats.MemUsed) / float64(m.stats.MemTotal) * 100
			right = fmt.Sprintf("CPU %3.0f%%  MEM %3.0f%%", m.stats.CPUPercent, mem)
		}
	}
	color := colMuted
	if m.dead {
		color = colAmber
	}
	return padANSI(paint(spread(strings.TrimRight(left, " "), right, w), color), w)
}

func (m *Model) termInner() (title, body string) {
	title = "TERMINAL"
	if m.shellName != "" {
		title = "TERMINAL · " + m.shellName
	}
	if m.title != "" {
		title = m.title
	}
	w, h := m.lay.termInnerW, m.lay.termInnerH
	if m.session == nil || m.session.Emu == nil {
		return title, centerBlock("starting shell…", w, h)
	}
	showCursor := m.focus == focusTerm && !m.cursorHidden && m.blinkOn && m.scroll == 0 && !m.dead
	mark := -1
	if m.findHit >= 0 && m.findQuery != "" {
		mark = m.findHit
	}
	return title, renderTerm(m.session.Emu, m.scroll, showCursor, mark)
}

func centerBlock(msg string, w, h int) string {
	if w < 1 || h < 1 {
		return ""
	}
	lines := make([]string, h)
	blank := strings.Repeat(" ", w)
	for i := 0; i < h; i++ {
		if i == h/2 {
			lines[i] = centerIn(msg, w)
		} else {
			lines[i] = blank
		}
	}
	return strings.Join(lines, "\n")
}

func (m *Model) fileInner() (title, body string) {
	w, h := m.lay.fileInnerW, m.lay.fileInnerH
	n := len(m.files.entries)
	title = "FILES"
	if n > 0 {
		title = fmt.Sprintf("FILES %d/%d", m.files.selected+1, n)
	}
	if w < 1 || h < 1 {
		return title, ""
	}
	lines := make([]string, 0, h)
	badge := "BROWSE"
	if m.files.follow {
		badge = "FOLLOW"
	}
	if m.search != "" {
		badge = "/" + m.search
	}
	path := format.PrettyPath(m.files.dir, m.home)
	if m.privacy {
		path = redactUser(path, m.user)
	}
	if m.files.hidden > 0 && !m.files.showHidden {
		path += fmt.Sprintf("  +%d", m.files.hidden)
	}
	if m.files.clipped > 0 {
		path += fmt.Sprintf("  +%d more", m.files.clipped)
	}
	lines = append(lines, paintWidth(spread(path, badge, w), w, lipgloss.NewStyle().Foreground(lipgloss.Color(colGreenHot))))
	if m.lay.fileHeader >= 2 {
		col := "NAME"
		if w >= 30 {
			col = "MODE       SIZE  NAME"
		} else if w >= 16 {
			col = "SIZE  NAME"
		}
		lines = append(lines, paintWidth(fitPlain(col, w), w, lipgloss.NewStyle().Foreground(lipgloss.Color(colMuted))))
	}
	if m.files.err != "" && n == 0 {
		lines = append(lines, paintWidth(fitPlain(m.files.err, w), w, lipgloss.NewStyle().Foreground(lipgloss.Color(colRed))))
		return title, strings.Join(lines, "\n")
	}
	if n == 0 {
		lines = append(lines, paintWidth(fitPlain("empty", w), w, lipgloss.NewStyle().Foreground(lipgloss.Color(colMuted))))
		return title, strings.Join(lines, "\n")
	}
	rows := h - len(lines)
	if rows < 1 {
		return title, strings.Join(lines, "\n")
	}
	m.files.ensure(rows)
	end := m.files.offset + rows
	if end > n {
		end = n
	}
	for i := m.files.offset; i < end; i++ {
		e := m.files.entries[i]
		selected := i == m.files.selected
		focused := selected && m.focus == focusFiles
		row := fileRowText(e, w, selected)
		if m.privacy {
			row = redactUser(row, m.user)
		}
		lines = append(lines, paintWidth(row, w, rowStyle(e, focused)))
	}
	return title, strings.Join(lines, "\n")
}

func fileRowText(e files.Entry, w int, selected bool) string {
	mark := " "
	if selected {
		mark = "▸"
	}
	label := e.Label()
	if w < 16 {
		return fitPlain(mark+" "+label, w)
	}
	size := e.SizeLabel()
	if len(size) > 6 {
		size = size[:6]
	}
	if w < 30 {
		return fitPlain(fmt.Sprintf("%s %6s %s", mark, size, label), w)
	}
	mode := e.Mode.String()
	if mode == "" {
		mode = "----------"
	}
	if len(mode) > 10 {
		mode = mode[:10]
	}
	return fitPlain(fmt.Sprintf("%s %-10s %6s %s", mark, mode, size, label), w)
}

func rowStyle(e files.Entry, selected bool) lipgloss.Style {
	if selected {
		return lipgloss.NewStyle().
			Foreground(lipgloss.Color("#F3FFF8")).
			Background(lipgloss.Color(colSel)).
			Bold(true)
	}
	c := colInk
	switch {
	case e.Name != ".." && strings.HasPrefix(e.Name, "."):
		c = colMuted
	case e.IsDir:
		c = colGreen
	case e.IsLink:
		c = colCyan
	case e.Mode&0o111 != 0:
		c = colAmber
	}
	st := lipgloss.NewStyle().Foreground(lipgloss.Color(c))
	if e.IsDir {
		st = st.Bold(true)
	}
	return st
}

func (m *Model) sysInner() (title, body string) {
	title = "SYSTEM"
	w, h := m.lay.sysInnerW, m.lay.sysInnerH
	if w < 1 || h < 1 {
		return title, ""
	}
	s := m.stats
	lines := make([]string, 0, h)
	if s.MemTotal == 0 {
		addPlain(&lines, "sampling…", w, h, colMuted)
	} else {
		addGauge(&lines, "CPU", s.CPUPercent, w, h)
		addGauge(&lines, "MEM", percent(s.MemUsed, s.MemTotal), w, h)
		addPlain(&lines, "    "+format.HumanBytes(s.MemUsed)+"/"+format.HumanBytes(s.MemTotal), w, h, colMuted)
		addPlain(&lines, fmt.Sprintf("LOAD %.2f %.2f %.2f", s.Load1, s.Load5, s.Load15), w, h, colInk)
		addPlain(&lines, fmt.Sprintf("NET  ↓%s  ↑%s", format.HumanBytes(uint64(s.NetRx)), format.HumanBytes(uint64(s.NetTx))), w, h, colCyan)
		ipText, ipColor := ipStatus(s.Addrs, m.addrsVisible())
		addPlain(&lines, ipText, w, h, ipColor)
		addPlain(&lines, "UP   "+format.Uptime(s.Uptime), w, h, colInk)
		if s.DiskTotal > 0 {
			addGauge(&lines, "DSK", percent(s.DiskUsed, s.DiskTotal), w, h)
		}
		if text, ok := powerText(s.Power); ok {
			addPlain(&lines, text, w, h, powerColor(s.Power))
		}
		for _, p := range s.Procs {
			addPlain(&lines, fmt.Sprintf("%3.0f%%  %s", p.CPU, p.Name), w, h, colMuted)
		}
	}
	return title, strings.Join(lines, "\n")
}

func primaryIP(addrs []sysinfo.Addr) string {
	if len(addrs) == 0 {
		return ""
	}
	return addrs[0].IP
}

// ipStatus is the single system-panel line for this machine's addresses.
// A hidden address is replaced entirely so it cannot leak into a screenshot.
func ipStatus(addrs []sysinfo.Addr, show bool) (string, string) {
	if !show {
		return "IP   hidden", colMuted
	}
	if len(addrs) == 0 {
		return "IP   none", colMuted
	}
	a := addrs[0]
	line := "IP " + a.IP
	if a.Iface != "" {
		line += "  " + a.Iface
	}
	if extra := len(addrs) - 1; extra > 0 {
		line += fmt.Sprintf(" +%d", extra)
	}
	return line, colCyan
}

func powerText(p sysinfo.Power) (string, bool) {
	if !p.HasBattery && !p.HasTemp {
		return "", false
	}
	var b strings.Builder
	if p.HasBattery {
		fmt.Fprintf(&b, "BAT %3d%% %s", p.Percent, shortStatus(p.Status))
	}
	if p.HasTemp {
		if b.Len() > 0 {
			b.WriteString("  ")
		}
		fmt.Fprintf(&b, "%dC", int(math.Round(p.TempC)))
	}
	return b.String(), true
}

func shortStatus(s string) string {
	switch strings.ToLower(s) {
	case "charging":
		return "chg"
	case "discharging":
		return "dis"
	case "full":
		return "full"
	case "not charging":
		return "idle"
	default:
		if len(s) > 6 {
			s = s[:6]
		}
		return strings.ToLower(s)
	}
}

func powerColor(p sysinfo.Power) string {
	if (p.HasBattery && p.Percent < 15) || (p.HasTemp && p.TempC >= 90) {
		return colRed
	}
	if (p.HasBattery && p.Percent < 30) || (p.HasTemp && p.TempC >= 80) {
		return colAmber
	}
	return colGreen
}

func addPlain(lines *[]string, text string, w, h int, color string) {
	if len(*lines) >= h {
		return
	}
	*lines = append(*lines, paintWidth(text, w, lipgloss.NewStyle().Foreground(lipgloss.Color(color))))
}

func addGauge(lines *[]string, label string, pct float64, w, h int) {
	if len(*lines) >= h {
		return
	}
	*lines = append(*lines, paintWidth(gauge(label, pct, w), w, gaugeStyle(pct)))
}

func gauge(label string, pct float64, w int) string {
	if pct < 0 {
		pct = 0
	}
	if pct > 100 {
		pct = 100
	}
	pctStr := fmt.Sprintf("%3.0f%%", pct)
	barW := w - len(label) - 1 - 1 - len(pctStr)
	if barW < 1 {
		return fitPlain(label+" "+pctStr, w)
	}
	return fitPlain(label+" "+bar(pct, barW)+" "+pctStr, w)
}

func bar(pct float64, w int) string {
	if w <= 0 {
		return ""
	}
	filled := int(math.Round(pct / 100 * float64(w)))
	if filled > w {
		filled = w
	}
	if filled < 0 {
		filled = 0
	}
	return strings.Repeat("█", filled) + strings.Repeat("░", w-filled)
}

func gaugeStyle(pct float64) lipgloss.Style {
	c := colGreen
	switch {
	case pct >= 90:
		c = colRed
	case pct >= 70:
		c = colAmber
	}
	return lipgloss.NewStyle().Foreground(lipgloss.Color(c))
}

func percent(used, total uint64) float64 {
	if total == 0 {
		return 0
	}
	return float64(used) / float64(total) * 100
}
