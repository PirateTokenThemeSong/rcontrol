//go:build windows

// Package tray puts an icon in the Windows notification area.
//
// Left click opens the settings UI; right click shows a menu with Settings,
// Open data folder and Exit.
package tray

import (
	"bytes"
	_ "embed"
	"fmt"
	"image"
	"image/png"
	"math"
	"runtime"
	"sync"
	"syscall"
	"unsafe"

	xdraw "golang.org/x/image/draw"
	"golang.org/x/sys/windows"
)

var (
	modUser32  = windows.NewLazySystemDLL("user32.dll")
	modShell32 = windows.NewLazySystemDLL("shell32.dll")
	modGDI32   = windows.NewLazySystemDLL("gdi32.dll")
	modKernel  = windows.NewLazySystemDLL("kernel32.dll")

	procRegisterClassW     = modUser32.NewProc("RegisterClassW")
	procCreateWindowExW    = modUser32.NewProc("CreateWindowExW")
	procDefWindowProcW     = modUser32.NewProc("DefWindowProcW")
	procDestroyWindow      = modUser32.NewProc("DestroyWindow")
	procGetMessageW        = modUser32.NewProc("GetMessageW")
	procTranslateMessage   = modUser32.NewProc("TranslateMessage")
	procDispatchMessageW   = modUser32.NewProc("DispatchMessageW")
	procPostQuitMessage    = modUser32.NewProc("PostQuitMessage")
	procPostMessageW       = modUser32.NewProc("PostMessageW")
	procCreatePopupMenu    = modUser32.NewProc("CreatePopupMenu")
	procAppendMenuW        = modUser32.NewProc("AppendMenuW")
	procDestroyMenu        = modUser32.NewProc("DestroyMenu")
	procTrackPopupMenu     = modUser32.NewProc("TrackPopupMenu")
	procSetForegroundWin   = modUser32.NewProc("SetForegroundWindow")
	procGetCursorPos       = modUser32.NewProc("GetCursorPos")
	procCreateIconIndirect = modUser32.NewProc("CreateIconIndirect")
	procDestroyIcon        = modUser32.NewProc("DestroyIcon")
	procLoadIconW          = modUser32.NewProc("LoadIconW")
	procGetDC              = modUser32.NewProc("GetDC")
	procReleaseDC          = modUser32.NewProc("ReleaseDC")
	procShellNotifyIconW   = modShell32.NewProc("Shell_NotifyIconW")
	procCreateCompatibleDC = modGDI32.NewProc("CreateCompatibleDC")
	procDeleteDC           = modGDI32.NewProc("DeleteDC")
	procCreateDIBSection   = modGDI32.NewProc("CreateDIBSection")
	procCreateBitmap       = modGDI32.NewProc("CreateBitmap")
	procSelectObject       = modGDI32.NewProc("SelectObject")
	procDeleteObject       = modGDI32.NewProc("DeleteObject")
	procGetModuleHandleW   = modKernel.NewProc("GetModuleHandleW")
)

const (
	wmApp          = 0x8000
	wmTrayCallback = wmApp + 1
	wmNull         = 0x0000
	wmClose        = 0x0010
	wmDestroy      = 0x0002
	wmLButtonUp    = 0x0202
	wmLButtonDbl   = 0x0203
	wmRButtonUp    = 0x0205

	nimAdd    = 0x00000000
	nimDelete = 0x00000002
	nifMessage = 0x00000001
	nifIcon    = 0x00000002
	nifTip     = 0x00000004

	mfString    = 0x00000000
	mfSeparator = 0x00000800
	tpmReturnCmd = 0x0100
	tpmNonotify  = 0x0080
)

type wndClass struct {
	Style      uint32
	WndProc    uintptr
	ClsExtra   int32
	WndExtra   int32
	Instance   uintptr
	Icon       uintptr
	Cursor     uintptr
	Background uintptr
	MenuName   *uint16
	ClassName  *uint16
}

type notifyIconData struct {
	CbSize          uint32
	Wnd             uintptr
	ID              uint32
	Flags           uint32
	CallbackMessage uint32
	Icon            uintptr
	Tip             [128]uint16
	State           uint32
	StateMask       uint32
	Info            [256]uint16
	Version         uint32
	InfoTitle       [64]uint16
	InfoFlags       uint32
	GUID            [16]byte
	BalloonIcon     uintptr
}

type point struct{ X, Y int32 }

type msgT struct {
	Hwnd    uintptr
	Message uint32
	WParam  uintptr
	LParam  uintptr
	Time    uint32
	Pt      point
	Private uint32
}

