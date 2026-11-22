//go:build !windows

package startup

import "errors"

// Enabled is always false outside Windows.
func Enabled() bool { return false }

// Set is unsupported outside Windows.
func Set(bool) error { return errors.New("startup entry is Windows-only") }
