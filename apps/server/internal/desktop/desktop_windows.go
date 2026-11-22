//go:build windows

package desktop

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
	"rcontrol-server/internal/icons"
)

var (
	modUser32 = windows.NewLazySystemDLL("user32.dll")
	modKernel = windows.NewLazySystemDLL("kernel32.dll")

	procEnumWindows              = modUser32.NewProc("EnumWindows")
	procIsWindow                 = modUser32.NewProc("IsWindow")
	procIsWindowVisible          = modUser32.NewProc("IsWindowVisible")
	procIsIconic                 = modUser32.NewProc("IsIconic")
	procGetWindowTextW           = modUser32.NewProc("GetWindowTextW")
	procGetWindowTextLengthW     = modUser32.NewProc("GetWindowTextLengthW")
	procGetWindowLongPtrW        = modUser32.NewProc("GetWindowLongPtrW")
	procGetWindow                = modUser32.NewProc("GetWindow")
	procGetForegroundWindow      = modUser32.NewProc("GetForegroundWindow")
	procGetWindowThreadProcessID = modUser32.NewProc("GetWindowThreadProcessId")
	procSetForegroundWindow      = modUser32.NewProc("SetForegroundWindow")
	procBringWindowToTop         = modUser32.NewProc("BringWindowToTop")
	procShowWindow               = modUser32.NewProc("ShowWindow")
	procAttachThreadInput        = modUser32.NewProc("AttachThreadInput")
	procKeybdEvent               = modUser32.NewProc("keybd_event")
	procGetCurrentThreadID       = modKernel.NewProc("GetCurrentThreadId")
)

const (
	wsExToolWindow = 0x00000080
	gwOwner        = 4
	swRestore      = 9
	vkMenu         = 0x12
	keyeventfKeyUp = 0x0002
	maxWindows     = 64
)

// GWL_EXSTYLE is -20; keep it as a uintptr because that is what the syscall takes.
var gwlExStyle = ^uintptr(19)

// List returns visible top-level windows in z-order (front-most first).
func (s *Service) List() []Window {
	fg, _, _ := procGetForegroundWindow.Call()
	out := make([]Window, 0, 24)

	cb := syscall.NewCallback(func(hwnd uintptr, _ uintptr) uintptr {
		if len(out) >= maxWindows {
			return 0 // stop enumerating
		}
		if v, _, _ := procIsWindowVisible.Call(hwnd); v == 0 {
			return 1
		}
		// Skip tool windows and owned popups/dialogs: they are not windows the
		// user thinks of when switching.
		if ex, _, _ := procGetWindowLongPtrW.Call(hwnd, gwlExStyle); ex&wsExToolWindow != 0 {
			return 1
		}
		if owner, _, _ := procGetWindow.Call(hwnd, gwOwner); owner != 0 {
			return 1
		}
		n, _, _ := procGetWindowTextLengthW.Call(hwnd)
		if n == 0 {
			return 1
		}
		buf := make([]uint16, n+1)
		if r, _, _ := procGetWindowTextW.Call(hwnd, uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf))); r == 0 {
			return 1
		}
		title := strings.TrimSpace(windows.UTF16ToString(buf))
		if title == "" {
			return 1
		}
		minimized, _, _ := procIsIconic.Call(hwnd)
		out = append(out, Window{
			ID:        strconv.FormatUint(uint64(hwnd), 10),
			Title:     title,
			Active:    hwnd == fg,
			Minimized: minimized != 0,
		})
		return 1
	})
	_, _, _ = procEnumWindows.Call(cb, 0)
	return out
}

// Focus restores (if minimized) and brings the window with this id to the front.
func (s *Service) Focus(id string) error {
	n, err := strconv.ParseUint(strings.TrimSpace(id), 10, 64)
	if err != nil || n == 0 {
		return fmt.Errorf("bad window id %q", id)
	}
	hwnd := uintptr(n)
	if ok, _, _ := procIsWindow.Call(hwnd); ok == 0 {
		return fmt.Errorf("window is gone")
	}
	if icon, _, _ := procIsIconic.Call(hwnd); icon != 0 {
		_, _, _ = procShowWindow.Call(hwnd, uintptr(swRestore))
	}

	fg, _, _ := procGetForegroundWindow.Call()
	if fg == hwnd {
		return nil
	}

	// A background process may not be allowed to steal the foreground; attach
	// to the foreground thread's input queue first, then fall back to the
	// classic Alt-key nudge if Windows still refuses.
	fgThread, _, _ := procGetWindowThreadProcessID.Call(fg, 0)
	curThread, _, _ := procGetCurrentThreadID.Call()
	if fgThread != 0 && fgThread != curThread {
		if ok, _, _ := procAttachThreadInput.Call(curThread, fgThread, 1); ok != 0 {
			defer procAttachThreadInput.Call(curThread, fgThread, 0)
		}
	}

	_, _, _ = procBringWindowToTop.Call(hwnd)
	if ok, _, _ := procSetForegroundWindow.Call(hwnd); ok != 0 {
		return nil
	}

	_, _, _ = procKeybdEvent.Call(vkMenu, 0, 0, 0)
	_, _, _ = procKeybdEvent.Call(vkMenu, 0, keyeventfKeyUp, 0)
	if ok, _, _ := procSetForegroundWindow.Call(hwnd); ok != 0 {
		return nil
	}
	return fmt.Errorf("windows refused to switch to that window")
}

// Locked reports whether Windows is showing the lock / sign-in screen.
//
// On Windows 11 the lock screen is owned by LockApp.exe (older builds:
// LogonUI.exe), and it becomes the foreground window while the session is
// locked. A session with no foreground window at all is also treated as locked.
func (s *Service) Locked() bool {
	fg, _, _ := procGetForegroundWindow.Call()
	if fg == 0 {
		return true
	}
	var pid uint32
	procGetWindowThreadProcessID.Call(fg, uintptr(unsafe.Pointer(&pid)))
	if pid == 0 {
		return false
	}
	h, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, pid)
	if err != nil {
		return false
	}
	defer windows.CloseHandle(h)
	buf := make([]uint16, 512)
	size := uint32(len(buf))
	if err := windows.QueryFullProcessImageName(h, 0, &buf[0], &size); err != nil {
		return false
	}
	exe := strings.ToLower(filepath.Base(windows.UTF16ToString(buf[:size])))
	return exe == "lockapp.exe" || exe == "logonui.exe"
}

// Icon returns the window's PNG icon, falling back to its process executable.
func (s *Service) Icon(id string) ([]byte, error) {
	n, err := strconv.ParseUint(strings.TrimSpace(id), 10, 64)
	if err != nil || n == 0 {
		return nil, fmt.Errorf("bad window id %q", id)
	}
	hwnd := uintptr(n)
	if ok, _, _ := procIsWindow.Call(hwnd); ok == 0 {
		return nil, fmt.Errorf("window is gone")
	}
	if png, err := icons.FromWindow(hwnd); err == nil {
		return png, nil
	}
	var pid uint32
	procGetWindowThreadProcessID.Call(hwnd, uintptr(unsafe.Pointer(&pid)))
	if pid == 0 {
		return nil, fmt.Errorf("no icon")
	}
	h, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, pid)
	if err != nil {
		return nil, err
	}
	defer windows.CloseHandle(h)
	buf := make([]uint16, 1024)
	size := uint32(len(buf))
	if err := windows.QueryFullProcessImageName(h, 0, &buf[0], &size); err != nil {
		return nil, err
	}
	return icons.ForFile(windows.UTF16ToString(buf[:size]))
}
