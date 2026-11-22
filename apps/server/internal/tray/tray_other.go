//go:build !windows

package tray

import "errors"

// Start is unsupported outside Windows.
func Start(func(), func(), func()) error {
	return errors.New("tray is Windows-only")
}

// Stop is a no-op outside Windows.
func Stop() {}
