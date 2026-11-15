//go:build !windows

package wsl

import "os/exec"

func hideConsole(*exec.Cmd) {}
func newConsole(*exec.Cmd)  {}
