// Package unlock talks to the optional rcontrol unlock helper service.
//
// Windows blocks SendInput from a normal process on the lock screen, so the
// unlock helper runs as SYSTEM, switches to the input desktop (RustDesk's
// desktop mask) and types the secret there. The helper is installed once by
// the user (cmd/rcontrol-unlock) and listens on loopback with a shared token.
package unlock

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	// Port is where the helper listens (loopback only).
	Port = 7947
	// ServiceName is the Windows service name.
	ServiceName = "rcontrol-unlock"
)

func dir() string {
	if d := os.Getenv("ProgramData"); d != "" {
		return filepath.Join(d, "rcontrol")
	}
	return `C:\ProgramData\rcontrol`
}

// TokenPath is where the shared helper token lives.
func TokenPath() string { return filepath.Join(dir(), "helper.token") }

func readToken() (string, error) {
	b, err := os.ReadFile(TokenPath())
	if err != nil {
		return "", err
	}
	t := strings.TrimSpace(string(b))
	if t == "" {
		return "", errors.New("empty helper token")
	}
	return t, nil
}

// Available reports whether the helper looks installed.
func Available() bool { _, err := readToken(); return err == nil }

// Unlock asks the helper to type the secret into the lock screen and press Enter.
func Unlock(secret string) error {
	if strings.TrimSpace(secret) == "" {
		return errors.New("empty secret")
	}
	if len([]rune(secret)) > 256 {
		return errors.New("secret too long")
	}
	tok, err := readToken()
	if err != nil {
		return errors.New("unlock helper is not installed (run rcontrol-unlock -install as admin)")
	}
	body, _ := json.Marshal(map[string]string{"secret": secret})
	req, err := http.NewRequest(http.MethodPost, fmt.Sprintf("http://127.0.0.1:%d/unlock", Port), bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+tok)
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{Timeout: 25 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("unlock helper unreachable: %w", err)
	}
	defer resp.Body.Close()
	var out struct {
		OK    bool   `json:"ok"`
		Error string `json:"error"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&out)
	if resp.StatusCode != http.StatusOK || !out.OK {
		if out.Error != "" {
			return errors.New(out.Error)
		}
		return fmt.Errorf("unlock helper: HTTP %d", resp.StatusCode)
	}
	return nil
}

// Helper adapts the package functions to an interface for the transport.
type Helper struct{}

// Unlock forwards to the helper service.
func (Helper) Unlock(secret string) error { return Unlock(secret) }
