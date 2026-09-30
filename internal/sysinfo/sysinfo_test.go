package sysinfo

import (
	"os"
	"testing"
	"time"
)

func TestParseCPU(t *testing.T) {
	idle, total, ok := parseCPU("cpu  10 20 30 40 5 1 2 0 0 0")
	if !ok {
		t.Fatal("expected ok")
	}
	if idle != 45 {
		t.Fatalf("idle = %d, want 45", idle)
	}
	if total != 108 {
		t.Fatalf("total = %d, want 108", total)
	}
	if _, _, ok := parseCPU("intr 1 2 3"); ok {
		t.Fatal("non-cpu line should fail")
	}
}

func TestCPUPercent(t *testing.T) {
	// 100 ticks, 25 of them busy.
	got := cpuPercent(75, 100, 150, 200)
	if got < 24.9 || got > 25.1 {
		t.Fatalf("percent = %v, want 25", got)
	}
	if cpuPercent(1, 1, 1, 1) != 0 {
		t.Fatal("zero delta should be 0")
	}
}

func TestParseMeminfo(t *testing.T) {
	const sample = `
MemTotal:       1000 kB
MemFree:         100 kB
MemAvailable:    400 kB
SwapTotal:       200 kB
SwapFree:         50 kB
`
	total, used, swapTotal, swapUsed := parseMeminfo(sample)
	if total != 1000*1024 || used != 600*1024 {
		t.Fatalf("mem total=%d used=%d", total, used)
	}
	if swapTotal != 200*1024 || swapUsed != 150*1024 {
		t.Fatalf("swap total=%d used=%d", swapTotal, swapUsed)
	}
}

func TestParseNetDev(t *testing.T) {
	const sample = `
Inter-|   Receive                                                |  Transmit
 face |bytes    packets errs drop fifo frame compressed multicast|bytes    packets
    lo: 9 0 0 0 0 0 0 0 8 0 0 0 0 0 0 0
  eth0: 1000 0 0 0 0 0 0 0 2000 0 0 0 0 0 0 0
  wlan0: 5 0 0 0 0 0 0 0 7 0 0 0 0 0 0 0
`
	rx, tx := parseNetDev(sample)
	if rx != 1005 || tx != 2007 {
		t.Fatalf("rx=%d tx=%d", rx, tx)
	}
}

func TestIfaceRates(t *testing.T) {
	prev := map[string]ifaceBytes{
		"wlan0": {rx: 1000, tx: 100},
		"eth0":  {rx: 50, tx: 50},
	}
	next := map[string]ifaceBytes{
		"wlan0": {rx: 3000, tx: 1100},
		"eth0":  {rx: 50, tx: 40},
	}
	got := ifaceRates(prev, next, 2)
	if len(got) != 1 || got[0].Name != "wlan0" || got[0].Rx != 1000 || got[0].Tx != 500 {
		t.Fatalf("%+v", got)
	}
	links := scanNetDev(`
    lo: 9 0 0 0 0 0 0 0 8 0 0 0 0 0 0 0
  eth0: 1000 0 0 0 0 0 0 0 2000 0 0 0 0 0 0 0
 wlan0: 5 0 0 0 0 0 0 0 7 0 0 0 0 0 0 0
`)
	if links["lo"].rx != 0 || links["eth0"].rx != 1000 || links["wlan0"].tx != 7 {
		t.Fatalf("%+v", links)
	}
}

func TestParseLoadavg(t *testing.T) {
	a, b, c := parseLoadavg("0.42 0.50 0.61 1/234 9999\n")
	if a != 0.42 || b != 0.50 || c != 0.61 {
		t.Fatalf("%v %v %v", a, b, c)
	}
}

func TestParseProcStat(t *testing.T) {
	// utime is fields[11] after comm, stime is fields[12].
	line := "42 (my proc) R 1 1 1 0 0 0 10 0 0 0 5 7"
	pid, comm, ticks, ok := parseProcStat(line)
	if !ok || pid != 42 || comm != "my proc" || ticks != 12 {
		t.Fatalf("pid=%d comm=%q ticks=%d ok=%v", pid, comm, ticks, ok)
	}
}

func TestLiveSample(t *testing.T) {
	if _, err := os.Stat("/proc/meminfo"); err != nil {
		t.Skip("/proc/meminfo unavailable")
	}
	s := New()
	s.Take()
	time.Sleep(30 * time.Millisecond)
	snap := s.Take()
	if snap.MemTotal == 0 {
		t.Fatal("expected MemTotal")
	}
	if snap.Host == "" || snap.User == "" {
		t.Fatalf("identity %+v", snap)
	}
	if snap.DiskTotal == 0 {
		t.Fatal("expected disk total")
	}
}
