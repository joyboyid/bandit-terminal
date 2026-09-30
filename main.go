package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"

	tea "github.com/charmbracelet/bubbletea"
	"golang.org/x/term"

	"bandit-terminal/internal/ui"
)

const helpText = `bandit-terminal — a small hacker terminal

Your login shell runs in the main pane. A file list and a system
HUD sit beside it. One static binary, no browser engine.

  bandit
  bandit --help

Keys
  F1         focus the file list, or return to the shell
  F2         show or hide the side panels
  F3         show or hide this machine's IP addresses
  F4         privacy mode: hide IP, username, and home path
  /          search scrollback (↑↓ next match, enter, esc)
  ^Q         quit
  ^PgUp      scroll back through shell output
  ^PgDn      scroll toward the live prompt
  wheel      scroll the pane under the pointer

The header shows the git branch of the shell directory. A trailing
* means the working tree has changes. The system panel shows this
machine's addresses, plus battery and CPU temperature when the
machine reports them.

File list
  ↑↓ j k     move
  enter      open a directory, or paste a file path
  h  ←       parent directory
  c          cd the shell into this directory
  p          paste the path into the shell
  .          show or hide dotfiles
  s          jump the list back to the shell's directory
  g G        top / bottom
  esc        back to the shell
`

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "bandit-terminal:", err)
		os.Exit(1)
	}
}

func run() error {
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "-h", "--help", "help":
			fmt.Print(helpText)
			return nil
		default:
			return fmt.Errorf("unknown argument %q\n\n%s", os.Args[1], helpText)
		}
	}
	if !term.IsTerminal(int(os.Stdout.Fd())) {
		if os.Getenv("BANDIT_LAUNCHED") == "1" || !graphicalSession() {
			return errors.New("stdout is not a terminal")
		}
		return launchInTerminal()
	}
	m := ui.New()
	p := tea.NewProgram(m, tea.WithAltScreen(), tea.WithMouseCellMotion())
	_, err := p.Run()
	m.Shutdown()
	if errors.Is(err, tea.ErrInterrupted) {
		return nil
	}
	return err
}

func graphicalSession() bool {
	return os.Getenv("DISPLAY") != "" || os.Getenv("WAYLAND_DISPLAY") != ""
}

// launchInTerminal replaces this process with a terminal emulator.
// Menus such as dmenu and rofi start programs without a tty.
func launchInTerminal() error {
	bin, prefix, err := pickTerminal()
	if err != nil {
		return err
	}
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	env := append(os.Environ(), "BANDIT_LAUNCHED=1")
	argv := append([]string{bin}, prefix...)
	argv = append(argv, exe)
	return syscall.Exec(bin, argv, env)
}

func pickTerminal() (string, []string, error) {
	if spec := strings.TrimSpace(os.Getenv("TERMINAL")); spec != "" {
		fields := strings.Fields(spec)
		if path, err := exec.LookPath(fields[0]); err == nil {
			args := fields[1:]
			if len(args) == 0 {
				args = terminalArgs(fields[0])
			}
			return path, args, nil
		}
	}
	for _, name := range []string{
		"x-terminal-emulator",
		"foot", "kitty", "alacritty", "ghostty", "wezterm", "st",
		"gnome-terminal", "kgx", "konsole", "xfce4-terminal",
		"mate-terminal", "tilix", "terminator", "lxterminal",
		"urxvt", "xterm",
	} {
		path, err := exec.LookPath(name)
		if err != nil {
			continue
		}
		return path, terminalArgs(name), nil
	}
	return "", nil, errors.New("stdout is not a terminal, and no terminal emulator was found")
}

func terminalArgs(name string) []string {
	base := name
	if i := strings.LastIndex(name, "/"); i >= 0 {
		base = name[i+1:]
	}
	switch base {
	case "wezterm":
		return []string{"start", "--"}
	case "gnome-terminal", "kgx":
		return []string{"--"}
	default:
		return []string{"-e"}
	}
}
