//go:build !windows

package main

import "os/exec"

func hideWindow(command *exec.Cmd) {}
