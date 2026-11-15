package wsl

import (
	"encoding/hex"
	"regexp"
	"slices"
	"strconv"
	"strings"
)

// Disk is the guest's root filesystem usage, in bytes.
type Disk struct {
	Total uint64 `json:"total"`
	Used  uint64 `json:"used"`
}

// parseDF reads the data line of `df -kP /`.
func parseDF(lines []string) Disk {
	if len(lines) == 0 {
		return Disk{}
	}
	f := strings.Fields(lines[len(lines)-1])
	if len(f) < 3 {
		return Disk{}
	}
	total, err1 := strconv.ParseUint(f[1], 10, 64)
	used, err2 := strconv.ParseUint(f[2], 10, 64)
	if err1 != nil || err2 != nil {
		return Disk{}
	}
	return Disk{Total: total * 1024, Used: used * 1024}
}

// Port is a TCP port listening inside the guest.
type Port struct {
	Port    int    `json:"port"`
	Process string `json:"process"` // empty when owned by another user (e.g. root)
	// Loopback is true when every listener is bound to 127.0.0.1/::1 only.
	Loopback bool `json:"loopback"`
}

var ssUser = regexp.MustCompile(`users:\(\("([^"]+)"`)

// parsePorts accepts `ss -ltnpH` lines or the "P <hexaddr>:<hexport>"
// fallback lines built from /proc/net/tcp{,6}.
func parsePorts(lines []string) []Port {
	type agg struct {
		proc     string
		loopback bool
	}
	seen := map[int]*agg{}
	add := func(port int, addr, proc string) {
		if port <= 0 || port > 65535 || ignoredAddr(addr) {
			return
		}
		lb := isLoopback(addr)
		a, ok := seen[port]
		if !ok {
			seen[port] = &agg{proc: proc, loopback: lb}
			return
		}
		a.loopback = a.loopback && lb
		if a.proc == "" {
			a.proc = proc
		}
	}
	for _, l := range lines {
		f := strings.Fields(l)
		switch {
		case len(f) >= 2 && f[0] == "P":
			addr, port, ok := parseHexAddr(f[1])
			if ok {
				add(port, addr, "")
			}
		case len(f) >= 4 && f[0] == "LISTEN":
			local := f[3]
			i := strings.LastIndex(local, ":")
			if i < 0 {
				continue
			}
			port, err := strconv.Atoi(local[i+1:])
			if err != nil {
				continue
			}
			addr := strings.Trim(local[:i], "[]")
			if j := strings.Index(addr, "%"); j >= 0 { // 127.0.0.53%lo
				addr = addr[:j]
			}
			proc := ""
			if m := ssUser.FindStringSubmatch(l); m != nil {
				proc = m[1]
			}
			add(port, addr, proc)
		}
	}
	out := make([]Port, 0, len(seen))
	for p, a := range seen {
		out = append(out, Port{Port: p, Process: a.proc, Loopback: a.loopback})
	}
	slices.SortFunc(out, func(a, b Port) int { return a.Port - b.Port })
	return out
}

// ignoredAddr drops listeners that are plumbing, not your servers:
// systemd-resolved's stub resolvers and WSL's DNS tunnel.
func ignoredAddr(addr string) bool {
	return addr == "127.0.0.53" || addr == "127.0.0.54" || addr == "10.255.255.254"
}

func isLoopback(addr string) bool {
	return addr == "::1" || strings.HasPrefix(addr, "127.") || addr == "::ffff:127.0.0.1"
}

// parseHexAddr decodes /proc/net/tcp "0100007F:0BB8" (little-endian IPv4)
// or the 32-hex-digit IPv6 form into a printable address and port.
func parseHexAddr(s string) (string, int, bool) {
	h, p, ok := strings.Cut(s, ":")
	if !ok {
		return "", 0, false
	}
	port, err := strconv.ParseUint(p, 16, 16)
	if err != nil {
		return "", 0, false
	}
	b, err := hex.DecodeString(h)
	if err != nil {
		return "", 0, false
	}
	switch len(b) {
	case 4:
		return strconv.Itoa(int(b[3])) + "." + strconv.Itoa(int(b[2])) + "." +
			strconv.Itoa(int(b[1])) + "." + strconv.Itoa(int(b[0])), int(port), true
	case 16:
		// Four little-endian 32-bit words; only "any" and "::1" matter here.
		allZero := true
		for _, x := range b {
			if x != 0 {
				allZero = false
			}
		}
		if allZero {
			return "::", int(port), true
		}
		if h == "00000000000000000000000001000000" {
			return "::1", int(port), true
		}
		return h, int(port), true
	}
	return "", 0, false
}
