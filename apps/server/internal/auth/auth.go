// Package auth manages the pairing PIN and bearer token.
// Files live in ~/.rcontrol: server_id, pin, token (0600 where possible).
package auth

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// Store holds persistent credentials.
type Store struct {
	mu       sync.RWMutex
	dir      string
	serverID string
	pin      string
	token    string
}

// Dir returns the config directory.
func (s *Store) Dir() string { return s.dir }

// PIN returns the pairing PIN (format 000-000).
func (s *Store) PIN() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.pin
}

// ServerID returns the stable server id advertised in discovery.
func (s *Store) ServerID() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.serverID
}

// LoadOrCreate loads ~/.rcontrol credentials or creates them.
func LoadOrCreate() (*Store, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	dir := filepath.Join(home, ".rcontrol")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, err
	}
	s := &Store{dir: dir}
	s.serverID, err = loadOrGen(filepath.Join(dir, "server_id"), func() string { return genHex(8) })
	if err != nil {
		return nil, err
	}
	rawPIN, err := loadOrGen(filepath.Join(dir, "pin"), genPIN)
	if err != nil {
		return nil, err
	}
	s.pin = formatPIN(rawPIN)
	s.token, err = loadOrGen(filepath.Join(dir, "token"), func() string { return genHex(32) })
	if err != nil {
		return nil, err
	}
	return s, nil
}

// CheckToken constant-time compares a presented token.
func (s *Store) CheckToken(presented string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	a := []byte(strings.TrimSpace(presented))
	b := []byte(s.token)
	if len(a) != len(b) || len(a) == 0 {
		return false
	}
	return subtle.ConstantTimeCompare(a, b) == 1
}

// CheckPIN compares a presented PIN (dashes/spaces ignored).
func (s *Store) CheckPIN(presented string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	a := normalizePIN(presented)
	b := normalizePIN(s.pin)
	if len(a) != 6 || len(b) != 6 {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}

// Token returns the current bearer token (for /pair responses).
func (s *Store) Token() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.token
}

// RotatePIN generates a new pairing PIN and persists it.
func (s *Store) RotatePIN() (string, error) {
	raw := genPIN()
	if err := os.WriteFile(filepath.Join(s.dir, "pin"), []byte(raw+"\n"), 0o600); err != nil {
		return "", err
	}
	s.mu.Lock()
	s.pin = formatPIN(raw)
	pin := s.pin
	s.mu.Unlock()
	return pin, nil
}

// RotateToken invalidates every paired device and returns the new token.
func (s *Store) RotateToken() (string, error) {
	t := genHex(32)
	if err := os.WriteFile(filepath.Join(s.dir, "token"), []byte(t+"\n"), 0o600); err != nil {
		return "", err
	}
	s.mu.Lock()
	s.token = t
	s.mu.Unlock()
	return t, nil
}

// Dir returns the config directory.
func (s *Store) ConfigDir() string { return s.dir }

func loadOrGen(path string, gen func() string) (string, error) {
	if b, err := os.ReadFile(path); err == nil {
		if v := strings.TrimSpace(string(b)); v != "" {
			return v, nil
		}
	}
	v := gen()
	if err := os.WriteFile(path, []byte(v+"\n"), 0o600); err != nil {
		return "", fmt.Errorf("write %s: %w", path, err)
	}
	return v, nil
}

func genHex(nBytes int) string {
	b := make([]byte, nBytes)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func genPIN() string {
	// 6 digits, may have leading zeros.
	b := make([]byte, 3)
	_, _ = rand.Read(b)
	n := int(b[0])<<16 | int(b[1])<<8 | int(b[2])
	n %= 1000000
	return fmt.Sprintf("%06d", n)
}

func normalizePIN(p string) string {
	p = strings.ReplaceAll(p, "-", "")
	p = strings.ReplaceAll(p, " ", "")
	return strings.TrimSpace(p)
}

func formatPIN(raw string) string {
	d := normalizePIN(raw)
	if len(d) != 6 {
		return raw
	}
	return d[:3] + "-" + d[3:]
}
