package ui

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
	"unicode"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/vt"

	"bandit-terminal/internal/files"
	"bandit-terminal/internal/format"
	"bandit-terminal/internal/gitinfo"
	"bandit-terminal/internal/shell"
	"bandit-terminal/internal/sysinfo"
)

type focus int

const (
	focusTerm focus = iota
	focusFiles
)

type fileState struct {
	dir        string
	entries    []files.Entry
	selected   int
	offset     int
	showHidden bool
	follow     bool
	hidden     int
	clipped    int
	err        string
}

func (f *fileState) current() (files.Entry, bool) {
	if f.selected < 0 || f.selected >= len(f.entries) {
		return files.Entry{}, false
	}
	return f.entries[f.selected], true
}

func (f *fileState) ensure(rows int) {
	if rows < 1 {
		rows = 1
	}
	n := len(f.entries)
	if n == 0 {
		f.selected = 0
		f.offset = 0
		return
	}
	if f.selected < 0 {
		f.selected = 0
	}
	if f.selected >= n {
		f.selected = n - 1
	}
	if f.selected < f.offset {
		f.offset = f.selected
	}
	if f.selected >= f.offset+rows {
		f.offset = f.selected - rows + 1
	}
	maxOff := n - rows
	if maxOff < 0 {
		maxOff = 0
	}
	if f.offset > maxOff {
		f.offset = maxOff
	}
	if f.offset < 0 {
		f.offset = 0
	}
}

func (f *fileState) move(d, rows int) {
	if len(f.entries) == 0 {
		return
	}
	f.selected += d
	f.ensure(rows)
}

func (f *fileState) jump(prefix string) {
	p := strings.ToLower(prefix)
	for i, e := range f.entries {
		if e.Name == ".." {
			continue
		}
		if strings.HasPrefix(strings.ToLower(e.Name), p) {
			f.selected = i
			return
		}
	}
}

// Model is the bandit terminal screen.
type Model struct {
	width, height int
	showRight     bool
	showIP        bool
	privacy       bool
	focus         focus
	session       *shell.Session
	shellCwd      string
	home          string
	files         fileState
	stats         sysinfo.Snapshot
	sampler       *sysinfo.Sampler
	title         string
	cursorHidden  bool
	blinkOn       bool
	scroll        int
	dead          bool
	deadErr       error
	startErr      error
	search        string
	searchGen     int
	listGen       int
	ready         bool
	booted        bool
	user          string
	host          string
	shellName     string
	lay           layout
	gitBranch     string
	gitDirty      bool
	gitDir        string
}

// New builds the screen. The shell starts when the program runs.
func New() *Model {
	home, _ := os.UserHomeDir()
	host, _ := os.Hostname()
	if i := strings.IndexByte(host, '.'); i > 0 {
		host = host[:i]
	}
	user := os.Getenv("USER")
	if user == "" {
		user = "user"
	}
	sh := shell.Resolve()
	wd, err := os.Getwd()
	if err != nil || wd == "" {
		wd = home
	}
	return &Model{
		showRight: true,
		showIP:    true,
		blinkOn:   true,
		focus:     focusTerm,
		sampler:   sysinfo.New(),
		home:      home,
		host:      host,
		user:      user,
		shellName: filepath.Base(sh),
		shellCwd:  wd,
		files: fileState{
			dir:        wd,
			follow:     true,
			showHidden: true,
		},
	}
}

// Shutdown stops the shell. Call it after the program returns.
func (m *Model) Shutdown() {
	if m != nil && m.session != nil {
		m.session.Close()
	}
}

