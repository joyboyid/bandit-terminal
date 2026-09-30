package sysinfo

import (
	"net"
	"sort"
	"strings"
)

// Addr is one IP on a network interface.
type Addr struct {
	Iface string
	IP    string
	IPv4  bool
}

// usableIP keeps addresses that identify this machine on a network.
// Loopback, unspecified, multicast, and link-local addresses are left out.
func usableIP(ip net.IP) (string, bool) {
	if ip == nil || ip.IsLoopback() || ip.IsUnspecified() || ip.IsMulticast() {
		return "", false
	}
	if ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() {
		return "", false
	}
	if v4 := ip.To4(); v4 != nil {
		return v4.String(), true
	}
	if ip.IsGlobalUnicast() {
		return ip.String(), true
	}
	return "", false
}

// virtualIface marks container and hypervisor bridges. They stay in the
// list, sorted after the machine's own interfaces.
func virtualIface(name string) bool {
	n := strings.ToLower(name)
	switch {
	case n == "docker0":
		return true
	case strings.HasPrefix(n, "br-"),
		strings.HasPrefix(n, "veth"),
		strings.HasPrefix(n, "virbr"),
		strings.HasPrefix(n, "cni"),
		strings.HasPrefix(n, "flannel"),
		strings.HasPrefix(n, "cali"),
		strings.HasPrefix(n, "vboxnet"),
		strings.HasPrefix(n, "vmnet"):
		return true
	default:
		return false
	}
}

func sortAddrs(in []Addr) []Addr {
	sort.SliceStable(in, func(i, j int) bool {
		if in[i].IPv4 != in[j].IPv4 {
			return in[i].IPv4
		}
		vi, vj := virtualIface(in[i].Iface), virtualIface(in[j].Iface)
		if vi != vj {
			return !vi
		}
		if in[i].Iface != in[j].Iface {
			return in[i].Iface < in[j].Iface
		}
		return in[i].IP < in[j].IP
	})
	return in
}

func readAddrs() []Addr {
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil
	}
	var out []Addr
	for _, ifc := range ifaces {
		if ifc.Flags&net.FlagUp == 0 || ifc.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, err := ifc.Addrs()
		if err != nil {
			continue
		}
		for _, a := range addrs {
			var ip net.IP
			switch v := a.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			}
			s, ok := usableIP(ip)
			if !ok {
				continue
			}
			out = append(out, Addr{Iface: ifc.Name, IP: s, IPv4: ip.To4() != nil})
		}
	}
	return sortAddrs(out)
}
