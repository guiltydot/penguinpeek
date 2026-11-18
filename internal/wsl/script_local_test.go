package wsl

import (
	"os"
	"os/exec"
	"runtime"
	"testing"
)

// TestMetricsScriptLocal runs the sampler under the local /bin/sh (Linux
// only) and checks that it still parses: the script is one long string, so a
// quoting mistake would otherwise only show up inside a real distro.
func TestMetricsScriptLocal(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("needs a Linux /proc")
	}
	if _, err := os.Stat("/bin/sh"); err != nil {
		t.Skip("no /bin/sh")
	}
	out, err := exec.Command("/bin/sh", "-c", metricsScript, "sh", "1", "2").CombinedOutput()
	if err != nil {
		t.Fatalf("script failed: %v\n%s", err, out)
	}
	s := ParseSample(string(out))
	if s.UptimeSec == 0 || len(s.Procs) == 0 || s.Mem.Total == 0 {
		t.Fatalf("incomplete sample: up=%v procs=%d mem=%+v", s.UptimeSec, len(s.Procs), s.Mem)
	}
	t.Logf("copilot=%v copilotd=%v", s.Procs["copilot"], s.Procs["copilotd"])
}
