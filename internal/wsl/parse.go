// Package wsl talks to wsl.exe and parses the guest's /proc output.
//
// Everything that reads guest state goes through Runner.Metrics, which only
// executes inside a distro the caller already knows is running. Listing
// distros never boots one.
package wsl

import (
	"bufio"
	"bytes"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"unicode/utf16"
)

// Distro is one registered WSL distribution.
type Distro struct {
	Name    string `json:"name"`
	Default bool   `json:"default"`
	Version int    `json:"version"`
	Running bool   `json:"running"`
}

// decodeOutput turns wsl.exe output into a string. wsl.exe writes UTF-16LE
// unless WSL_UTF8=1 is honoured (older builds ignore it), so detect NULs.
func decodeOutput(b []byte) string {
	b = bytes.TrimPrefix(b, []byte{0xFF, 0xFE})
	if len(b) >= 2 && bytes.Count(b, []byte{0}) > len(b)/4 {
		u := make([]uint16, len(b)/2)
		for i := range u {
			u[i] = uint16(b[2*i]) | uint16(b[2*i+1])<<8
		}
		return string(utf16.Decode(u))
	}
	return string(b)
}

// parseListVerbose parses `wsl.exe -l -v`. The STATE column is localized, so
// it is ignored here; running state comes from parseNames on `-l --running -q`.
func parseListVerbose(out string) []Distro {
	var ds []Distro
	sc := bufio.NewScanner(strings.NewReader(out))
	first := true
	for sc.Scan() {
		line := strings.TrimRight(sc.Text(), "\r \t")
		if strings.TrimSpace(line) == "" {
			continue
		}
		if first { // header row, localized
			first = false
			continue
		}
		d := Distro{}
		if strings.HasPrefix(line, "*") {
			d.Default = true
			line = line[1:]
		}
		f := strings.Fields(line)
		if len(f) < 2 {
			continue
		}
		d.Name = f[0]
		if v, err := strconv.Atoi(f[len(f)-1]); err == nil {
			d.Version = v
		}
		ds = append(ds, d)
	}
	return ds
}

// parseNames parses `wsl.exe -l -q` style output: one distro name per line.
// When nothing is running, wsl.exe prints a localized sentence instead, so
// callers must intersect the result with known distro names.
func parseNames(out string) []string {
	var names []string
	for _, l := range strings.Split(out, "\n") {
		l = strings.TrimSpace(l)
		if l != "" {
			names = append(names, l)
		}
	}
	return names
}

// CPUSample is the aggregate "cpu" line of /proc/stat.
type CPUSample struct {
	Idle, Total uint64
}

func parseCPULine(line string) (CPUSample, bool) {
	f := strings.Fields(line)
	if len(f) < 5 || f[0] != "cpu" {
		return CPUSample{}, false
	}
	var s CPUSample
	for i, v := range f[1:] {
		n, err := strconv.ParseUint(v, 10, 64)
		if err != nil {
			return CPUSample{}, false
		}
		// guest and guest_nice (fields 9, 10) are already counted in user/nice.
		if i >= 8 {
			break
		}
		s.Total += n
		if i == 3 || i == 4 { // idle, iowait
			s.Idle += n
		}
	}
	return s, true
}

// CPUPercent returns busy time between two samples as 0..100.
func CPUPercent(prev, cur CPUSample) float64 {
	dt := float64(cur.Total) - float64(prev.Total)
	if dt <= 0 {
		return 0
	}
	busy := dt - (float64(cur.Idle) - float64(prev.Idle))
	p := busy / dt * 100
	if p < 0 {
		return 0
	}
	if p > 100 {
		return 100
	}
	return p
}

// Memory is in bytes.
type Memory struct {
	Total     uint64 `json:"total"`
	Used      uint64 `json:"used"`
	SwapTotal uint64 `json:"swapTotal"`
	SwapUsed  uint64 `json:"swapUsed"`
}

