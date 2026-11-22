//go:build !windows

package screen

import (
	"bytes"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"os/exec"
)

func displaySize() (int, int) { return 1920, 1080 }

func gdiSize() (int, int, error) { return 0, 0, fmt.Errorf("not windows") }

func gdiCapture() (image.Image, error) { return nil, fmt.Errorf("not windows") }

func capturePrimary() (image.Image, error) {
	for _, tool := range [][]string{
		{"scrot", "-o", "-"},
		{"import", "-window", "root", "jpg:-"},
		{"grim", "-"},
	} {
		if _, lookErr := exec.LookPath(tool[0]); lookErr != nil {
			continue
		}
		cmd := exec.Command(tool[0], tool[1:]...)
		var buf bytes.Buffer
		cmd.Stdout = &buf
		if err := cmd.Run(); err != nil {
			continue
		}
		if decoded, _, decErr := image.Decode(&buf); decErr == nil {
			return decoded, nil
		}
	}
	return nil, fmt.Errorf("screen capture not supported on this OS (install scrot/grim)")
}
