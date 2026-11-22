//go:build !windows

package icons

import "errors"

// ForFile is unsupported outside Windows in v1.
func ForFile(string) ([]byte, error) { return nil, errors.New("icons are Windows-only in v1") }

// FromWindow is unsupported outside Windows in v1.
func FromWindow(uintptr) ([]byte, error) { return nil, errors.New("icons are Windows-only in v1") }
