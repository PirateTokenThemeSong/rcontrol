//go:build windows

// rcontrol-unlock is the optional Windows helper that types into the lock
// screen. It runs as a SYSTEM service, spawns a typist in the logged-in
// session with the winlogon token, switches that thread to the input desktop
// (the same mask TigerVNC/RustDesk use) and sends the secret with SendInput.
//
// Install once, as admin:
//
//	rcontrol-unlock.exe -install
package main

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/mgr"
)

const serviceName = "rcontrol-unlock"

var (
	modUser32   = windows.NewLazySystemDLL("user32.dll")
	modAdvapi32 = windows.NewLazySystemDLL("advapi32.dll")
	modKernel32 = windows.NewLazySystemDLL("kernel32.dll")

	procOpenInputDesktop  = modUser32.NewProc("OpenInputDesktop")
	procSetThreadDesktop  = modUser32.NewProc("SetThreadDesktop")
	procCloseDesktop      = modUser32.NewProc("CloseDesktop")
	procGetThreadDesktop  = modUser32.NewProc("GetThreadDesktop")
	procGetCurrentTID     = modKernel32.NewProc("GetCurrentThreadId")
	procSendInput         = modUser32.NewProc("SendInput")
	procWTSActiveSession  = modKernel32.NewProc("WTSGetActiveConsoleSessionId")
	procProcessSession    = modKernel32.NewProc("ProcessIdToSessionId")
	procCreateProcessAsU  = modAdvapi32.NewProc("CreateProcessAsUserW")
	procSetTokenInfo      = modAdvapi32.NewProc("SetTokenInformation")
	procGetCurrentPID     = modKernel32.NewProc("GetCurrentProcessId")
	procGetUserObjectInfo = modUser32.NewProc("GetUserObjectInformationW")
)

// TigerVNC/RustDesk's mask: everything plus GENERIC_WRITE. Requesting only
// DESKTOP_SWITCHDESKTOP is not enough — SendInput then fails with Access denied.
const desktopRights = 0x0002 | 0x0004 | 0x0010 | 0x0020 | 0x0080 | 0x0001 | 0x0100 | 0x10000000

func main() {
	install := flag.Bool("install", false, "install and start the service (admin)")
	uninstall := flag.Bool("uninstall", false, "stop and remove the service (admin)")
	service := flag.Bool("service", false, "run as the service (internal)")
	typist := flag.String("typist", "", "type the contents of this file (internal)")
	enter := flag.Bool("enter", false, "press Enter after typing (internal)")
	flag.Parse()

	switch {
	case *install:
		must(installService())
		fmt.Println("installed and started", serviceName)
	case *uninstall:
		must(uninstallService())
		fmt.Println("removed", serviceName)
	case *service:
		must(runService())
	case *typist != "":
		os.Exit(typistMain(*typist, *enter))
	default:
		flag.Usage()
	}
}

func must(err error) {
	if err != nil {
		fmt.Println("error:", err)
		os.Exit(1)
	}
}

func dataDir() string {
	if d := os.Getenv("ProgramData"); d != "" {
		return filepath.Join(d, "rcontrol")
	}
	return `C:\ProgramData\rcontrol`
}

func tokenPath() string { return filepath.Join(dataDir(), "helper.token") }
func logPath() string   { return filepath.Join(dataDir(), "helper.log") }

