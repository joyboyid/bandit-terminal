package gitinfo

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestParseStatus(t *testing.T) {
	clean := parseStatus("## main...origin/main\n")
	if clean.Branch != "main" || clean.Dirty {
		t.Fatalf("%+v", clean)
	}
	dirty := parseStatus("## feature/ui...origin/feature/ui [ahead 1]\n M view.go\n?? new.go\n")
	if dirty.Branch != "feature/ui" || !dirty.Dirty {
		t.Fatalf("%+v", dirty)
	}
	det := parseStatus("## HEAD (no branch)\n")
	if det.Branch != "detached" || det.Dirty {
		t.Fatalf("%+v", det)
	}
	fresh := parseStatus("## No commits yet on main\n?? a.txt\n")
	if fresh.Branch != "main" || !fresh.Dirty {
		t.Fatalf("%+v", fresh)
	}
}

func TestProbe(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not installed")
	}
	dir := t.TempDir()
	git := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=bandit",
			"GIT_AUTHOR_EMAIL=bandit@example.com",
			"GIT_COMMITTER_NAME=bandit",
			"GIT_COMMITTER_EMAIL=bandit@example.com",
		)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	git("init", "-b", "main")
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("a"), 0o644); err != nil {
		t.Fatal(err)
	}
	git("add", "a.txt")
	git("commit", "-m", "init")
	st := Probe(dir)
	if !st.OK || st.Branch != "main" || st.Dirty {
		t.Fatalf("clean %+v", st)
	}
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("b"), 0o644); err != nil {
		t.Fatal(err)
	}
	st = Probe(dir)
	if !st.Dirty || st.Branch != "main" {
		t.Fatalf("dirty %+v", st)
	}
	if Probe(t.TempDir()).OK {
		t.Fatal("a fresh directory is not a repository")
	}
}
