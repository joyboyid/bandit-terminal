package files

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReadOrderAndHidden(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "b.txt"))
	mustWrite(t, filepath.Join(dir, "a.txt"))
	if err := os.Mkdir(filepath.Join(dir, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	mustWrite(t, filepath.Join(dir, ".hide"))

	got, err := Read(dir, false)
	if err != nil {
		t.Fatal(err)
	}
	if got.Hidden != 1 {
		t.Fatalf("hidden = %d, want 1", got.Hidden)
	}
	names := namesOf(got.Entries)
	want := []string{"..", "sub", "a.txt", "b.txt"}
	if !equal(names, want) {
		t.Fatalf("names = %v, want %v", names, want)
	}
	if !got.Entries[1].IsDir {
		t.Fatal("sub should be a directory")
	}

	all, err := Read(dir, true)
	if err != nil {
		t.Fatal(err)
	}
	if all.Hidden != 0 {
		t.Fatalf("hidden with show = %d", all.Hidden)
	}
	allNames := namesOf(all.Entries)
	wantAll := []string{"..", "sub", ".hide", "a.txt", "b.txt"}
	if !equal(allNames, wantAll) {
		t.Fatalf("all = %v, want %v", allNames, wantAll)
	}
}

func TestReadMissing(t *testing.T) {
	if _, err := Read(filepath.Join(t.TempDir(), "nope"), true); err == nil {
		t.Fatal("expected an error")
	}
}

func TestSizeLabel(t *testing.T) {
	dir := Entry{IsDir: true}
	if dir.SizeLabel() != "DIR" {
		t.Fatal(dir.SizeLabel())
	}
	link := Entry{IsLink: true, Link: "x"}
	if link.SizeLabel() != "LNK" || link.Label() != " → x" {
		t.Fatalf("%s %s", link.SizeLabel(), link.Label())
	}
	file := Entry{Name: "a", Size: 1536}
	if file.SizeLabel() != "1.5K" {
		t.Fatal(file.SizeLabel())
	}
}

func mustWrite(t *testing.T, path string) {
	t.Helper()
	if err := os.WriteFile(path, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func namesOf(entries []Entry) []string {
	out := make([]string, len(entries))
	for i, e := range entries {
		out[i] = e.Name
	}
	return out
}

func equal(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
