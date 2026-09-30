// Package files lists a directory for the side panel.
package files

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"bandit-terminal/internal/format"
)

// MaxEntries is how many names we keep after sorting.
// Larger directories still open; the rest is reported as Clipped.
const MaxEntries = 3000

// Entry is one row in the file panel.
type Entry struct {
	Name   string
	Path   string
	Mode   os.FileMode
	Size   int64
	IsDir  bool
	IsLink bool
	Link   string
}

// Label is the name as shown in the list.
func (e Entry) Label() string {
	name := e.Name
	if e.IsDir {
		name += "/"
	}
	if e.IsLink && e.Link != "" {
		name += " → " + e.Link
	}
	return name
}

// SizeLabel is the short size column.
func (e Entry) SizeLabel() string {
	if e.IsLink {
		return "LNK"
	}
	if e.IsDir {
		return "DIR"
	}
	if e.Size < 0 {
		return "0B"
	}
	return format.HumanBytes(uint64(e.Size))
}

// Result is a directory listing.
type Result struct {
	Entries []Entry
	Hidden  int
	Clipped int
}

// Read lists dir. Hidden dotfiles are omitted when showHidden is false,
// except ".." which is always present so the panel can walk upward.
func Read(dir string, showHidden bool) (Result, error) {
	dir = filepath.Clean(dir)
	dents, err := os.ReadDir(dir)
	if err != nil {
		return Result{}, err
	}

	parent := filepath.Dir(dir)
	entries := make([]Entry, 0, len(dents)+1)
	entries = append(entries, Entry{
		Name:  "..",
		Path:  parent,
		Mode:  os.ModeDir | 0755,
		IsDir: true,
	})

	var hidden int
	for _, d := range dents {
		name := d.Name()
		if name == "." || name == ".." {
			continue
		}
		if strings.HasPrefix(name, ".") && !showHidden {
			hidden++
			continue
		}
		path := filepath.Join(dir, name)
		info, infoErr := d.Info()
		var mode os.FileMode
		var size int64
		if infoErr == nil {
			mode = info.Mode()
			size = info.Size()
		}
		isLink := mode&os.ModeSymlink != 0
		isDir := infoErr == nil && info.IsDir()
		link := ""
		if isLink {
			if target, rerr := os.Readlink(path); rerr == nil {
				link = target
			}
			if st, serr := os.Stat(path); serr == nil {
				isDir = st.IsDir()
			}
		}
		entries = append(entries, Entry{
			Name:   name,
			Path:   path,
			Mode:   mode,
			Size:   size,
			IsDir:  isDir,
			IsLink: isLink,
			Link:   link,
		})
	}

	sort.SliceStable(entries, func(i, j int) bool {
		a, b := entries[i], entries[j]
		if a.Name == ".." {
			return true
		}
		if b.Name == ".." {
			return false
		}
		if a.IsDir != b.IsDir {
			return a.IsDir
		}
		return strings.ToLower(a.Name) < strings.ToLower(b.Name)
	})

	clipped := 0
	if len(entries) > MaxEntries {
		clipped = len(entries) - MaxEntries
		entries = entries[:MaxEntries]
	}
	return Result{Entries: entries, Hidden: hidden, Clipped: clipped}, nil
}

// FormatCount is a short "12 items" / "1 item" label.
func FormatCount(n int) string {
	if n == 1 {
		return "1 item"
	}
	return fmt.Sprintf("%d items", n)
}
