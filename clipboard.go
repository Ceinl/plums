package main

import (
	"os/exec"
	"strings"
)

var writeClipboard = func(text string) error {
	cmd := exec.Command("pbcopy")
	cmd.Stdin = strings.NewReader(text)
	return cmd.Run()
}
