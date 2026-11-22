// Package screen captures the primary display and encodes JPEG frames.
//
// Windows: native GDI BitBlt (no CGO, no screenshot dependency).
// Linux: tries `scrot` / `import` / `grim` fallbacks; returns a clear error otherwise.
package screen

import (
	"bytes"
	"fmt"
	"image"
	"image/jpeg"
	"sync"

	"golang.org/x/image/draw"
)

// Quality controls frame rate and encoding.
type Quality struct {
	FPS      int // 1..15
	JPEG     int // 30..90
	MaxWidth int // max encoded width, aspect preserved
}

func (q Quality) normalized() Quality {
	if q.FPS < 1 {
		q.FPS = 1
	}
	if q.FPS > 15 {
		q.FPS = 15
	}
	if q.JPEG < 30 {
		q.JPEG = 30
	}
	if q.JPEG > 90 {
		q.JPEG = 90
	}
	if q.MaxWidth < 320 {
		q.MaxWidth = 320
	}
	if q.MaxWidth > 1920 {
		q.MaxWidth = 1920
	}
	return q
}

// Capturer is safe for concurrent use.
type Capturer struct {
	mu sync.Mutex
	q  Quality
}

// NewCapturer returns a Capturer with phone-friendly defaults.
func NewCapturer() *Capturer {
	return &Capturer{q: Quality{FPS: 5, JPEG: 70, MaxWidth: 960}}
}

// SetQuality updates encoding quality.
func (c *Capturer) SetQuality(q Quality) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.q = q.normalized()
}

// GetQuality returns a copy of the current quality.
func (c *Capturer) GetQuality() Quality {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.q
}

// Bounds returns the primary display bounds.
func (c *Capturer) Bounds() image.Rectangle {
	w, h := displaySize()
	return image.Rect(0, 0, w, h)
}

// Frame captures, downscales, and JPEG-encodes one frame.
func (c *Capturer) Frame() (data []byte, w, h int, err error) {
	c.mu.Lock()
	q := c.q
	c.mu.Unlock()

	src, err := capturePrimary()
	if err != nil {
		return nil, 0, 0, err
	}
	img := downscale(src, q.MaxWidth)
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: q.JPEG}); err != nil {
		return nil, 0, 0, fmt.Errorf("jpeg encode: %w", err)
	}
	b := img.Bounds()
	return buf.Bytes(), b.Dx(), b.Dy(), nil
}

func downscale(src image.Image, maxWidth int) image.Image {
	b := src.Bounds()
	w := b.Dx()
	if w <= maxWidth {
		return src
	}
	scale := float64(maxWidth) / float64(w)
	newW := maxWidth
	newH := int(float64(b.Dy()) * scale)
	if newH < 1 {
		newH = 1
	}
	dst := image.NewRGBA(image.Rect(0, 0, newW, newH))
	draw.CatmullRom.Scale(dst, dst.Bounds(), src, b, draw.Over, nil)
	return dst
}