type iconInfo struct {
	FIcon    int32
	XHotspot uint32
	YHotspot uint32
	HbmMask  uintptr
	HbmColor uintptr
}

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

type callbacks struct {
	onSettings func()
	onFolder   func()
	onExit     func()
}

//go:embed icon.png
var Icon []byte

var (
	mu     sync.Mutex
	hwnd   uintptr
	hicon  uintptr
	cb     callbacks
	nid    notifyIconData
	wndProc = syscall.NewCallback(windowProc)
)

// Start installs the tray icon. It returns as soon as the message loop is
// running; Stop removes the icon and ends the loop.
func Start(onSettings, onFolder, onExit func()) error {
	cb = callbacks{onSettings: onSettings, onFolder: onFolder, onExit: onExit}
	ready := make(chan error, 1)
	go func() {
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()
		ready <- run(Icon)
	}()
	return <-ready
}

func run(iconPNG []byte) error {
	hInst, _, _ := procGetModuleHandleW.Call(0)
	className, _ := windows.UTF16PtrFromString("RControlTray")
	wc := wndClass{
		WndProc:   wndProc,
		Instance:  hInst,
		ClassName: className,
	}
	if r, _, e := procRegisterClassW.Call(uintptr(unsafe.Pointer(&wc))); r == 0 {
		return fmt.Errorf("RegisterClass: %v", e)
	}

	// HWND_MESSAGE = -3
	hwndMsg := ^uintptr(2)
	h, _, e := procCreateWindowExW.Call(0,
		uintptr(unsafe.Pointer(className)), 0, 0, 0, 0, 0, 0,
		hwndMsg, 0, hInst, 0)
	if h == 0 {
		return fmt.Errorf("CreateWindowEx: %v", e)
	}
	mu.Lock()
	hwnd = h
	hicon = iconFromPNG(iconPNG)
	mu.Unlock()

	nid = notifyIconData{
		Wnd:             h,
		ID:              1,
		Flags:           nifMessage | nifIcon | nifTip,
		CallbackMessage: wmTrayCallback,
		Icon:            hicon,
	}
	nid.CbSize = uint32(unsafe.Sizeof(nid))
	tip := "rcontrol — LAN PC remote"
	for i, r := range tip {
		if i >= len(nid.Tip)-1 {
			break
		}
		nid.Tip[i] = uint16(r)
	}
	if r, _, e := procShellNotifyIconW.Call(nimAdd, uintptr(unsafe.Pointer(&nid))); r == 0 {
		return fmt.Errorf("Shell_NotifyIcon: %v", e)
	}

	var msg msgT
	for {
		r, _, _ := procGetMessageW.Call(uintptr(unsafe.Pointer(&msg)), 0, 0, 0)
		if int32(r) <= 0 {
			break
		}
		procTranslateMessage.Call(uintptr(unsafe.Pointer(&msg)))
		procDispatchMessageW.Call(uintptr(unsafe.Pointer(&msg)))
	}
	return nil
}

// Stop removes the icon and ends the message loop.
func Stop() {
	mu.Lock()
	h := hwnd
	mu.Unlock()
	if h != 0 {
		procPostMessageW.Call(h, wmClose, 0, 0)
	}
}

func windowProc(h, msg, wparam, lparam uintptr) uintptr {
	switch uint32(msg) {
	case wmTrayCallback:
		switch uint32(lparam) {
		case wmLButtonUp, wmLButtonDbl:
			if cb.onSettings != nil {
				go cb.onSettings()
			}
		case wmRButtonUp:
			showMenu(h)
		}
		return 0
	case wmClose:
		procShellNotifyIconW.Call(nimDelete, uintptr(unsafe.Pointer(&nid)))
		procDestroyWindow.Call(h)
		return 0
	case wmDestroy:
		procPostQuitMessage.Call(0)
		if hicon != 0 {
			procDestroyIcon.Call(hicon)
		}
		return 0
	}
	r, _, _ := procDefWindowProcW.Call(h, msg, wparam, lparam)
	return r
}