func (m *Model) Init() tea.Cmd {
	return tea.Batch(startShell(), primeStats(m.sampler), scheduleBlink(), probeGit(m.shellCwd), scheduleGit())
}

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.ready = true
		m.applyResize()
		return m, nil
	case tea.KeyMsg:
		return m, m.onKey(msg)
	case tea.MouseMsg:
		return m, m.onMouse(msg)
	case startedMsg:
		if msg.err != nil {
			m.startErr = msg.err
			return m, nil
		}
		m.session = msg.session
		m.booted = true
		m.shellCwd = msg.session.Dir
		if m.files.dir == "" {
			m.files.dir = msg.session.Dir
		}
		m.files.follow = true
		m.hookCallbacks()
		m.applyResize()
		return m, tea.Batch(
			readPty(m.session.PTY),
			waitExit(m.session.Cmd),
			m.listNow(),
			scheduleCwd(),
		)
	case ptyMsg:
		if m.session != nil && len(msg.data) > 0 {
			_, _ = m.session.Emu.Write(msg.data)
			m.clampScroll()
		}
		if msg.err != nil || m.session == nil || m.dead {
			return m, nil
		}
		return m, readPty(m.session.PTY)
	case exitMsg:
		m.dead = true
		m.deadErr = msg.err
		m.focus = focusTerm
		return m, nil
	case statsMsg:
		m.stats = msg.snap
		cmds := []tea.Cmd{scheduleStats(m.sampler)}
		if m.booted && !m.dead {
			cmds = append(cmds, m.listNow())
		}
		return m, tea.Batch(cmds...)
	case blinkMsg:
		m.blinkOn = !m.blinkOn
		return m, scheduleBlink()
	case cwdTickMsg:
		return m, m.onCwdTick()
	case listedMsg:
		if msg.gen != m.listGen || msg.dir != m.files.dir {
			return m, nil
		}
		prev := ""
		if e, ok := m.files.current(); ok {
			prev = e.Name
		}
		m.files.entries = msg.entries
		m.files.hidden = msg.hidden
		m.files.clipped = msg.clipped
		if msg.err != nil {
			m.files.err = msg.err.Error()
		} else {
			m.files.err = ""
		}
		m.files.selected = 0
		if prev != "" {
			for i, e := range msg.entries {
				if e.Name == prev {
					m.files.selected = i
					break
				}
			}
		}
		m.files.ensure(m.fileRows())
		return m, nil
	case searchResetMsg:
		if msg.gen == m.searchGen {
			m.search = ""
		}
		return m, nil
	case gitMsg:
		if msg.dir == m.shellCwd {
			m.gitDir = msg.dir
			m.gitBranch = msg.branch
			m.gitDirty = msg.dirty
		}
		return m, nil
	case gitTickMsg:
		return m, tea.Batch(probeGit(m.shellCwd), scheduleGit())
	}
	return m, nil
}

func (m *Model) onKey(k tea.KeyMsg) tea.Cmd {
	if m.startErr != nil || (m.dead && (k.String() == "q" || k.String() == "enter" || k.String() == "esc" || k.String() == "ctrl+q")) {
		return tea.Quit
	}
	if k.String() == "ctrl+q" {
		return tea.Quit
	}
	if m.dead || !m.booted || m.session == nil {
		return nil
	}
	switch k.String() {
	case "f4":
		m.privacy = !m.privacy
		return nil
	case "f3":
		m.showIP = !m.showIP
		return nil
	case "f2":
		m.showRight = !m.showRight
		if !m.showRight && m.focus == focusFiles {
			m.focus = focusTerm
		}
		m.applyResize()
		return nil
	case "f1":
		if !m.lay.showRight {
			m.showRight = true
			m.applyResize()
		}
		if !m.lay.showRight {
			return nil
		}
		if m.focus == focusFiles {
			m.focus = focusTerm
			m.search = ""
		} else {
			m.focus = focusFiles
		}
		return nil
	}
	if m.focus == focusFiles && m.lay.showRight {
		return m.onFileKey(k)
	}
	switch k.String() {
	case "ctrl+pgup":
		if m.session.Emu.IsAltScreen() {
			m.forwardKey(k)
			return nil
		}
		m.scrollBy(max(1, m.lay.termInnerH/2))
		return nil
	case "ctrl+pgdown":
		if m.session.Emu.IsAltScreen() {
			m.forwardKey(k)
			return nil
		}
		m.scrollBy(-max(1, m.lay.termInnerH/2))
		return nil
	}
	m.forwardKey(k)
	return nil
}

func (m *Model) onFileKey(k tea.KeyMsg) tea.Cmd {
	rows := m.fileRows()
	switch k.String() {
	case "esc":
		m.focus = focusTerm
		m.search = ""
		return nil
	case "ctrl+c":
		m.focus = focusTerm
		m.forwardKey(k)
		return nil
	case "up", "k":
		m.search = ""
		m.files.move(-1, rows)
	case "down", "j":
		m.search = ""
		m.files.move(1, rows)
	case "pgup", "ctrl+u":
		m.search = ""
		m.files.move(-rows, rows)
	case "pgdown", "ctrl+d":
		m.search = ""
		m.files.move(rows, rows)
	case "home", "g":
		m.search = ""
		m.files.selected = 0
		m.files.ensure(rows)
	case "end", "G":
		m.search = ""
		if n := len(m.files.entries); n > 0 {
			m.files.selected = n - 1
		}
		m.files.ensure(rows)
	case "left", "h", "backspace":
		m.search = ""
		return m.upDir()
	case "right", "l", "enter":
		m.search = ""
		return m.openSelected()
	case "c":
		m.search = ""
		return m.cdShell()
	case "p":
		m.search = ""
		m.pasteSelected()
	case ".":
		m.files.showHidden = !m.files.showHidden
		m.search = ""
		return m.listNow()
	case "s":
		m.search = ""
		return m.syncToShell()
	default:
		if k.Type == tea.KeyRunes && !k.Alt && len(k.Runes) == 1 {
			r := k.Runes[0]
			if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' || r == '-' {
				m.search += string(unicode.ToLower(r))
				m.files.jump(m.search)
				m.files.ensure(rows)
				m.searchGen++
				gen := m.searchGen
				return tea.Tick(650*time.Millisecond, func(time.Time) tea.Msg {
					return searchResetMsg{gen: gen}
				})
			}
		}
	}
	return nil
}

