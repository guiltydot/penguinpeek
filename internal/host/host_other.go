//go:build !windows

// Package host is Windows-only; these stubs keep the module building and
// testable on Linux.
package host

import "errors"

var errUnsupported = errors.New("only supported on Windows")

func VMMemory() (uint64, error)       { return 0, errUnsupported }
func VHDXPath(string) (string, error) { return "", errUnsupported }
func FileSize(string) (uint64, error) { return 0, errUnsupported }
func AutostartEnabled() (bool, error) { return false, errUnsupported }
func SetAutostart(bool) error         { return errUnsupported }
func CompactVHDX(string) error        { return errUnsupported }
