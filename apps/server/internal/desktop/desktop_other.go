//go:build !windows

package desktop

import "errors"

// List returns nothing on platforms without window enumeration in v1.
func (s *Service) List() []Window { return nil }

// Focus is unsupported outside Windows in v1.
func (s *Service) Focus(string) error {
	return errors.New("window control is Windows-only in v1")
}

// Locked is always false on platforms without lock detection in v1.
func (s *Service) Locked() bool { return false }

// Icon is unsupported outside Windows in v1.
func (s *Service) Icon(string) ([]byte, error) { return nil, errors.New("icons are Windows-only in v1") }