func (m *Model) onMouse(msg tea.MouseMsg) tea.Cmd {
	if !m.booted || m.lay.tooSmall || m.width == 0 {
		return nil
	}
	x, y := msg.X, msg.Y
	inFile := m.lay.showRight &&
		x >= m.lay.fileX && x < m.lay.fileX+m.lay.fileW &&
		y >= m.lay.fileY && y < m.lay.fileY+m.lay.fileH
	inTerm := x >= m.lay.termX && x < m.lay.termX+m.lay.termOuterW &&
		y >= m.lay.termY && y < m.lay.termY+m.lay.termOuterH
	switch msg.Button {
	case tea.MouseButtonWheelUp:
		if inFile {
			m.files.move(-3, m.fileRows())
			return nil
		}
		if inTerm {
			m.scrollBy(3)
		}
	case tea.MouseButtonWheelDown:
		if inFile {
			m.files.move(3, m.fileRows())
			return nil
		}
		if inTerm {
			m.scrollBy(-3)
		}
	case tea.MouseButtonLeft:
		if msg.Action != tea.MouseActionPress {
			return nil
		}
		if inFile {
			m.focus = focusFiles
			row := y - (m.lay.fileY + 1 + m.lay.fileHeader)
			if row >= 0 {
				idx := m.files.offset + row
				if idx >= 0 && idx < len(m.files.entries) {
					m.files.selected = idx
					m.files.ensure(m.fileRows())
				}
			}
			return nil
		}
		if inTerm {
			m.focus = focusTerm
		}
	}
	return nil
}

func (m *Model) forwardKey(k tea.KeyMsg) {
	if m.session == nil {
		return
	}
	m.scroll = 0
	if k.Paste {
		m.session.Paste(string(k.Runes))
		return
	}
	if ev, ok := keyEvent(k); ok {
		m.session.SendKey(ev)
		return
	}
	if k.Type == tea.KeyRunes && len(k.Runes) > 0 {
		m.session.SendText(string(k.Runes))
	}
}

func (m *Model) openSelected() tea.Cmd {
	e, ok := m.files.current()
	if !ok {
		return nil
	}
	if e.IsDir {
		next := filepath.Clean(e.Path)
		m.files.follow = next == m.shellCwd
		m.files.dir = next
		m.files.selected = 0
		m.files.offset = 0
		m.search = ""
		return m.listNow()
	}
	m.pastePath(e.Path)
	m.focus = focusTerm
	return nil
}

func (m *Model) upDir() tea.Cmd {
	parent := filepath.Dir(m.files.dir)
	if parent == m.files.dir {
		return nil
	}
	m.files.follow = parent == m.shellCwd
	m.files.dir = parent
	m.files.selected = 0
	m.files.offset = 0
	m.search = ""
	return m.listNow()
}

func (m *Model) cdShell() tea.Cmd {
	if m.session == nil || m.files.dir == "" {
		return nil
	}
	m.session.SendText("cd " + format.ShellQuote(m.files.dir) + "\r")
	m.files.follow = true
	m.focus = focusTerm
	m.scroll = 0
	return nil
}

func (m *Model) pasteSelected() {
	e, ok := m.files.current()
	if !ok {
		return
	}
	m.pastePath(e.Path)
	m.focus = focusTerm
}

func (m *Model) pastePath(path string) {
	if m.session == nil || path == "" {
		return
	}
	m.scroll = 0
	m.session.Paste(format.ShellQuote(path))
}

func (m *Model) syncToShell() tea.Cmd {
	if m.shellCwd == "" {
		return nil
	}
	m.files.follow = true
	if m.files.dir == m.shellCwd {
		return m.listNow()
	}
	m.files.dir = m.shellCwd
	m.files.selected = 0
	m.files.offset = 0
	return m.listNow()
}

func (m *Model) onCwdTick() tea.Cmd {
	var cmd tea.Cmd
	if m.session != nil && m.session.Cmd != nil && m.session.Cmd.Process != nil && !m.dead {
		if dir := shell.Cwd(m.session.Cmd.Process.Pid); dir != "" && dir != m.shellCwd {
			m.shellCwd = dir
			if m.files.follow && dir != m.files.dir {
				m.files.dir = dir
				m.files.selected = 0
				m.files.offset = 0
				cmd = m.listNow()
			}
		}
	}
	var extra tea.Cmd
	if m.shellCwd != "" && m.shellCwd != m.gitDir {
		dir := m.shellCwd
		m.gitDir = dir
		m.gitBranch = ""
		m.gitDirty = false
		extra = probeGit(dir)
	}
	if m.dead {
		return tea.Batch(cmd, extra)
	}
	return tea.Batch(cmd, extra, scheduleCwd())
}