func logf(format string, args ...any) {
	f, err := os.OpenFile(logPath(), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	defer f.Close()
	fmt.Fprintf(f, "%s %s\n", time.Now().Format("2006-01-02 15:04:05"), fmt.Sprintf(format, args...))
}

// ---- install ----

func installService() error {
	// Stop an existing instance first, otherwise its exe is locked.
	_ = exec.Command("sc", "stop", serviceName).Run()
	time.Sleep(1500 * time.Millisecond)
	if err := os.MkdirAll(dataDir(), 0o755); err != nil {
		return err
	}
	self, err := os.Executable()
	if err != nil {
		return err
	}
	dst := filepath.Join(dataDir(), "rcontrol-unlock.exe")
	if !strings.EqualFold(self, dst) {
		if err := copyFile(self, dst); err != nil {
			return fmt.Errorf("copy helper: %w", err)
		}
	}
	if _, err := os.Stat(tokenPath()); os.IsNotExist(err) {
		buf := make([]byte, 32)
		if _, err := rand.Read(buf); err != nil {
			return err
		}
		if err := os.WriteFile(tokenPath(), []byte(hex.EncodeToString(buf)+"\n"), 0o644); err != nil {
			return err
		}
	}
	_ = exec.Command("icacls", tokenPath(), "/inheritance:r",
		"/grant:r", "*S-1-5-18:(F)", "*S-1-5-32-544:(F)", "*S-1-5-32-545:(R)").Run()

	m, err := mgr.Connect()
	if err != nil {
		return err
	}
	defer m.Disconnect()
	if s, err := m.OpenService(serviceName); err == nil {
		_, _ = s.Control(svc.Stop)
		_ = s.Delete()
		s.Close()
		time.Sleep(700 * time.Millisecond)
	}
	s, err := m.CreateService(serviceName, dst, mgr.Config{
		DisplayName: "rcontrol unlock helper",
		Description: "Types the Windows PIN/password into the lock screen for rcontrol.",
		StartType:   mgr.StartAutomatic,
	}, "-service")
	if err != nil {
		return err
	}
	defer s.Close()
	return s.Start()
}

func uninstallService() error {
	m, err := mgr.Connect()
	if err != nil {
		return err
	}
	defer m.Disconnect()
	s, err := m.OpenService(serviceName)
	if err != nil {
		return nil
	}
	defer s.Close()
	_, _ = s.Control(svc.Stop)
	time.Sleep(time.Second)
	return s.Delete()
}

func copyFile(src, dst string) error {
	in, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, in, 0o755)
}

// ---- service ----

type handler struct{}

func runService() error { return svc.Run(serviceName, handler{}) }

func (handler) Execute(_ []string, r <-chan svc.ChangeRequest, changes chan<- svc.Status) (bool, uint32) {
	changes <- svc.Status{State: svc.StartPending}
	go serveHTTP()
	changes <- svc.Status{State: svc.Running, Accepts: svc.AcceptStop | svc.AcceptShutdown}
	for c := range r {
		switch c.Cmd {
		case svc.Interrogate:
			changes <- c.CurrentStatus
		case svc.Stop, svc.Shutdown:
			changes <- svc.Status{State: svc.StopPending}
			return false, 0
		}
	}
	return false, 0
}

func serveHTTP() {
	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 200, map[string]any{"ok": true})
	})
	mux.HandleFunc("/unlock", func(w http.ResponseWriter, r *http.Request) {
		if !authorized(r) {
			writeJSON(w, 401, map[string]any{"error": "unauthorized"})
			return
		}
		var body struct {
			Secret string `json:"secret"`
		}
		if err := json.NewDecoder(io.LimitReader(r.Body, 4096)).Decode(&body); err != nil || body.Secret == "" {
			writeJSON(w, 400, map[string]any{"error": "missing secret"})
			return
		}
		if err := unlockViaSelf(body.Secret); err != nil {
			logf("unlock failed: %v", err)
			writeJSON(w, 500, map[string]any{"error": err.Error()})
			return
		}
		logf("unlock typed")
		writeJSON(w, 200, map[string]any{"ok": true})
	})
	logf("helper listening on 127.0.0.1:7947")
	if err := http.ListenAndServe("127.0.0.1:7947", mux); err != nil {
		logf("http: %v", err)
	}
}

func authorized(r *http.Request) bool {
	b, err := os.ReadFile(tokenPath())
	if err != nil {
		return false
	}
	want := strings.TrimSpace(string(b))
	got := strings.TrimSpace(strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer "))
	return want != "" && len(want) == len(got) && subtle.ConstantTimeCompare([]byte(want), []byte(got)) == 1
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

// ---- spawn the typist in the user session as SYSTEM ----

func unlockViaSelf(secret string) error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	dir := dataDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, "unlock-*.txt")
	if err != nil {
		return err
	}
	name := tmp.Name()
	if _, err := tmp.WriteString(secret); err != nil {
		tmp.Close()
		os.Remove(name)
		return err
	}
	tmp.Close()
	_ = exec.Command("icacls", name, "/inheritance:r",
		"/grant:r", "*S-1-5-18:(F)", "*S-1-5-32-544:(F)").Run()
	defer os.Remove(name)

	session := activeSession()
	pid := winlogonPID(session)
	if pid == 0 {
		return fmt.Errorf("no winlogon.exe in session %d", session)
	}
	token, err := systemToken(pid)
	if err != nil {
		return err
	}
	defer token.Close()

	si := startupInfo{Cb: uint32(unsafe.Sizeof(startupInfo{}))}
	desktop, _ := windows.UTF16PtrFromString(`WinSta0\Default`)
	si.Desktop = desktop
	var pi processInfo
	cmdline, _ := windows.UTF16PtrFromString(fmt.Sprintf(`"%s" -typist "%s" -enter`, exe, name))
	r, _, e := procCreateProcessAsU.Call(
		uintptr(token), 0, uintptr(unsafe.Pointer(cmdline)),
		0, 0, 0,
		uintptr(0x08000000), // CREATE_NO_WINDOW
		0, 0,
		uintptr(unsafe.Pointer(&si)),
		uintptr(unsafe.Pointer(&pi)),
	)
	if r == 0 {
		return fmt.Errorf("CreateProcessAsUser: %v", e)
	}
	defer windows.CloseHandle(pi.Process)
	defer windows.CloseHandle(pi.Thread)
	_, _ = windows.WaitForSingleObject(pi.Process, 20000)
	var code uint32
	if err := windows.GetExitCodeProcess(pi.Process, &code); err != nil {
		return err
	}
	if code != 0 {
		return fmt.Errorf("typist exited with %d", code)
	}
	return nil
}

