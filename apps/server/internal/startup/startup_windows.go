//go:build windows

// Package startup manages the "start rcontrol at login" entry.
package startup

import (
	"fmt"
	"os"
	"path/filepath"
)

const entryName = "rcontrol server"
const vbsName = "rcontrol server.vbs"

func startupDir() (string, error) {
	appData := os.Getenv("APPDATA")
	if appData == "" {
		return "", fmt.Errorf("APPDATA not set")
	}
	return filepath.Join(appData, `Microsoft\Windows\Start Menu\Programs\Startup`), nil
}

func vbsPath() (string, error) {
	d, err := startupDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(d, vbsName), nil
}

// Enabled reports whether the login entry exists.
func Enabled() bool {
	p, err := vbsPath()
	if err != nil {
		return false
	}
	_, err = os.Stat(p)
	return err == nil
}

// Set creates or removes the login entry. The entry starts the current
// executable hidden, so the server keeps using its saved config.
func Set(enabled bool) error {
	d, err := startupDir()
	if err != nil {
		return err
	}
	vbs, err := vbsPath()
	if err != nil {
		return err
	}
	// Drop the older shortcut-based entry either way.
	_ = os.Remove(filepath.Join(d, entryName+".lnk"))

	if !enabled {
		if err := os.Remove(vbs); err != nil && !os.IsNotExist(err) {
			return err
		}
		return nil
	}
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	body := "Set sh = CreateObject(\"WScript.Shell\")\r\n" +
		"sh.Run \"\"\"" + exe + "\"\"\", 0, False\r\n"
	return os.WriteFile(vbs, []byte(body), 0o644)
}
