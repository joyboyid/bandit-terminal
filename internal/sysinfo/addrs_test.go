package sysinfo

import (
	"net"
	"testing"
)

func TestUsableIP(t *testing.T) {
	cases := []struct {
		in   string
		want string
		ok   bool
	}{
		{"192.168.1.20", "192.168.1.20", true},
		{"10.0.0.5", "10.0.0.5", true},
		{"127.0.0.1", "", false},
		{"0.0.0.0", "", false},
		{"169.254.1.1", "", false},
		{"::1", "", false},
		{"fe80::1", "", false},
		{"2001:db8::1", "2001:db8::1", true},
	}
	for _, tc := range cases {
		got, ok := usableIP(net.ParseIP(tc.in))
		if ok != tc.ok || got != tc.want {
			t.Errorf("%s → %q %v, want %q %v", tc.in, got, ok, tc.want, tc.ok)
		}
	}
	if _, ok := usableIP(nil); ok {
		t.Fatal("nil ip should be dropped")
	}
}

func TestSortAddrsPrefersLAN(t *testing.T) {
	in := []Addr{
		{Iface: "docker0", IP: "172.17.0.1", IPv4: true},
		{Iface: "wlan0", IP: "2001:db8::5", IPv4: false},
		{Iface: "wlan0", IP: "192.168.1.20", IPv4: true},
		{Iface: "eth0", IP: "10.0.0.5", IPv4: true},
	}
	got := sortAddrs(append([]Addr(nil), in...))
	want := []string{"10.0.0.5", "192.168.1.20", "172.17.0.1", "2001:db8::5"}
	if len(got) != len(want) {
		t.Fatalf("len %d", len(got))
	}
	for i, ip := range want {
		if got[i].IP != ip {
			t.Fatalf("order %v, want %v", ips(got), want)
		}
	}
}

func ips(a []Addr) []string {
	out := make([]string, len(a))
	for i := range a {
		out[i] = a[i].IP
	}
	return out
}
