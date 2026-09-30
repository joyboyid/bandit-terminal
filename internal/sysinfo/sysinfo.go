// Package sysinfo reads a lightweight Linux snapshot from /proc.
package sysinfo

import (
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

// Proc is one process row in the system panel.
type Proc struct {
	PID  int
	Name string
	CPU  float64
}

// Snapshot is one sample of the machine.
type Snapshot struct {
	CPUPercent float64
	MemTotal   uint64
	MemUsed    uint64
	SwapTotal  uint64
	SwapUsed   uint64
	DiskTotal  uint64
	DiskUsed   uint64
	Load1      float64
	Load5      float64
	Load15     float64
	Uptime     time.Duration
	NetRx      float64 // bytes per second
	NetTx      float64
	Addrs      []Addr
	Ifaces     []Iface
	Power      Power
	Procs      []Proc
	User       string
	Host       string
}

// Sampler keeps the previous counters so rates have a delta.
type Sampler struct {
	mu       sync.Mutex
	user     string
	host     string
	hasCPU   bool
	idle     uint64
	total    uint64
	hasNet   bool
	rx       uint64
	tx       uint64
	netAt    time.Time
	prevLink map[string]ifaceBytes
	prevProc map[int]uint64
}

// New returns a sampler with host and user filled in.
func New() *Sampler {
	host, _ := os.Hostname()
	if i := strings.IndexByte(host, '.'); i > 0 {
		host = host[:i]
	}
	user := os.Getenv("USER")
	if user == "" {
		user = "user"
	}
	return &Sampler{
		user:     user,
		host:     host,
		prevProc: map[int]uint64{},
		prevLink: map[string]ifaceBytes{},
	}
}

// Take reads /proc and returns a snapshot. The first call reports zero
// CPU and network rates; later calls report the delta since the previous take.
func (s *Sampler) Take() Snapshot {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	snap := Snapshot{User: s.user, Host: s.host}

	var cpuDelta uint64
	if idle, total, ok := readCPU(); ok {
		if s.hasCPU && total > s.total {
			cpuDelta = total - s.total
			snap.CPUPercent = cpuPercent(s.idle, s.total, idle, total)
		}
		s.idle, s.total, s.hasCPU = idle, total, true
	}
	snap.MemTotal, snap.MemUsed, snap.SwapTotal, snap.SwapUsed = readMem()
	snap.Load1, snap.Load5, snap.Load15 = readLoad()
	snap.Uptime = readUptime()
	snap.DiskTotal, snap.DiskUsed = readDisk("/")

	if links, ok := readNetFile(); ok {
		var rx, tx uint64
		for _, c := range links {
			rx += c.rx
			tx += c.tx
		}
		if s.hasNet {
			dt := now.Sub(s.netAt).Seconds()
			if dt > 0 && rx >= s.rx && tx >= s.tx {
				snap.NetRx = float64(rx-s.rx) / dt
				snap.NetTx = float64(tx-s.tx) / dt
			}
			snap.Ifaces = ifaceRates(s.prevLink, links, dt)
		}
		s.prevLink = links
		s.rx, s.tx, s.netAt, s.hasNet = rx, tx, now, true
	}
	snap.Addrs = readAddrs()
	snap.Power = readPower()

	var procs []Proc
	procs, s.prevProc = readProcs(s.prevProc)
	if cpuDelta > 0 {
		for i := range procs {
			procs[i].CPU = procs[i].CPU / float64(cpuDelta) * 100
		}
		sort.Slice(procs, func(i, j int) bool {
			if procs[i].CPU == procs[j].CPU {
				return procs[i].Name < procs[j].Name
			}
			return procs[i].CPU > procs[j].CPU
		})
		if len(procs) > 3 {
			procs = procs[:3]
		}
		snap.Procs = procs
	}
	return snap
}

func cpuPercent(idle0, total0, idle1, total1 uint64) float64 {
	if total1 <= total0 {
		return 0
	}
	dt := total1 - total0
	di := uint64(0)
	if idle1 > idle0 {
		di = idle1 - idle0
	}
	if di > dt {
		return 0
	}
	return float64(dt-di) / float64(dt) * 100
}

func readCPU() (idle, total uint64, ok bool) {
	b, err := os.ReadFile("/proc/stat")
	if err != nil {
		return 0, 0, false
	}
	line, _, _ := strings.Cut(string(b), "\n")
	return parseCPU(line)
}

func parseCPU(line string) (idle, total uint64, ok bool) {
	f := strings.Fields(line)
	if len(f) < 5 || f[0] != "cpu" {
		return 0, 0, false
	}
	nums := make([]uint64, 0, len(f)-1)
	for _, tok := range f[1:] {
		n, err := strconv.ParseUint(tok, 10, 64)
		if err != nil {
			return 0, 0, false
		}
		nums = append(nums, n)
		total += n
	}
	idle = nums[3]
	if len(nums) > 4 {
		idle += nums[4]
	}
	return idle, total, true
}

func readMem() (total, used, swapTotal, swapUsed uint64) {
	b, err := os.ReadFile("/proc/meminfo")
	if err != nil {
		return 0, 0, 0, 0
	}
	return parseMeminfo(string(b))
}

func parseMeminfo(s string) (total, used, swapTotal, swapUsed uint64) {
	vals := map[string]uint64{}
	for _, line := range strings.Split(s, "\n") {
		key, rest, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		fields := strings.Fields(rest)
		if len(fields) == 0 {
			continue
		}
		n, err := strconv.ParseUint(fields[0], 10, 64)
		if err != nil {
			continue
		}
		vals[key] = n * 1024
	}
	total = vals["MemTotal"]
	avail := vals["MemAvailable"]
	if avail == 0 {
		avail = vals["MemFree"]
	}
	if total > avail {
		used = total - avail
	}
	swapTotal = vals["SwapTotal"]
	swapFree := vals["SwapFree"]
	if swapTotal > swapFree {
		swapUsed = swapTotal - swapFree
	}
	return total, used, swapTotal, swapUsed
}

func readLoad() (float64, float64, float64) {
	b, err := os.ReadFile("/proc/loadavg")
	if err != nil {
		return 0, 0, 0
	}
	return parseLoadavg(string(b))
}

func parseLoadavg(s string) (float64, float64, float64) {
	f := strings.Fields(s)
	if len(f) < 3 {
		return 0, 0, 0
	}
	a, _ := strconv.ParseFloat(f[0], 64)
	b, _ := strconv.ParseFloat(f[1], 64)
	c, _ := strconv.ParseFloat(f[2], 64)
	return a, b, c
}

func readUptime() time.Duration {
	b, err := os.ReadFile("/proc/uptime")
	if err != nil {
		return 0
	}
	f := strings.Fields(string(b))
	if len(f) == 0 {
		return 0
	}
	sec, err := strconv.ParseFloat(f[0], 64)
	if err != nil {
		return 0
	}
	return time.Duration(sec * float64(time.Second))
}

func readDisk(path string) (total, used uint64) {
	var st syscall.Statfs_t
	if err := syscall.Statfs(path, &st); err != nil || st.Blocks == 0 {
		return 0, 0
	}
	bsize := uint64(st.Bsize)
	total = st.Blocks * bsize
	free := st.Bavail * bsize
	if total > free {
		used = total - free
	}
	return total, used
}

// Iface is one interface's transfer rate in bytes per second.
type Iface struct {
	Name   string
	Rx, Tx float64
}

type ifaceBytes struct {
	rx, tx uint64
}

func readNetFile() (map[string]ifaceBytes, bool) {
	b, err := os.ReadFile("/proc/net/dev")
	if err != nil {
		return nil, false
	}
	return scanNetDev(string(b)), true
}

func parseNetDev(s string) (rx, tx uint64) {
	for _, c := range scanNetDev(s) {
		rx += c.rx
		tx += c.tx
	}
	return rx, tx
}

func scanNetDev(s string) map[string]ifaceBytes {
	out := map[string]ifaceBytes{}
	for _, line := range strings.Split(s, "\n") {
		name, rest, found := strings.Cut(line, ":")
		if !found {
			continue
		}
		name = strings.TrimSpace(name)
		if name == "" || name == "lo" {
			continue
		}
		f := strings.Fields(rest)
		if len(f) < 9 {
			continue
		}
		r, errR := strconv.ParseUint(f[0], 10, 64)
		t, errT := strconv.ParseUint(f[8], 10, 64)
		if errR != nil || errT != nil {
			continue
		}
		out[name] = ifaceBytes{rx: r, tx: t}
	}
	return out
}

func ifaceRates(prev, next map[string]ifaceBytes, dt float64) []Iface {
	if dt <= 0 {
		return nil
	}
	out := make([]Iface, 0, len(next))
	for name, c := range next {
		old, seen := prev[name]
		if !seen || c.rx < old.rx || c.tx < old.tx {
			continue
		}
		out = append(out, Iface{
			Name: name,
			Rx:   float64(c.rx-old.rx) / dt,
			Tx:   float64(c.tx-old.tx) / dt,
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// readProcs returns processes whose utime+stime advanced since prev.
// Proc.CPU holds the raw tick delta; the caller scales it to a percent.
func readProcs(prev map[int]uint64) (out []Proc, next map[int]uint64) {
	dents, err := os.ReadDir("/proc")
	if err != nil {
		return nil, map[int]uint64{}
	}
	next = make(map[int]uint64, len(dents))
	for _, d := range dents {
		if _, err := strconv.Atoi(d.Name()); err != nil {
			continue
		}
		b, err := os.ReadFile("/proc/" + d.Name() + "/stat")
		if err != nil {
			continue
		}
		p, comm, ticks, ok := parseProcStat(string(b))
		if !ok || strings.HasPrefix(comm, "[") {
			continue
		}
		next[p] = ticks
		old, seen := prev[p]
		if !seen || ticks <= old {
			continue
		}
		out = append(out, Proc{PID: p, Name: comm, CPU: float64(ticks - old)})
	}
	return out, next
}

// parseProcStat parses a /proc/pid/stat line.
// Field positions follow proc(5): utime is the 14th field overall,
// which is index 11 of the fields that follow the comm ")".
func parseProcStat(s string) (pid int, comm string, ticks uint64, ok bool) {
	s = strings.TrimSpace(s)
	lparen := strings.IndexByte(s, '(')
	rparen := strings.LastIndexByte(s, ')')
	if lparen <= 0 || rparen < lparen {
		return 0, "", 0, false
	}
	var err error
	pid, err = strconv.Atoi(strings.TrimSpace(s[:lparen]))
	if err != nil {
		return 0, "", 0, false
	}
	comm = s[lparen+1 : rparen]
	fields := strings.Fields(s[rparen+1:])
	if len(fields) < 13 {
		return 0, "", 0, false
	}
	utime, errU := strconv.ParseUint(fields[11], 10, 64)
	stime, errS := strconv.ParseUint(fields[12], 10, 64)
	if errU != nil || errS != nil {
		return 0, "", 0, false
	}
	return pid, comm, utime + stime, true
}
