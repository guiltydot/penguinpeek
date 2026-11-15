package host

import (
	"encoding/base64"
	"strings"
	"testing"
	"unicode/utf16"
)

func decodePS(t *testing.T, enc string) string {
	t.Helper()
	b, err := base64.StdEncoding.DecodeString(enc)
	if err != nil || len(b)%2 != 0 {
		t.Fatalf("bad encoding: %v", err)
	}
	u := make([]uint16, len(b)/2)
	for i := range u {
		u[i] = uint16(b[2*i]) | uint16(b[2*i+1])<<8
	}
	return string(utf16.Decode(u))
}

func TestCompactCommandQuotesPath(t *testing.T) {
	enc, err := compactCommand(`C:\WSL\O'Brien $(calc)\ext4.vhdx`, `C:\Windows\System32`)
	if err != nil {
		t.Fatal(err)
	}
	s := decodePS(t, enc)
	if !strings.HasPrefix(s, "$Vhdx = 'C:\\WSL\\O''Brien $(calc)\\ext4.vhdx'\n$Sys = 'C:\\Windows\\System32'\n") {
		t.Fatalf("script head %q", s[:80])
	}
	if strings.Contains(s, "TEMP") || strings.Contains(s, "/s ") {
		t.Fatal("script must not use temp files")
	}
}

func TestCompactCommandRejectsOddPaths(t *testing.T) {
	for _, p := range []string{
		`ext4.vhdx`,
		`C:\WSL\ext4.vhd`,
		"C:\\WSL\\a\"b.vhdx",
		"C:\\WSL\\x.vhdx\n'; calc",
	} {
		if _, err := compactCommand(p, `C:\Windows\System32`); err == nil {
			t.Errorf("accepted %q", p)
		}
	}
}