func activeSession() uint32 {
	s, _, _ := procWTSActiveSession.Call()
	return uint32(s)
}

func winlogonPID(session uint32) uint32 {
	snap, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return 0
	}
	defer windows.CloseHandle(snap)
	var pe windows.ProcessEntry32
	pe.Size = uint32(unsafe.Sizeof(pe))
	if err := windows.Process32First(snap, &pe); err != nil {
		return 0
	}
	for {
		if strings.EqualFold(windows.UTF16ToString(pe.ExeFile[:]), "winlogon.exe") {
			var sid uint32
			procProcessSession.Call(uintptr(pe.ProcessID), uintptr(unsafe.Pointer(&sid)))
			if sid == session {
				return pe.ProcessID
			}
		}
		if windows.Process32Next(snap, &pe) != nil {
			break
		}
	}
	return 0
}

func systemToken(pid uint32) (windows.Token, error) {
	enableDebugPrivilege()
	h, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, pid)
	if err != nil {
		return 0, fmt.Errorf("open winlogon: %w", err)
	}
	defer windows.CloseHandle(h)
	var tok windows.Token
	if err := windows.OpenProcessToken(h, windows.TOKEN_DUPLICATE|windows.TOKEN_QUERY|windows.TOKEN_ASSIGN_PRIMARY, &tok); err != nil {
		return 0, fmt.Errorf("open token: %w", err)
	}
	defer tok.Close()
	var dup windows.Token
	if err := windows.DuplicateTokenEx(tok, windows.TOKEN_ALL_ACCESS, nil, windows.SecurityImpersonation, windows.TokenPrimary, &dup); err != nil {
		return 0, fmt.Errorf("duplicate token: %w", err)
	}
	// TokenUIAccess is what lets the typist send input to the secure desktop.
	// Without it SendInput returns Access denied once the sign-in UI is up.
	var one uint32 = 1
	const tokenUIAccess = 26
	if r, _, e := procSetTokenInfo.Call(uintptr(dup), tokenUIAccess, uintptr(unsafe.Pointer(&one)), 4); r == 0 {
		logf("SetTokenInformation(TokenUIAccess) failed: %v", e)
	} else {
		logf("typist token has UIAccess")
	}
	return dup, nil
}

func enableDebugPrivilege() {
	proc, _ := windows.GetCurrentProcess()
	var tok windows.Token
	if err := windows.OpenProcessToken(proc, windows.TOKEN_ADJUST_PRIVILEGES|windows.TOKEN_QUERY, &tok); err != nil {
		return
	}
	defer tok.Close()
	var luid windows.LUID
	name, _ := windows.UTF16PtrFromString("SeDebugPrivilege")
	if err := windows.LookupPrivilegeValue(nil, name, &luid); err != nil {
		return
	}
	tp := windows.Tokenprivileges{PrivilegeCount: 1}
	tp.Privileges[0].Luid = luid
	tp.Privileges[0].Attributes = windows.SE_PRIVILEGE_ENABLED
	_ = windows.AdjustTokenPrivileges(tok, false, &tp, 0, nil, nil)

	// SeTcbPrivilege is required to set TokenUIAccess on the child token.
	var luid2 windows.LUID
	name2, _ := windows.UTF16PtrFromString("SeTcbPrivilege")
	if err := windows.LookupPrivilegeValue(nil, name2, &luid2); err == nil {
		tp2 := windows.Tokenprivileges{PrivilegeCount: 1}
		tp2.Privileges[0].Luid = luid2
		tp2.Privileges[0].Attributes = windows.SE_PRIVILEGE_ENABLED
		_ = windows.AdjustTokenPrivileges(tok, false, &tp2, 0, nil, nil)
	}
}

