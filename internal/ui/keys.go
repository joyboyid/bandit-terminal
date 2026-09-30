package ui

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/vt"
)

// keyEvent converts a Bubble Tea key into a virtual-terminal key.
// Multi-rune pastes are handled by the caller; this returns false for those.
func keyEvent(k tea.KeyMsg) (vt.KeyPressEvent, bool) {
	mod := vt.KeyMod(0)
	if k.Alt {
		mod |= vt.ModAlt
	}
	ev := func(code rune, extra vt.KeyMod) (vt.KeyPressEvent, bool) {
		return vt.KeyPressEvent{Code: code, Mod: mod | extra}, true
	}

	switch k.Type {
	case tea.KeyRunes:
		if len(k.Runes) != 1 {
			return vt.KeyPressEvent{}, false
		}
		return ev(k.Runes[0], 0)
	case tea.KeySpace:
		return ev(vt.KeySpace, 0)
	case tea.KeyEnter:
		return ev(vt.KeyEnter, 0)
	case tea.KeyTab:
		return ev(vt.KeyTab, 0)
	case tea.KeyShiftTab:
		return ev(vt.KeyTab, vt.ModShift)
	case tea.KeyBackspace:
		return ev(vt.KeyBackspace, 0)
	case tea.KeyEscape:
		return ev(vt.KeyEscape, 0)
	case tea.KeyUp:
		return ev(vt.KeyUp, 0)
	case tea.KeyDown:
		return ev(vt.KeyDown, 0)
	case tea.KeyRight:
		return ev(vt.KeyRight, 0)
	case tea.KeyLeft:
		return ev(vt.KeyLeft, 0)
	case tea.KeyShiftUp:
		return ev(vt.KeyUp, vt.ModShift)
	case tea.KeyShiftDown:
		return ev(vt.KeyDown, vt.ModShift)
	case tea.KeyShiftRight:
		return ev(vt.KeyRight, vt.ModShift)
	case tea.KeyShiftLeft:
		return ev(vt.KeyLeft, vt.ModShift)
	case tea.KeyCtrlUp:
		return ev(vt.KeyUp, vt.ModCtrl)
	case tea.KeyCtrlDown:
		return ev(vt.KeyDown, vt.ModCtrl)
	case tea.KeyCtrlRight:
		return ev(vt.KeyRight, vt.ModCtrl)
	case tea.KeyCtrlLeft:
		return ev(vt.KeyLeft, vt.ModCtrl)
	case tea.KeyHome, tea.KeyCtrlHome:
		return ev(vt.KeyHome, 0)
	case tea.KeyEnd, tea.KeyCtrlEnd:
		return ev(vt.KeyEnd, 0)
	case tea.KeyPgUp:
		return ev(vt.KeyPgUp, 0)
	case tea.KeyPgDown:
		return ev(vt.KeyPgDown, 0)
	case tea.KeyInsert:
		return ev(vt.KeyInsert, 0)
	case tea.KeyDelete:
		return ev(vt.KeyDelete, 0)
	case tea.KeyF1:
		return ev(vt.KeyF1, 0)
	case tea.KeyF2:
		return ev(vt.KeyF2, 0)
	case tea.KeyF3:
		return ev(vt.KeyF3, 0)
	case tea.KeyF4:
		return ev(vt.KeyF4, 0)
	case tea.KeyF5:
		return ev(vt.KeyF5, 0)
	case tea.KeyF6:
		return ev(vt.KeyF6, 0)
	case tea.KeyF7:
		return ev(vt.KeyF7, 0)
	case tea.KeyF8:
		return ev(vt.KeyF8, 0)
	case tea.KeyF9:
		return ev(vt.KeyF9, 0)
	case tea.KeyF10:
		return ev(vt.KeyF10, 0)
	case tea.KeyF11:
		return ev(vt.KeyF11, 0)
	case tea.KeyF12:
		return ev(vt.KeyF12, 0)
	case tea.KeyCtrlAt:
		return ev(vt.KeySpace, vt.ModCtrl)
	case tea.KeyCtrlBackslash:
		return ev('\\', vt.ModCtrl)
	case tea.KeyCtrlCloseBracket:
		return ev(']', vt.ModCtrl)
	case tea.KeyCtrlCaret:
		return ev('^', vt.ModCtrl)
	case tea.KeyCtrlUnderscore:
		return ev('_', vt.ModCtrl)
	default:
		if k.Type >= tea.KeyCtrlA && k.Type <= tea.KeyCtrlZ {
			return ev(rune('a'+int(k.Type-tea.KeyCtrlA)), vt.ModCtrl)
		}
		return vt.KeyPressEvent{}, false
	}
}
