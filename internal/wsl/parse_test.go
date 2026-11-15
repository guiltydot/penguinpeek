package wsl

import (
	"math"
	"os"
	"testing"
	"unicode/utf16"
)

func TestParseListVerboseFixture(t *testing.T) {
	b, err := os.ReadFile("testdata/list_verbose.txt")
	if err != nil {
		t.Fatal(err)
	}
	ds := parseListVerbose(decodeOutput(b))
	if len(ds) != 4 || ds[0].Name != "Ubuntu-24.04" || !ds[0].Default || ds[0].Version != 2 ||
		ds[2].Name != "docker-desktop" || ds[3].Version != 1 {
		t.Fatalf("got %+v", ds)
	}
}

func TestParseListVerboseLocalizedMulti(t *testing.T) {
	out := "  NAME            STATUS            VERSION\r\n" +
		"* Ubuntu          Wird ausgeführt   2\r\n" +
		"  docker-desktop  Beendet           2\r\n" +
		"  Legacy          Stopped           1\r\n"
	ds := parseListVerbose(out)
	if len(ds) != 3 {
		t.Fatalf("want 3, got %+v", ds)
	}
	if !ds[0].Default || ds[1].Default || ds[1].Name != "docker-desktop" || ds[2].Version != 1 {
		t.Fatalf("got %+v", ds)
	}
}

func TestDecodeUTF16(t *testing.T) {
	u := utf16.Encode([]rune("Ubuntu\r\n"))
	b := []byte{0xFF, 0xFE}
	for _, c := range u {
		b = append(b, byte(c), byte(c>>8))
	}
	if got := decodeOutput(b); got != "Ubuntu\r\n" {
		t.Fatalf("got %q", got)
	}
	if got := decodeOutput([]byte("plain")); got != "plain" {
		t.Fatalf("got %q", got)
	}
}

func TestParseSample(t *testing.T) {
	out := "4792.65 56156.95\n@@\n" +
		"cpu  100 0 100 700 100 0 0 0 0 0\n" +
		"cpu  150 0 150 750 100 0 0 0 0 0\n@@\n" +
		"MemTotal:        8000 kB\nMemAvailable:    6000 kB\nSwapTotal:       1000 kB\nSwapFree:         250 kB\n@@\n" +
		"dockerd\nbash\nredis-server\n" +
		"%0::/system.slice/docker-abc123abc123abc123abc123abc123abc123abc123abc123abc123abc123abc1.scope nginx\n" +
		"%0::/system.slice/docker-abc123abc123abc123abc123abc123abc123abc123abc123abc123abc123abc1.scope nginx\n" +
		"%0::/docker/oddpath pg worker\n"
	s := ParseSample(out)
	if s.UptimeSec != 4792.65 {
		t.Errorf("uptime %v", s.UptimeSec)
	}
	if len(s.CPU) != 2 {
		t.Fatalf("cpu %+v", s.CPU)
	}
	// 150 jiffies elapsed, 50 idle -> 66.7%.
	if p := CPUPercent(s.CPU[0], s.CPU[1]); math.Abs(p-200.0/3) > 0.01 {
		t.Errorf("cpu%% %v", p)
	}
	if s.Mem.Total != 8000*1024 || s.Mem.Used != 2000*1024 || s.Mem.SwapUsed != 750*1024 {
		t.Errorf("mem %+v", s.Mem)
	}
	if !s.Procs["dockerd"] || !s.Procs["redis-server"] || s.Procs["postgres"] {
		t.Errorf("procs %v", s.Procs)
	}
	if s.Procs["nginx"] || s.ContainerProcs["nginx"] != "abc123abc123abc123abc123abc123abc123abc123abc123abc123abc123abc1" {
		t.Errorf("container procs %v, procs %v", s.ContainerProcs, s.Procs)
	}
	if id, ok := s.ContainerProcs["pg worker"]; !ok || id != "" {
		t.Errorf("unnamed container proc %q %v", id, ok)
	}
}

func TestCPUPercentClamps(t *testing.T) {
	if p := CPUPercent(CPUSample{10, 100}, CPUSample{10, 100}); p != 0 {
		t.Errorf("no elapsed: %v", p)
	}
}

func TestParseSampleOSRelease(t *testing.T) {
	out := "1 1\n@@\ncpu  1 0 1 1 0 0 0 0 0 0\n@@\nMemTotal: 10 kB\nMemAvailable: 5 kB\n@@\nbash\n@@\n" +
		"/dev/sdc 1000 400 600 40% /\n@@\n@@\n/home/me\nos:kali:debian\nsystemd\n"
	s := ParseSample(out)
	if s.OSID != "kali" || s.OSLike != "debian" || s.Home != "/home/me" || !s.Systemd {
		t.Fatalf("got id=%q like=%q home=%q systemd=%v", s.OSID, s.OSLike, s.Home, s.Systemd)
	}
	if s.Disk.Used != 400*1024 {
		t.Fatalf("disk %+v", s.Disk)
	}
}
