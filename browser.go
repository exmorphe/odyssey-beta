package main

import (
	"errors"
	"os"
	"os/exec"
	"runtime"
)

var openBrowser = defaultOpenBrowser

var shouldAutoOpen = func() bool {
	info, err := os.Stdout.Stat()
	tty := err == nil && (info.Mode()&os.ModeCharDevice) != 0
	return autoOpenAllowed(tty)
}

func autoOpenAllowed(stdoutIsTTY bool) bool {
	if os.Getenv("NO_BROWSER") != "" {
		return false
	}
	return stdoutIsTTY
}

func defaultOpenBrowser(url string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "linux":
		cmd = exec.Command("xdg-open", url)
	case "darwin":
		cmd = exec.Command("open", url)
	case "windows":
		cmd = exec.Command("cmd", "/c", "start", "", url)
	default:
		return errors.New("unsupported platform for auto-open")
	}
	return cmd.Start()
}
