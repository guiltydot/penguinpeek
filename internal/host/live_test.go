package host

import (
	"os"
	"testing"
)

// TestLiveHost reads real Windows state when PEEK_LIVE=1.
func TestLiveHost(t *testing.T) {
	if os.Getenv("PEEK_LIVE") == "" {
		t.Skip("set PEEK_LIVE=1 to read real Windows state")
	}
	ws, err := VMMemory()
	t.Logf("vmmem working set=%.2f GB err=%v", float64(ws)/(1<<30), err)
	p, err := VHDXPath(os.Getenv("PEEK_DISTRO"))
	t.Logf("vhdx=%q err=%v", p, err)
	if p != "" {
		sz, err := FileSize(p)
		t.Logf("vhdx size=%.2f GB err=%v", float64(sz)/(1<<30), err)
	}
	on, err := AutostartEnabled()
	t.Logf("autostart=%v err=%v", on, err)
}
