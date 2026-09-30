package format

import (
	"testing"
	"time"
)

func TestHumanBytes(t *testing.T) {
	cases := []struct {
		n    uint64
		want string
	}{
		{0, "0B"},
		{1023, "1023B"},
		{1024, "1.0K"},
		{1536, "1.5K"},
		{1024 * 1024, "1.0M"},
		{100 * 1024 * 1024, "100M"},
	}
	for _, tc := range cases {
		got := HumanBytes(tc.n)
		if got != tc.want {
			t.Errorf("HumanBytes(%d) = %q, want %q", tc.n, got, tc.want)
		}
		if len(got) > 6 {
			t.Errorf("HumanBytes(%d) is %d bytes, want <= 6", tc.n, len(got))
		}
	}
}

func TestUptime(t *testing.T) {
	cases := []struct {
		d    time.Duration
		want string
	}{
		{90 * time.Second, "1m 30s"},
		{3700 * time.Second, "1h 01m"},
		{90000 * time.Second, "1d 01h"},
	}
	for _, tc := range cases {
		if got := Uptime(tc.d); got != tc.want {
			t.Errorf("Uptime(%s) = %q, want %q", tc.d, got, tc.want)
		}
	}
}

func TestPrettyPath(t *testing.T) {
	home := "/home/mandex"
	if got := PrettyPath(home, home); got != "~" {
		t.Fatalf("home: %q", got)
	}
	if got := PrettyPath(home+"/src", home); got != "~/src" {
		t.Fatalf("child: %q", got)
	}
	if got := PrettyPath("/tmp", home); got != "/tmp" {
		t.Fatalf("other: %q", got)
	}
	if got := PrettyPath(home+"2", home); got != home+"2" {
		t.Fatalf("prefix trap: %q", got)
	}
}

func TestShellQuote(t *testing.T) {
	if got := ShellQuote("a b"); got != "'a b'" {
		t.Fatal(got)
	}
	if got := ShellQuote("it's"); got != `'it'\''s'` {
		t.Fatal(got)
	}
	if got := ShellQuote(""); got != "''" {
		t.Fatal(got)
	}
}
