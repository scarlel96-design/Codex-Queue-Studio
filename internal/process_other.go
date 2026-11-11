//go:build !windows

package internal

import "os/exec"

func configureHiddenProcess(cmd *exec.Cmd) {}
