// Package desktop lists and focuses top-level windows on the PC.
//
// v1 is Windows-only: EnumWindows for the list, SetForegroundWindow (plus the
// usual AttachThreadInput / Alt fallbacks) to bring one to the front.
package desktop

// Window is one focusable top-level window, newest z-order first.
type Window struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	Active    bool   `json:"active"`
	Minimized bool   `json:"minimized"`
}

// Service lists and focuses windows for the current OS.
type Service struct{}

// New returns a Service.
func New() *Service { return &Service{} }
