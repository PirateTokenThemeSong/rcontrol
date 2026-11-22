//go:build windows

package screen

import (
	"fmt"
	"image"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	modGDI32               = windows.NewLazySystemDLL("gdi32.dll")
	modUser32              = windows.NewLazySystemDLL("user32.dll")
	procGetDC              = modUser32.NewProc("GetDC")
	procReleaseDC          = modUser32.NewProc("ReleaseDC")
	procGetSystemMetrics   = modUser32.NewProc("GetSystemMetrics")
	procCreateCompatibleDC = modGDI32.NewProc("CreateCompatibleDC")
	procCreateCompatibleBM = modGDI32.NewProc("CreateCompatibleBitmap")
	procSelectObject       = modGDI32.NewProc("SelectObject")
	procBitBlt             = modGDI32.NewProc("BitBlt")
	procDeleteDC           = modGDI32.NewProc("DeleteDC")
	procDeleteObject       = modGDI32.NewProc("DeleteObject")
	procGetDIBits          = modGDI32.NewProc("GetDIBits")

	procSetProcessDpiAwarenessContext = modUser32.NewProc("SetProcessDpiAwarenessContext")
	procSetProcessDPIAware            = modUser32.NewProc("SetProcessDPIAware")
)

// init makes the process DPI-aware before anything measures the screen.
//
// Without it Windows virtualizes GetSystemMetrics to the *scaled* desktop
// (e.g. 2752x1152 for a 3440x1440 monitor at 125%) while GetDC(0) still hands
// back the full framebuffer, so BitBlt copied just the top-left corner of the
// real desktop: a cropped, blurry-looking stream.
func init() {
	const perMonitorAwareV2 = ^uintptr(3) // DPI_AWARENESS_CONTEXT_PER_MONITOR_AWARE_V2 (-4)
	if ok, _, _ := procSetProcessDpiAwarenessContext.Call(perMonitorAwareV2); ok != 0 {
		return
	}
	if p := procSetProcessDPIAware; p.Find() == nil {
		_, _, _ = p.Call()
	}
}

const (
	smCxScreen = 0
	smCyScreen = 1
	srccopy    = 0x00CC0020
	biRGB      = 0
	dibRGB     = 0
)

type bitmapInfoHeader struct {
	Size          uint32
	Width         int32
	Height        int32
	Planes        uint16
	BitCount      uint16
	Compression   uint32
	SizeImage     uint32
	XPelsPerMeter int32
	YPelsPerMeter int32
	ClrUsed       uint32
	ClrImportant  uint32
}

func gdiSize() (int, int, error) {
	w, _, _ := procGetSystemMetrics.Call(uintptr(smCxScreen))
	h, _, _ := procGetSystemMetrics.Call(uintptr(smCyScreen))
	if w <= 0 || h <= 0 {
		return 0, 0, fmt.Errorf("GetSystemMetrics failed")
	}
	return int(w), int(h), nil
}

func gdiCapture() (image.Image, error) {
	w, h, err := gdiSize()
	if err != nil {
		return nil, err
	}
	screenDC, _, _ := procGetDC.Call(0)
	if screenDC == 0 {
		return nil, fmt.Errorf("GetDC failed")
	}
	defer procReleaseDC.Call(0, screenDC)

	memDC, _, _ := procCreateCompatibleDC.Call(screenDC)
	if memDC == 0 {
		return nil, fmt.Errorf("CreateCompatibleDC failed")
	}
	defer procDeleteDC.Call(memDC)

	hbm, _, _ := procCreateCompatibleBM.Call(screenDC, uintptr(w), uintptr(h))
	if hbm == 0 {
		return nil, fmt.Errorf("CreateCompatibleBitmap failed")
	}
	defer procDeleteObject.Call(hbm)

	old, _, _ := procSelectObject.Call(memDC, hbm)
	defer procSelectObject.Call(memDC, old)

	ok, _, _ := procBitBlt.Call(memDC, 0, 0, uintptr(w), uintptr(h), screenDC, 0, 0, uintptr(srccopy))
	if ok == 0 {
		return nil, fmt.Errorf("BitBlt failed")
	}

	// Top-down 32-bit DIB.
	bi := bitmapInfoHeader{
		Size:        40,
		Width:       int32(w),
		Height:      int32(-h),
		Planes:      1,
		BitCount:    32,
		Compression: biRGB,
	}
	pixels := make([]byte, w*h*4)
	ret, _, _ := procGetDIBits.Call(memDC, hbm, 0, uintptr(h),
		uintptr(unsafe.Pointer(&pixels[0])),
		uintptr(unsafe.Pointer(&bi)), uintptr(dibRGB))
	if ret == 0 {
		return nil, fmt.Errorf("GetDIBits failed")
	}

	// BGRA -> RGBA.
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for i := 0; i < w*h; i++ {
		b := pixels[i*4+0]
		g := pixels[i*4+1]
		rr := pixels[i*4+2]
		img.Pix[i*4+0] = rr
		img.Pix[i*4+1] = g
		img.Pix[i*4+2] = b
		img.Pix[i*4+3] = 255
	}
	return img, nil
}

func capturePrimary() (image.Image, error) { return gdiCapture() }

func displaySize() (int, int) {
	if w, h, err := gdiSize(); err == nil {
		return w, h
	}
	return 1920, 1080
}