func showMenu(h uintptr) {
	menu, _, _ := procCreatePopupMenu.Call()
	if menu == 0 {
		return
	}
	defer procDestroyMenu.Call(menu)
	add := func(id uintptr, text string) {
		p, _ := windows.UTF16PtrFromString(text)
		procAppendMenuW.Call(menu, mfString, id, uintptr(unsafe.Pointer(p)))
	}
	add(1, "Settings…")
	add(2, "Open data folder")
	p, _ := windows.UTF16PtrFromString("")
	procAppendMenuW.Call(menu, mfSeparator, 0, uintptr(unsafe.Pointer(p)))
	add(3, "Exit")

	var pt point
	procGetCursorPos.Call(uintptr(unsafe.Pointer(&pt)))
	procSetForegroundWin.Call(h)
	r, _, _ := procTrackPopupMenu.Call(menu,
		tpmReturnCmd|tpmNonotify,
		uintptr(pt.X), uintptr(pt.Y), 0, h, 0)
	procPostMessageW.Call(h, wmNull, 0, 0)

	switch r {
	case 1:
		if cb.onSettings != nil {
			go cb.onSettings()
		}
	case 2:
		if cb.onFolder != nil {
			go cb.onFolder()
		}
	case 3:
		if cb.onExit != nil {
			go cb.onExit()
		}
	}
}

func iconFromPNG(b []byte) uintptr {
	src, err := png.Decode(bytes.NewReader(b))
	if err != nil {
		return systemIcon()
	}
	const s = 32
	const pad = 1
	dst := image.NewRGBA(image.Rect(0, 0, s, s))
	// The launcher artwork has a wide safe-zone margin. Crop to the visible
	// pixels so the tray icon fills the notification area like its neighbours.
	box := alphaBounds(src)
	bw, bh := box.Dx(), box.Dy()
	if bw <= 0 || bh <= 0 {
		box, bw, bh = src.Bounds(), src.Bounds().Dx(), src.Bounds().Dy()
	}
	inner := float64(s - pad*2)
	scale := math.Min(inner/float64(bw), inner/float64(bh))
	w, h := int(float64(bw)*scale), int(float64(bh)*scale)
	if w < 1 {
		w = 1
	}
	if h < 1 {
		h = 1
	}
	ox, oy := (s-w)/2, (s-h)/2
	xdraw.CatmullRom.Scale(dst, image.Rect(ox, oy, ox+w, oy+h), src, box, xdraw.Over, nil)

	screen, _, _ := procGetDC.Call(0)
	if screen == 0 {
		return systemIcon()
	}
	defer procReleaseDC.Call(0, screen)
	mem, _, _ := procCreateCompatibleDC.Call(screen)
	if mem == 0 {
		return systemIcon()
	}
	defer procDeleteDC.Call(mem)

	bi := bitmapInfoHeader{Size: 40, Width: s, Height: -s, Planes: 1, BitCount: 32}
	var bits unsafe.Pointer
	hbm, _, _ := procCreateDIBSection.Call(mem, uintptr(unsafe.Pointer(&bi)), 0,
		uintptr(unsafe.Pointer(&bits)), 0, 0)
	if hbm == 0 || bits == nil {
		return systemIcon()
	}
	out := unsafe.Slice((*byte)(bits), s*s*4)
	for i := 0; i < s*s; i++ {
		r, g, bl, a := dst.Pix[i*4], dst.Pix[i*4+1], dst.Pix[i*4+2], dst.Pix[i*4+3]
		out[i*4+0] = bl
		out[i*4+1] = g
		out[i*4+2] = r
		out[i*4+3] = a
	}
	mask, _, _ := procCreateBitmap.Call(s, s, 1, 1, 0)
	ii := iconInfo{FIcon: 1, HbmMask: mask, HbmColor: hbm}
	hic, _, _ := procCreateIconIndirect.Call(uintptr(unsafe.Pointer(&ii)))
	procDeleteObject.Call(hbm)
	if mask != 0 {
		procDeleteObject.Call(mask)
	}
	if hic == 0 {
		return systemIcon()
	}
	return hic
}

func systemIcon() uintptr {
	const idiApplication = 32512
	h, _, _ := procLoadIconW.Call(0, idiApplication)
	return h
}

// alphaBounds is the bounding box of non-transparent pixels.
func alphaBounds(img image.Image) image.Rectangle {
	b := img.Bounds()
	minX, minY, maxX, maxY := b.Max.X, b.Max.Y, b.Min.X, b.Min.Y
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			_, _, _, a := img.At(x, y).RGBA()
			if a <= 0x1000 {
				continue
			}
			if x < minX {
				minX = x
			}
			if y < minY {
				minY = y
			}
			if x+1 > maxX {
				maxX = x + 1
			}
			if y+1 > maxY {
				maxY = y + 1
			}
		}
	}
	if minX >= maxX || minY >= maxY {
		return b
	}
	return image.Rect(minX, minY, maxX, maxY)
}