// ---- typist child ----

func typistMain(file string, enter bool) int {
	b, err := os.ReadFile(file)
	os.Remove(file)
	if err != nil {
		logf("typist: %v", err)
		return 2
	}
	pid, _, _ := procGetCurrentPID.Call()
	var sid uint32
	procProcessSession.Call(pid, uintptr(unsafe.Pointer(&sid)))
	logf("typist session=%d", sid)

	name, err := attachInput()
	if err != nil {
		logf("attach: %v", err)
		return 3
	}
	logf("input desktop: %q", name)
	if strings.EqualFold(name, "Default") {
		// The lock screen is still showing the wallpaper. Any key moves it to
		// the secure sign-in desktop — which also changes the input desktop,
		// so the thread has to be re-attached before typing.
		sendVK(0x7E)
		time.Sleep(1200 * time.Millisecond)
		if name, err = attachInput(); err != nil {
			logf("attach after F15: %v", err)
			return 3
		}
		logf("input desktop after F15: %q", name)
	}
	for _, r := range string(b) {
		if r == '\b' {
			sendVK(0x08)
			continue
		}
		if err := sendRune(r); err != nil {
			logf("typist send %q: %v", r, err)
			return 4
		}
		time.Sleep(12 * time.Millisecond)
	}
	if enter {
		time.Sleep(120 * time.Millisecond)
		sendVK(0x0D)
	}
	logf("typist ok (%d chars, enter=%v)", len([]rune(string(b))), enter)
	return 0
}

func sendInput(vk, scan uint16, flags uint32) error {
	var in [40]byte
	in[0] = 1 // INPUT_KEYBOARD
	in[8] = byte(vk)
	in[9] = byte(vk >> 8)
	in[10] = byte(scan)
	in[11] = byte(scan >> 8)
	in[12] = byte(flags)
	in[13] = byte(flags >> 8)
	in[14] = byte(flags >> 16)
	in[15] = byte(flags >> 24)
	n, _, e := procSendInput.Call(1, uintptr(unsafe.Pointer(&in[0])), 40)
	if n != 1 {
		return fmt.Errorf("SendInput: %v", e)
	}
	return nil
}

func sendVK(vk uint16) {
	_ = sendInput(vk, 0, 0)
	_ = sendInput(vk, 0, 2) // KEYEVENTF_KEYUP
}

func sendRune(r rune) error {
	if r < 0 || r > 0xFFFF {
		return nil
	}
	if err := sendInput(0, uint16(r), 4); err != nil { // KEYEVENTF_UNICODE
		return err
	}
	return sendInput(0, uint16(r), 4|2)
}

// ---- Win32 structs ----

type startupInfo struct {
	Cb            uint32
	Reserved      *uint16
	Desktop       *uint16
	Title         *uint16
	X             uint32
	Y             uint32
	XSize         uint32
	YSize         uint32
	XCountChars   uint32
	YCountChars   uint32
	FillAttribute uint32
	Flags         uint32
	ShowWindow    uint16
	CbReserved2   uint16
	Reserved2     *byte
	StdInput      windows.Handle
	StdOutput     windows.Handle
	StdError      windows.Handle
}

type processInfo struct {
	Process   windows.Handle
	Thread    windows.Handle
	ProcessID uint32
	ThreadID  uint32
}

// attachInput switches this thread onto the current input desktop and returns
// its name. The rights mask matters: with only DESKTOP_SWITCHDESKTOP SendInput
// fails with Access denied.
func attachInput() (string, error) {
	h, _, e := procOpenInputDesktop.Call(0, 0, desktopRights)
	if h == 0 {
		return "", fmt.Errorf("OpenInputDesktop: %v", e)
	}
	name := objectName(h)
	cur, _, _ := procGetCurrentTID.Call()
	old, _, _ := procGetThreadDesktop.Call(cur)
	if r, _, e := procSetThreadDesktop.Call(h); r == 0 {
		return "", fmt.Errorf("SetThreadDesktop: %v", e)
	}
	procCloseDesktop.Call(old)
	return name, nil
}

func objectName(h uintptr) string {
	buf := make([]uint16, 256)
	var needed uint32
	r, _, _ := procGetUserObjectInfo.Call(h, 2, uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)*2), uintptr(unsafe.Pointer(&needed)))
	if r == 0 {
		return "?"
	}
	return windows.UTF16ToString(buf)
}
