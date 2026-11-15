package main

import (
	"os"
	"path/filepath"
)

// webviewDataDir is where WebView2 keeps its profile:
// %LOCALAPPDATA%\PenguinPeek\WebView2.
func webviewDataDir() string {
	base, err := os.UserCacheDir() // %LOCALAPPDATA% on Windows
	if err != nil {
		return "" // Wails falls back to its default
	}
	return filepath.Join(base, "PenguinPeek", "WebView2")
}
