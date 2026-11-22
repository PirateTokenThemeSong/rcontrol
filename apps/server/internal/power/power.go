// Package power executes shutdown / reboot / sleep / lock on Windows and Linux.
package power

import (
	"fmt"
	"os/exec"
	"runtime"
)

// Action is one of shutdown, reboot, sleep, lock.
type Action string

const (
	Shutdown   Action = "shutdown"
	Reboot     Action = "reboot"
	Sleep      Action = "sleep"
	Lock       Action = "lock"
	DisplayOff Action = "display"
)

// Service runs power actions for the current OS.
type Service struct{ os string }

// New returns a Service for runtime.GOOS.
func New() *Service { return &Service{os: runtime.GOOS} }

// Run executes the action. Shutdown/reboot use a 5s delay so the
// phone gets the ack before the PC goes away.
func (s *Service) Run(a Action) error {
	switch s.os {
	case "windows":
		return s.runWindows(a)
	default: // linux, darwin (best effort)
		return s.runNix(a)
	}
}

func (s *Service) runWindows(a Action) error {
	var cmd *exec.Cmd
	switch a {
	case Shutdown:
		cmd = exec.Command("shutdown", "/s", "/t", "5", "/c", "rcontrol remote shutdown")
	case Reboot:
		cmd = exec.Command("shutdown", "/r", "/t", "5", "/c", "rcontrol remote reboot")
	case Sleep:
		// rundll32 powrprof.dll,SetSuspendState 0,1,0
		cmd = exec.Command("rundll32.exe", "powrprof.dll,SetSuspendState", "0,1,0")
	case Lock:
		cmd = exec.Command("rundll32.exe", "user32.dll,LockWorkStation")
	case DisplayOff:
		// SC_MONITORPOWER 2 = turn display off (any input wakes it).
		cmd = exec.Command("powershell", "-NoProfile", "-Command",
			"(Add-Type '[DllImport(\"user32.dll\")]public static extern int SendMessage(int hWnd, int hMsg, int wParam, int lParam);' -Name RCMon -Pas)::SendMessage(-1, 0x0112, 0xF170, 2)")
	default:
		return fmt.Errorf("unknown action %q", a)
	}
	return cmd.Start()
}

func (s *Service) runNix(a Action) error {
	var cmd *exec.Cmd
	switch a {
	case Shutdown:
		cmd = exec.Command("shutdown", "+1", "rcontrol remote shutdown")
	case Reboot:
		cmd = exec.Command("shutdown", "-r", "+1", "rcontrol remote reboot")
	case Sleep:
		cmd = exec.Command("systemctl", "suspend")
	case Lock:
		cmd = exec.Command("loginctl", "lock-session")
	case DisplayOff:
		if s.os == "darwin" {
			cmd = exec.Command("pmset", "displaysleepnow")
		} else {
			cmd = exec.Command("xset", "dpms", "force", "off")
		}
	default:
		return fmt.Errorf("unknown action %q", a)
	}
	return cmd.Start()
}

// Valid reports whether the action name is known.
func Valid(a string) bool {
	switch Action(a) {
	case Shutdown, Reboot, Sleep, Lock, DisplayOff:
		return true
	}
	return false
}
