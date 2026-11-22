// Package launch manages apps the phone may start.
//
// Two sources:
//
//  1. Allowlist file ~/.rcontrol/apps.json (created with samples on first run):
//     hand-picked entries with explicit per-OS commands.
//  2. Start Menu search: the phone can search installed apps by name
//     (Windows .lnk shortcuts, Linux .desktop files) and launch them.
//     Only files inside the real Start Menu / applications directories can
//     be launched, and the phone only ever sees ids + labels.
package launch

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
)

// App is the public list item sent to the phone.
type App struct {
	ID    string `json:"id"`
	Label string `json:"label"`
}

// def is one entry of the JSON config file.
type def struct {
	Label   string   `json:"label"`
	Windows string   `json:"windows,omitempty"`
	Linux   string   `json:"linux,omitempty"`
	Darwin  string   `json:"darwin,omitempty"`
	Args    []string `json:"args,omitempty"`
	Dir     string   `json:"dir,omitempty"`
}

// Service launches allowlisted and Start Menu apps.
type Service struct {
	apps      map[string]def
	startDirs []string
}

// startMenuPrefix marks ids that refer to Start Menu entries.
const startMenuPrefix = "startmenu:"

// sample is written when no config exists yet.
const sample = `{
  "steam":   {"label": "Steam",   "windows": "C:\\Program Files (x86)\\Steam\\steam.exe", "linux": "steam"},
  "notepad": {"label": "Notepad", "windows": "notepad.exe"}
}
`

// DefaultPath returns ~/.rcontrol/apps.json.
func DefaultPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "apps.json"
	}
	return filepath.Join(home, ".rcontrol", "apps.json")
}

// defaultStartDirs returns the system Start Menu / applications directories.
func defaultStartDirs() []string {
	if runtime.GOOS == "windows" {
		var dirs []string
		if pd := os.Getenv("ProgramData"); pd != "" {
			dirs = append(dirs, filepath.Join(pd, `Microsoft\Windows\Start Menu\Programs`))
		}
		if ad := os.Getenv("AppData"); ad != "" {
			dirs = append(dirs, filepath.Join(ad, `Microsoft\Windows\Start Menu\Programs`))
		}
		return dirs
	}
	var dirs []string
	dirs = append(dirs, "/usr/share/applications")
	if home, err := os.UserHomeDir(); err == nil {
		dirs = append(dirs, filepath.Join(home, ".local", "share", "applications"))
	}
	return dirs
}

// LoadOrSample loads the config at path (or the default path when empty),
// writing a sample file first if none exists.
func LoadOrSample(path string) (*Service, error) {
	if strings.TrimSpace(path) == "" {
		path = DefaultPath()
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, err
	}
	if _, err := os.Stat(path); os.IsNotExist(err) {
		if err := os.WriteFile(path, []byte(sample), 0o600); err != nil {
			return nil, fmt.Errorf("write sample apps file: %w", err)
		}
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var parsed map[string]def
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	if parsed == nil {
		parsed = map[string]def{}
	}
	return &Service{apps: parsed, startDirs: defaultStartDirs()}, nil
}

// List returns the allowlisted apps sorted by id (labels only — no paths).
func (s *Service) List() []App {
	out := make([]App, 0, len(s.apps))
	for id, d := range s.apps {
		label := d.Label
		if label == "" {
			label = id
		}
		out = append(out, App{ID: id, Label: label})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

// Search finds installed Start Menu apps whose name contains q
// (case-insensitive). Empty query returns nothing. Capped at 25 results.
func (s *Service) Search(q string) []App {
	q = strings.ToLower(strings.TrimSpace(q))
	if q == "" {
		return []App{}
	}
	var ext string
	if runtime.GOOS == "windows" {
		ext = ".lnk"
	} else {
		ext = ".desktop"
	}
	out := []App{}
	for _, dir := range s.startDirs {
		_ = filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() || len(out) >= 25 {
				return nil
			}
			if !strings.EqualFold(filepath.Ext(path), ext) {
				return nil
			}
			name := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
			if !strings.Contains(strings.ToLower(name), q) {
				return nil
			}
			abs, err := filepath.Abs(path)
			if err != nil {
				return nil
			}
			out = append(out, App{ID: startMenuPrefix + abs, Label: name})
			return nil
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Label < out[j].Label })
	return out
}

// Run starts an allowlisted app or a Start Menu entry by id.
// Unknown ids, and paths escaping the Start Menu dirs, are rejected.
func (s *Service) Run(id string) error {
	id = strings.TrimSpace(id)
	if d, ok := s.apps[id]; ok {
		return s.runDef(id, d)
	}
	if strings.HasPrefix(id, startMenuPrefix) {
		return s.runStartMenu(strings.TrimPrefix(id, startMenuPrefix))
	}
	return fmt.Errorf("unknown app %q", id)
}

func (s *Service) runDef(id string, d def) error {
	var cmd string
	switch runtime.GOOS {
	case "windows":
		cmd = d.Windows
	case "darwin":
		cmd = d.Darwin
	default:
		cmd = d.Linux
	}
	if strings.TrimSpace(cmd) == "" {
		return fmt.Errorf("app %q has no command configured for %s", id, runtime.GOOS)
	}
	c := exec.Command(cmd, d.Args...)
	if d.Dir != "" {
		c.Dir = d.Dir
	}
	// Start detached: the phone gets the ack while the app keeps running.
	return c.Start()
}

func (s *Service) runStartMenu(path string) error {
	abs, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("bad app path: %w", err)
	}
	inside := false
	for _, dir := range s.startDirs {
		absDir, err := filepath.Abs(dir)
		if err != nil {
			continue
		}
		if abs == absDir || strings.HasPrefix(abs, absDir+string(filepath.Separator)) {
			inside = true
			break
		}
	}
	if !inside {
		return fmt.Errorf("app path outside Start Menu directories")
	}
	info, err := os.Stat(abs)
	if err != nil || info.IsDir() {
		return fmt.Errorf("app shortcut not found")
	}
	if runtime.GOOS == "windows" {
		if !strings.EqualFold(filepath.Ext(abs), ".lnk") {
			return fmt.Errorf("not a Start Menu shortcut")
		}
		// cmd's `start` resolves the .lnk target.
		return exec.Command("cmd", "/c", "start", "", abs).Start()
	}
	if strings.EqualFold(filepath.Ext(abs), ".desktop") {
		base := strings.TrimSuffix(filepath.Base(abs), filepath.Ext(abs))
		if _, err := exec.LookPath("gtk-launch"); err == nil {
			return exec.Command("gtk-launch", base).Start()
		}
	}
	return exec.Command("xdg-open", abs).Start()
}

// PathFor returns a local file (exe or Start Menu shortcut) for an app id,
// so callers can read its icon. Unknown ids return ok=false.
func (s *Service) PathFor(id string) (string, bool) {
	id = strings.TrimSpace(id)
	if d, ok := s.apps[id]; ok {
		var cmd string
		switch runtime.GOOS {
		case "windows":
			cmd = d.Windows
		case "darwin":
			cmd = d.Darwin
		default:
			cmd = d.Linux
		}
		cmd = strings.TrimSpace(cmd)
		if cmd == "" {
			return "", false
		}
		if !strings.ContainsAny(cmd, `\/`) {
			if p, err := exec.LookPath(cmd); err == nil {
				return p, true
			}
			return "", false
		}
		return cmd, true
	}
	if strings.HasPrefix(id, startMenuPrefix) {
		p := strings.TrimPrefix(id, startMenuPrefix)
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			return p, true
		}
	}
	return "", false
}