func (m *Model) applyResize() {
	if m.width <= 0 || m.height <= 0 {
		return
	}
	m.lay = computeLayout(m.width, m.height, m.showRight)
	if m.lay.tooSmall || m.session == nil {
		return
	}
	m.session.Resize(m.lay.termInnerW, m.lay.termInnerH)
	m.clampScroll()
	m.files.ensure(m.fileRows())
}

func (m *Model) fileRows() int {
	n := m.lay.fileInnerH - m.lay.fileHeader
	if n < 1 {
		return 1
	}
	return n
}

func (m *Model) scrollBy(d int) {
	if m.session == nil || m.session.Emu == nil || m.session.Emu.IsAltScreen() {
		return
	}
	m.scroll += d
	m.clampScroll()
}

func (m *Model) clampScroll() {
	if m.session == nil || m.session.Emu == nil || m.session.Emu.IsAltScreen() {
		m.scroll = 0
		return
	}
	maxScroll := m.session.Emu.ScrollbackLen()
	if m.scroll > maxScroll {
		m.scroll = maxScroll
	}
	if m.scroll < 0 {
		m.scroll = 0
	}
}

func (m *Model) hookCallbacks() {
	if m.session == nil || m.session.Emu == nil {
		return
	}
	m.session.Emu.SetCallbacks(vt.Callbacks{
		Title: func(title string) { m.title = title },
		CursorVisibility: func(visible bool) {
			m.cursorHidden = !visible
		},
	})
}

func (m *Model) listNow() tea.Cmd {
	if m.files.dir == "" {
		return nil
	}
	m.listGen++
	gen := m.listGen
	dir := m.files.dir
	hidden := m.files.showHidden
	return func() tea.Msg {
		res, err := files.Read(dir, hidden)
		return listedMsg{
			gen:     gen,
			dir:     dir,
			entries: res.Entries,
			hidden:  res.Hidden,
			clipped: res.Clipped,
			err:     err,
		}
	}
}

type startedMsg struct {
	session *shell.Session
	err     error
}

type ptyMsg struct {
	data []byte
	err  error
}

type exitMsg struct{ err error }

type statsMsg struct{ snap sysinfo.Snapshot }

type blinkMsg struct{}

type cwdTickMsg struct{}

type listedMsg struct {
	gen     int
	dir     string
	entries []files.Entry
	hidden  int
	clipped int
	err     error
}

type searchResetMsg struct{ gen int }

type gitMsg struct {
	dir    string
	branch string
	dirty  bool
}

type gitTickMsg struct{}

func probeGit(dir string) tea.Cmd {
	return func() tea.Msg {
		st := gitinfo.Probe(dir)
		return gitMsg{dir: dir, branch: st.Branch, dirty: st.Dirty}
	}
}

func scheduleGit() tea.Cmd {
	return tea.Tick(2*time.Second, func(time.Time) tea.Msg {
		return gitTickMsg{}
	})
}

func startShell() tea.Cmd {
	return func() tea.Msg {
		s, err := shell.Start(80, 24)
		return startedMsg{session: s, err: err}
	}
}

func readPty(f *os.File) tea.Cmd {
	return func() tea.Msg {
		buf := make([]byte, 32*1024)
		n, err := f.Read(buf)
		msg := ptyMsg{err: err}
		if n > 0 {
			msg.data = append([]byte(nil), buf[:n]...)
		}
		return msg
	}
}

func waitExit(cmd *exec.Cmd) tea.Cmd {
	return func() tea.Msg {
		return exitMsg{err: cmd.Wait()}
	}
}

func primeStats(s *sysinfo.Sampler) tea.Cmd {
	return func() tea.Msg {
		s.Take()
		time.Sleep(180 * time.Millisecond)
		return statsMsg{snap: s.Take()}
	}
}

func scheduleStats(s *sysinfo.Sampler) tea.Cmd {
	return tea.Tick(time.Second, func(time.Time) tea.Msg {
		return statsMsg{snap: s.Take()}
	})
}

func scheduleBlink() tea.Cmd {
	return tea.Tick(500*time.Millisecond, func(time.Time) tea.Msg {
		return blinkMsg{}
	})
}

func scheduleCwd() tea.Cmd {
	return tea.Tick(400*time.Millisecond, func(time.Time) tea.Msg {
		return cwdTickMsg{}
	})
}
