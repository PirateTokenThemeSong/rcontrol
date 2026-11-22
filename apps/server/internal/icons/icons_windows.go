//go:build windows

package icons

import (
	"bytes"
	"fmt"
	"image"
	"image/png"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	modUser32  = windows.NewLazySystemDLL("user32.dll")
	modGDI32   = windows.NewLazySystemDLL("gdi32.dll")
	modShell32 = windows.NewLazySystemDLL("shell32.dll")

	procSHGetFileInfoW     = modShell32.NewProc("SHGetFileInfoW")
	procDestroyIcon        = modUser32.NewProc("DestroyIcon")
	procDrawIconEx         = modUser32.NewProc("DrawIconEx")
	procGetDC              = modUser32.NewProc("GetDC")
	procReleaseDC          = modUser32.NewProc("ReleaseDC")
	procSendMessageTimeout = modUser32.NewProc("SendMessageTimeoutW")
	procGetClassLongPtrW   = modUser32.NewProc("GetClassLongPtrW")
	procGetWindowThreadPID = modUser32.NewProc("GetWindowThreadProcessId")

	procCreateCompatibleDC = modGDI32.NewProc("CreateCompatibleDC")
	procDeleteDC           = modGDI32.NewProc("DeleteDC")
	procCreateDIBSection   = modGDI32.NewProc("CreateDIBSection")
	procSelectObject       = modGDI32.NewProc("SelectObject")
	procDeleteObject       = modGDI32.NewProc("DeleteObject")
)

type shFileInfo struct {
	HIcon         uintptr
	IIcon         int32
	DwAttributes  uint32
	SzDisplayName [260]uint16
	SzTypeName    [80]uint16
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

const (
	shgfiIcon      = 0x000000100
	shgfiLargeIcon = 0x000000000
	wmGetIcon      = 0x007F
	iconBig        = 1
	iconSmall2     = 2
	iconSmall      = 0
	smtoAbortIfHung = 0x0002
	diNormal       = 3
)

// ForFile returns the PNG icon Windows associates with a file (exe, .lnk, …).
func ForFile(path string) ([]byte, error) {
	p, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return nil, err
	}
	var sfi shFileInfo
	r, _, e := procSHGetFileInfoW.Call(
		uintptr(unsafe.Pointer(p)), 0,
		uintptr(unsafe.Pointer(&sfi)), unsafe.Sizeof(sfi),
		shgfiIcon|shgfiLargeIcon,
	)
	if r == 0 || sfi.HIcon == 0 {
		return nil, fmt.Errorf("SHGetFileInfo %q: %v", path, e)
	}
	defer procDestroyIcon.Call(sfi.HIcon)
	return hiconPNG(sfi.HIcon)
}

// FromWindow returns the PNG icon of a top-level window, falling back to its class icon.
func FromWindow(hwnd uintptr) ([]byte, error) {
	for _, which := range []uintptr{iconBig, iconSmall2, iconSmall} {
		var res uintptr
		r, _, _ := procSendMessageTimeout.Call(hwnd, wmGetIcon, which, 0, smtoAbortIfHung, 150,
			uintptr(unsafe.Pointer(&res)))
		if r != 0 && res != 0 {
			return hiconPNG(res)
		}
	}
	if h, _, _ := procGetClassLongPtrW.Call(hwnd, ^uintptr(13)); h != 0 { // GCLP_HICON
		return hiconPNG(h)
	}
	return nil, fmt.Errorf("window has no icon")
}

func hiconPNG(hicon uintptr) ([]byte, error) {
	if hicon == 0 {
		return nil, fmt.Errorf("nil icon")
	}
	screen, _, _ := procGetDC.Call(0)
	if screen == 0 {
		return nil, fmt.Errorf("GetDC failed")
	}
	defer procReleaseDC.Call(0, screen)

	mem, _, _ := procCreateCompatibleDC.Call(screen)
	if mem == 0 {
		return nil, fmt.Errorf("CreateCompatibleDC failed")
	}
	defer procDeleteDC.Call(mem)

	bi := bitmapInfoHeader{
		Size: 40, Width: Size, Height: -Size, Planes: 1, BitCount: 32,
	}
	var bits unsafe.Pointer
	hbm, _, _ := procCreateDIBSection.Call(mem, uintptr(unsafe.Pointer(&bi)), 0,
		uintptr(unsafe.Pointer(&bits)), 0, 0)
	if hbm == 0 || bits == nil {
		return nil, fmt.Errorf("CreateDIBSection failed")
	}
	old, _, _ := procSelectObject.Call(mem, hbm)
	defer func() {
		procSelectObject.Call(mem, old)
		procDeleteObject.Call(hbm)
	}()

	if r, _, e := procDrawIconEx.Call(mem, 0, 0, hicon, Size, Size, 0, 0, diNormal); r == 0 {
		return nil, fmt.Errorf("DrawIconEx: %v", e)
	}

	pix := unsafe.Slice((*byte)(bits), Size*Size*4)
	img := image.NewRGBA(image.Rect(0, 0, Size, Size))
	for i := 0; i < Size*Size; i++ {
		b, g, r, a := pix[i*4], pix[i*4+1], pix[i*4+2], pix[i*4+3]
		if a == 0 && (r|g|b) != 0 {
			a = 255 // GDI leaves the alpha channel empty for mask-based icons
		}
		img.Pix[i*4+0] = r
		img.Pix[i*4+1] = g
		img.Pix[i*4+2] = b
		img.Pix[i*4+3] = a
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