func parseMeminfo(lines []string) Memory {
	kv := map[string]uint64{}
	for _, l := range lines {
		k, rest, ok := strings.Cut(l, ":")
		if !ok {
			continue
		}
		f := strings.Fields(rest)
		if len(f) == 0 {
			continue
		}
		n, err := strconv.ParseUint(f[0], 10, 64)
		if err != nil {
			continue
		}
		kv[k] = n * 1024
	}
	m := Memory{Total: kv["MemTotal"], SwapTotal: kv["SwapTotal"]}
	if avail, ok := kv["MemAvailable"]; ok && avail <= m.Total {
		m.Used = m.Total - avail
	}
	if kv["SwapFree"] <= m.SwapTotal {
		m.SwapUsed = m.SwapTotal - kv["SwapFree"]
	}
	return m
}

// Sample is one parsed run of metricsScript.
type Sample struct {
	UptimeSec float64
	CPU       []CPUSample // one or two samples
	Mem       Memory
	Procs     map[string]bool // process names outside containers
	// ContainerProcs maps names of processes running inside a container to
	// that container's full ID ("" when the cgroup path does not name it).
	ContainerProcs map[string]string
	Disk           Disk
	Ports          []Port
	Home           string // $HOME of the distro's default user
	Systemd        bool   // PID 1 is systemd
	OSID           string // /etc/os-release ID, e.g. "ubuntu"
	OSLike         string // /etc/os-release ID_LIKE, e.g. "debian"
	TailscaleSSH   bool   // tailscaled serves SSH (`tailscale set --ssh`)
}

const sectionSep = "@@"

// cgroupContainerID finds the container ID in a cgroup path such as
// "0::/system.slice/docker-<id>.scope" or "0::/docker/<id>".
var cgroupContainerID = regexp.MustCompile(`[0-9a-f]{64}`)

func ParseSample(out string) Sample {
	var r Sample
	r.Procs = map[string]bool{}
	sections := splitSections(out)
	get := func(i int) []string {
		if i < len(sections) {
			return sections[i]
		}
		return nil
	}
	if up := get(0); len(up) > 0 {
		if f := strings.Fields(up[0]); len(f) > 0 {
			r.UptimeSec, _ = strconv.ParseFloat(f[0], 64)
		}
	}
	for _, l := range get(1) {
		if s, ok := parseCPULine(l); ok {
			r.CPU = append(r.CPU, s)
		}
	}
	r.Mem = parseMeminfo(get(2))
	r.ContainerProcs = map[string]string{}
	for _, l := range get(3) {
		cg, comm, ok := strings.Cut(l, " ")
		if !strings.HasPrefix(cg, "%") || !ok {
			r.Procs[l] = true
			continue
		}
		if r.ContainerProcs[comm] == "" { // prefer an entry that names its container
			r.ContainerProcs[comm] = cgroupContainerID.FindString(cg)
		}
	}
	r.Disk = parseDF(get(4))
	r.Ports = parsePorts(get(5))
	if hi := get(6); len(hi) > 0 {
		if strings.HasPrefix(hi[0], "/") {
			r.Home = hi[0]
		}
		for _, l := range hi {
			if rest, ok := strings.CutPrefix(l, "os:"); ok {
				id, like, _ := strings.Cut(rest, ":")
				r.OSID, r.OSLike = strings.Trim(id, `"`), strings.Trim(like, `"`)
			}
		}
		r.Systemd = hi[len(hi)-1] == "systemd"
	}
	r.TailscaleSSH = slices.Contains(get(7), "tsssh")
	return r
}

func splitSections(out string) [][]string {
	var secs [][]string
	cur := []string{}
	for _, l := range strings.Split(out, "\n") {
		l = strings.TrimSpace(l)
		if l == sectionSep {
			secs = append(secs, cur)
			cur = []string{}
			continue
		}
		if l != "" {
			cur = append(cur, l)
		}
	}
	return append(secs, cur)
}
