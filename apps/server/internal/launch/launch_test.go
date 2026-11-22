package launch

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestRunRejectsUnknownApp(t *testing.T) {
	s := &Service{apps: map[string]def{"calc": {Label: "Calculator", Windows: "calc.exe", Linux: "gnome-calculator"}}}
	if err := s.Run("nope"); err == nil || !strings.Contains(err.Error(), "unknown app") {
		t.Fatalf("err = %v, want unknown app", err)
	}
}

func TestRunRejectsMissingOSCommand(t *testing.T) {
	// Entry configured for every OS except the current one: must fail
	// without executing anything.
	var d def
	switch runtime.GOOS {
	case "windows":
		d = def{Label: "X", Linux: "x-toast-that-does-not-exist"}
	case "darwin":
		d = def{Label: "X", Windows: "x-toast-that-does-not-exist"}
	default:
		d = def{Label: "X", Windows: "x-toast-that-does-not-exist"}
	}
	s := &Service{apps: map[string]def{"x": d}}
	if err := s.Run("x"); err == nil || !strings.Contains(err.Error(), "no command") {
		t.Fatalf("err = %v, want no-command error", err)
	}
}

func TestLoadOrSampleCreatesFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "apps.json")
	s, err := LoadOrSample(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("sample file not created: %v", err)
	}
	if len(s.List()) == 0 {
		t.Fatal("sample allowlist is empty")
	}
	// Round-trip: reload parses the written file identically.
	s2, err := LoadOrSample(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(s2.List()) != len(s.List()) {
		t.Fatalf("reload mismatch: %v vs %v", s2.List(), s.List())
	}
}

func TestSearchFindsStartMenuEntries(t *testing.T) {
	progs := filepath.Join(t.TempDir(), "Programs")
	if err := os.MkdirAll(progs, 0o755); err != nil {
		t.Fatal(err)
	}
	ext := ".desktop"
	if runtime.GOOS == "windows" {
		ext = ".lnk"
	}
	for _, name := range []string{"Cool App" + ext, "Other Thing" + ext, "notes.txt"} {
		if err := os.WriteFile(filepath.Join(progs, name), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	s := &Service{apps: map[string]def{}, startDirs: []string{progs}}

	got := s.Search("cool")
	if len(got) != 1 || got[0].Label != "Cool App" {
		t.Fatalf("search cool = %+v", got)
	}
	if !strings.HasPrefix(got[0].ID, startMenuPrefix) {
		t.Fatalf("id = %q, want startmenu: prefix", got[0].ID)
	}
	if len(s.Search("")) != 0 {
		t.Fatal("empty query must return nothing")
	}
	if len(s.Search("zzz-no-such-app")) != 0 {
		t.Fatal("non-matching query must return nothing")
	}
}

func TestRunRejectsOutsideStartMenu(t *testing.T) {
	s := &Service{apps: map[string]def{}, startDirs: []string{t.TempDir()}}
	if err := s.Run(startMenuPrefix + `C:\Windows\evil.lnk`); err == nil {
		t.Fatal("expected rejection of path outside Start Menu dirs")
	}
	if err := s.Run(startMenuPrefix + filepath.Join(t.TempDir(), "missing.lnk")); err == nil {
		t.Fatal("expected rejection of nonexistent shortcut")
	}
}
