// Package gitinfo reads the branch and dirty state of a working tree.
package gitinfo

import (
	"context"
	"os/exec"
	"strings"
	"time"
)

// Status is the branch shown in the header. Ok is false outside a repository.
type Status struct {
	Branch string
	Dirty  bool
	OK     bool
}

// Probe asks git for the branch at dir. It gives up after a short wait
// so a huge working tree cannot stall the terminal.
func Probe(dir string) Status {
	if dir == "" {
		return Status{}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	cmd := exec.CommandContext(ctx, "git", "-C", dir, "status", "--porcelain=v1", "-b")
	cmd.Env = append(cmd.Environ(), "GIT_OPTIONAL_LOCKS=0")
	out, err := cmd.Output()
	if err != nil {
		return Status{}
	}
	st := parseStatus(string(out))
	st.OK = st.Branch != ""
	return st
}

func parseStatus(s string) Status {
	s = strings.TrimRight(s, "\n")
	if s == "" {
		return Status{}
	}
	lines := strings.Split(s, "\n")
	head := lines[0]
	if !strings.HasPrefix(head, "## ") {
		return Status{}
	}
	head = strings.TrimPrefix(head, "## ")
	if i := strings.Index(head, " ["); i >= 0 {
		head = head[:i]
	}
	branch := head
	if i := strings.Index(branch, "..."); i >= 0 {
		branch = branch[:i]
	}
	branch = strings.TrimSpace(branch)
	switch {
	case strings.HasPrefix(branch, "No commits yet on "):
		branch = strings.TrimPrefix(branch, "No commits yet on ")
	case branch == "HEAD (no branch)" || branch == "HEAD" || strings.HasPrefix(branch, "HEAD "):
		branch = "detached"
	}
	if branch == "" {
		return Status{}
	}
	dirty := false
	for _, ln := range lines[1:] {
		if strings.TrimSpace(ln) != "" {
			dirty = true
			break
		}
	}
	return Status{Branch: branch, Dirty: dirty}
}
