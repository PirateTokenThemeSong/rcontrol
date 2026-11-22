// Package transport wires HTTP routes: /health, /pair, /apps, /launch, /ws, /stream.
package transport

import (
	"encoding/json"
	"fmt"
	"image"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"rcontrol-server/internal/desktop"
	"rcontrol-server/internal/icons"
	"rcontrol-server/internal/launch"
	"rcontrol-server/internal/power"
	"rcontrol-server/internal/screen"
)

// Deps are the server dependencies (interfaces keep this package testable;
// production impls live in internal/auth, internal/screen, internal/power,
// internal/launch and are wired in main.go).
type Deps struct {
	HostName string
	NoAuth   bool
	Auth     AuthChecker
	Capturer Capturer
	Power    PowerRunner
	Launcher Launcher
	Windows  WindowManager
	Unlocker Unlocker
	Port     int
}

// AuthChecker validates tokens and PINs (implemented by internal/auth.Store).
type AuthChecker interface {
	CheckToken(string) bool
	CheckPIN(string) bool
	Token() string
	ServerID() string
}

// Capturer captures screen frames (implemented by internal/screen.Capturer).
type Capturer interface {
	GetQuality() screen.Quality
	SetQuality(screen.Quality)
	Bounds() image.Rectangle
	Frame() ([]byte, int, int, error)
}

// PowerRunner executes power actions (implemented by internal/power.Service).
type PowerRunner interface {
	Run(power.Action) error
}

// Launcher lists, searches, and starts apps (implemented by internal/launch.Service).
type Launcher interface {
	List() []launch.App
	Search(query string) []launch.App
	Run(id string) error
	PathFor(id string) (string, bool)
}

// Unlocker types a secret into the Windows lock screen (implemented by internal/unlock.Helper).
type Unlocker interface {
	Unlock(secret string) error
}

// WindowManager lists and focuses open windows (implemented by internal/desktop.Service).
type WindowManager interface {
	List() []desktop.Window
	Focus(string) error
	Locked() bool
	Icon(id string) ([]byte, error)
}

// authorized reports whether the request carries a valid token (or auth is off).
func (d Deps) authorized(r *http.Request) bool {
	return d.NoAuth || d.Auth.CheckToken(r.URL.Query().Get("token"))
}

var upgrader = websocket.Upgrader{
	ReadBufferSize:  4096,
	WriteBufferSize: 256 * 1024,
	CheckOrigin:     func(r *http.Request) bool { return true }, // LAN app, no browser CORS risk
}

// RegisterRoutes mounts all endpoints.
func RegisterRoutes(mux *http.ServeMux, d Deps) {
	lim := newPairLimiter()
	unlockLim := newPairLimiter()

	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 200, map[string]any{"ok": true, "version": 1, "name": d.HostName})
	})

	mux.HandleFunc("/pair", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeJSON(w, 405, map[string]any{"error": "POST only"})
			return
		}
		ip, _, _ := net.SplitHostPort(r.RemoteAddr)
		if !lim.allow(ip) {
			writeJSON(w, 429, map[string]any{"error": "too many attempts"})
			return
		}
		var body struct {
			PIN string `json:"pin"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeJSON(w, 400, map[string]any{"error": "bad json"})
			return
		}
		if !d.Auth.CheckPIN(body.PIN) {
			writeJSON(w, 401, map[string]any{"error": "bad pin"})
			return
		}
		writeJSON(w, 200, map[string]any{"token": d.Auth.Token()})
	})

	mux.HandleFunc("/apps", func(w http.ResponseWriter, r *http.Request) {
		if !d.authorized(r) {
			writeJSON(w, 401, map[string]any{"error": "unauthorized"})
			return
		}
		apps := []launch.App{}
		if d.Launcher != nil {
			if a := d.Launcher.List(); a != nil {
				apps = a
			}
		}
		writeJSON(w, 200, map[string]any{"apps": apps})
	})

	mux.HandleFunc("/launch", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeJSON(w, 405, map[string]any{"error": "POST only"})
			return
		}
		if !d.authorized(r) {
			writeJSON(w, 401, map[string]any{"error": "unauthorized"})
			return
		}
		if d.Launcher == nil {
			writeJSON(w, 501, map[string]any{"error": "launch not configured"})
			return
		}
		var body struct {
			App string `json:"app"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || strings.TrimSpace(body.App) == "" {
			writeJSON(w, 400, map[string]any{"error": "missing app id"})
			return
		}
		if err := d.Launcher.Run(body.App); err != nil {
			writeJSON(w, 400, map[string]any{"error": err.Error()})
			return
		}
		writeJSON(w, 200, map[string]any{"type": "launch-ack", "app": body.App})
	})

	mux.HandleFunc("/search", func(w http.ResponseWriter, r *http.Request) {
		if !d.authorized(r) {
			writeJSON(w, 401, map[string]any{"error": "unauthorized"})
			return
		}
		results := []launch.App{}
		if d.Launcher != nil {
			if r := d.Launcher.Search(r.URL.Query().Get("q")); r != nil {
				results = r
			}
		}
		writeJSON(w, 200, map[string]any{"apps": results})
	})

	mux.HandleFunc("/icon", func(w http.ResponseWriter, r *http.Request) {
		if !d.authorized(r) {
			writeJSON(w, 401, map[string]any{"error": "unauthorized"})
			return
		}
		var png []byte
		var err error
		switch r.URL.Query().Get("kind") {
		case "app":
			if d.Launcher != nil {
				if p, ok := d.Launcher.PathFor(r.URL.Query().Get("id")); ok {
					png, err = icons.ForFile(p)
				}
			}
		case "window":
			if d.Windows != nil {
				png, err = d.Windows.Icon(r.URL.Query().Get("id"))
			}
		}
		if err != nil || len(png) == 0 {
			writeJSON(w, 404, map[string]any{"error": "no icon"})
			return
		}
		w.Header().Set("Content-Type", "image/png")
		w.Header().Set("Cache-Control", "max-age=3600")
		_, _ = w.Write(png)
	})

	mux.HandleFunc("/windows", func(w http.ResponseWriter, r *http.Request) {
		if !d.authorized(r) {
			writeJSON(w, 401, map[string]any{"error": "unauthorized"})
			return
		}
		list := []desktop.Window{}
		locked := false
		if d.Windows != nil {
			if l := d.Windows.List(); l != nil {
				list = l
			}
			locked = d.Windows.Locked()
		}
		writeJSON(w, 200, map[string]any{"windows": list, "locked": locked})
	})

	mux.HandleFunc("/unlock", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeJSON(w, 405, map[string]any{"error": "POST only"})
			return
		}
		if !d.authorized(r) {
			writeJSON(w, 401, map[string]any{"error": "unauthorized"})
			return
		}
		if d.Unlocker == nil {
			writeJSON(w, 501, map[string]any{"error": "unlock helper not available"})
			return
		}
		var body struct {
			Secret string `json:"secret"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || strings.TrimSpace(body.Secret) == "" {
			writeJSON(w, 400, map[string]any{"error": "missing secret"})
			return
		}
		if !unlockLim.allow(ipOf(r)) {
			writeJSON(w, 429, map[string]any{"error": "too many unlock attempts"})
			return
		}
		if err := d.Unlocker.Unlock(body.Secret); err != nil {
			writeJSON(w, 400, map[string]any{"error": err.Error()})
			return
		}
		writeJSON(w, 200, map[string]any{"type": "unlock-ack"})
	})

	mux.HandleFunc("/lockstate", func(w http.ResponseWriter, r *http.Request) {
		if !d.authorized(r) {
			writeJSON(w, 401, map[string]any{"error": "unauthorized"})
			return
		}
		locked := d.Windows != nil && d.Windows.Locked()
		writeJSON(w, 200, map[string]any{"locked": locked})
	})

	mux.HandleFunc("/focus", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeJSON(w, 405, map[string]any{"error": "POST only"})
			return
		}
		if !d.authorized(r) {
			writeJSON(w, 401, map[string]any{"error": "unauthorized"})
			return
		}
		if d.Windows == nil {
			writeJSON(w, 501, map[string]any{"error": "window control not available"})
			return
		}
		var body struct {
			ID string `json:"id"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || strings.TrimSpace(body.ID) == "" {
			writeJSON(w, 400, map[string]any{"error": "missing window id"})
			return
		}
		if err := d.Windows.Focus(body.ID); err != nil {
			writeJSON(w, 400, map[string]any{"error": err.Error()})
			return
		}
		writeJSON(w, 200, map[string]any{"type": "focus-ack", "id": body.ID})
	})

	mux.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		if !d.authorized(r) {
			// Gorilla has no 401 upgrade path; close with policy code after accept.
			c, err := upgrader.Upgrade(w, r, nil)
			if err != nil {
				return
			}
			_ = c.WriteControl(websocket.CloseMessage,
				websocket.FormatCloseMessage(4401, "unauthorized"),
				time.Now().Add(2*time.Second))
			_ = c.Close()
			return
		}
		serveWS(w, r, d)
	})

	mux.HandleFunc("/stream", func(w http.ResponseWriter, r *http.Request) {
		if !d.authorized(r) {
			writeJSON(w, 401, map[string]any{"error": "unauthorized"})
			return
		}
		serveMJPEG(w, r, d)
	})
}

// ---- WebSocket ----

type clientMsg struct {
	Type   string `json:"type"`
	Action string `json:"action,omitempty"`
	App    string `json:"app,omitempty"`
	ID     string `json:"id,omitempty"`
	FPS    int    `json:"fps,omitempty"`
	JPEG   int    `json:"jpeg,omitempty"`
	MaxW   int    `json:"maxWidth,omitempty"`
}

func serveWS(w http.ResponseWriter, r *http.Request, d Deps) {
	c, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	defer c.Close()

	// Gorilla allows one concurrent writer. Acks come from the reader
	// goroutine, frames/pings from the writer loop, so serialize them.
	var writeMu sync.Mutex
	writeMsg := func(mt int, data []byte) error {
		writeMu.Lock()
		defer writeMu.Unlock()
		_ = c.SetWriteDeadline(time.Now().Add(5 * time.Second))
		return c.WriteMessage(mt, data)
	}
	sendJSON := func(v any) error {
		b, err := json.Marshal(v)
		if err != nil {
			return err
		}
		return writeMsg(websocket.TextMessage, b)
	}

	_ = c.SetReadDeadline(time.Now().Add(60 * time.Second))
	c.SetPongHandler(func(string) error {
		return c.SetReadDeadline(time.Now().Add(60 * time.Second))
	})

	// Per-connection quality (starts from server default).
	q := d.Capturer.GetQuality()
	stop := make(chan struct{})
	var stopOnce sync.Once
	defer stopOnce.Do(func() { close(stop) })

	// Reader: handle ping / quality / power / launch.
	go func() {
		defer stopOnce.Do(func() { close(stop) })
		for {
			mt, data, err := c.ReadMessage()
			if err != nil {
				return
			}
			if mt != websocket.TextMessage {
				continue
			}
			var m clientMsg
			if err := json.Unmarshal(data, &m); err != nil {
				_ = sendJSON(map[string]any{"type": "error", "error": "bad json"})
				continue
			}
			switch strings.ToLower(m.Type) {
			case "ping":
				_ = sendJSON(map[string]any{"type": "pong"})
			case "quality":
				if m.FPS != 0 {
					q.FPS = clamp(m.FPS, 1, 15)
				}
				if m.JPEG != 0 {
					q.JPEG = clamp(m.JPEG, 30, 90)
				}
				if m.MaxW != 0 {
					q.MaxWidth = clamp(m.MaxW, 320, 1920)
				}
				_ = sendJSON(map[string]any{"type": "quality-ack", "fps": q.FPS, "jpeg": q.JPEG})
			case "power":
				if !power.Valid(m.Action) {
					_ = sendJSON(map[string]any{"type": "error", "error": "unknown power action"})
					continue
				}
				if err := d.Power.Run(power.Action(m.Action)); err != nil {
					_ = sendJSON(map[string]any{"type": "error", "error": err.Error()})
					continue
				}
				_ = sendJSON(map[string]any{"type": "power-ack", "action": m.Action})
			case "lockstate":
				_ = sendJSON(map[string]any{
					"type":   "locked",
					"locked": d.Windows != nil && d.Windows.Locked(),
				})
			case "windows":
				list := []desktop.Window{}
				if d.Windows != nil {
					if l := d.Windows.List(); l != nil {
						list = l
					}
				}
				_ = sendJSON(map[string]any{"type": "windows", "windows": list})
			case "focus":
				id := strings.TrimSpace(m.ID)
				if id == "" {
					_ = sendJSON(map[string]any{"type": "error", "error": "missing window id"})
					continue
				}
				if d.Windows == nil {
					_ = sendJSON(map[string]any{"type": "error", "error": "window control not available"})
					continue
				}
				if err := d.Windows.Focus(id); err != nil {
					_ = sendJSON(map[string]any{"type": "error", "error": err.Error()})
					continue
				}
				_ = sendJSON(map[string]any{"type": "focus-ack", "id": id})
			case "launch":
				if strings.TrimSpace(m.App) == "" {
					_ = sendJSON(map[string]any{"type": "error", "error": "missing app id"})
					continue
				}
				if d.Launcher == nil {
					_ = sendJSON(map[string]any{"type": "error", "error": "launch not configured"})
					continue
				}
				if err := d.Launcher.Run(m.App); err != nil {
					_ = sendJSON(map[string]any{"type": "error", "error": err.Error()})
					continue
				}
				_ = sendJSON(map[string]any{"type": "launch-ack", "app": m.App})
			default:
				_ = sendJSON(map[string]any{"type": "error", "error": "unknown type"})
			}
		}
	}()

	// Hello first so the client knows the desktop size.
	b := d.Capturer.Bounds()
	_ = sendJSON(map[string]any{
		"type": "hello", "width": b.Dx(), "height": b.Dy(), "fps": q.FPS,
	})

	// Writer: JPEG frame loop with per-connection quality snapshot.
	ticker := time.NewTicker(frameInterval(q.FPS))
	defer ticker.Stop()
	pingTicker := time.NewTicker(30 * time.Second)
	defer pingTicker.Stop()

	lockTicker := time.NewTicker(time.Second)
	defer lockTicker.Stop()
	lockedKnown, lockedNow := false, false
	for {
		select {
		case <-stop:
			return
		case <-lockTicker.C:
			l := d.Windows != nil && d.Windows.Locked()
			if !lockedKnown || l != lockedNow {
				lockedKnown, lockedNow = true, l
				_ = sendJSON(map[string]any{"type": "locked", "locked": l})
			}
		case <-pingTicker.C:
			if err := writeMsg(websocket.PingMessage, nil); err != nil {
				return
			}
		case <-ticker.C:
			ticker.Reset(frameInterval(q.FPS))
			// Temporarily apply per-connection quality for this frame.
			prev := d.Capturer.GetQuality()
			d.Capturer.SetQuality(screen.Quality{FPS: q.FPS, JPEG: q.JPEG, MaxWidth: nz(q.MaxWidth, 960)})
			frame, _, _, err := d.Capturer.Frame()
			d.Capturer.SetQuality(prev)
			if err != nil {
				continue
			}
			if err := writeMsg(websocket.BinaryMessage, frame); err != nil {
				return
			}
		}
	}
}

func frameInterval(fps int) time.Duration {
	if fps < 1 {
		fps = 1
	}
	if fps > 15 {
		fps = 15
	}
	return time.Second / time.Duration(fps)
}

// ---- MJPEG fallback ----

func serveMJPEG(w http.ResponseWriter, r *http.Request, d Deps) {
	w.Header().Set("Content-Type", "multipart/x-mixed-replace; boundary=frame")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "close")
	fl, ok := w.(http.Flusher)
	if !ok {
		writeJSON(w, 500, map[string]any{"error": "streaming unsupported"})
		return
	}
	ticker := time.NewTicker(frameInterval(d.Capturer.GetQuality().FPS))
	defer ticker.Stop()
	for {
		select {
		case <-r.Context().Done():
			return
		case <-ticker.C:
			frame, _, _, err := d.Capturer.Frame()
			if err != nil {
				continue
			}
			_, _ = fmt.Fprintf(w, "--frame\r\nContent-Type: image/jpeg\r\nContent-Length: %d\r\n\r\n", len(frame))
			_, _ = w.Write(frame)
			_, _ = fmt.Fprint(w, "\r\n")
			fl.Flush()
		}
	}
}

// ---- helpers ----

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func ipOf(r *http.Request) string {
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return ip
}

func clamp(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func nz(v, d int) int {
	if v == 0 {
		return d
	}
	return v
}

// pairLimiter: max 10 attempts/min per IP.
type pairLimiter struct {
	mu       sync.Mutex
	attempts map[string][]time.Time
}

func newPairLimiter() *pairLimiter { return &pairLimiter{attempts: map[string][]time.Time{}} }

func (l *pairLimiter) allow(ip string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := time.Now()
	kept := l.attempts[ip][:0]
	for _, t := range l.attempts[ip] {
		if now.Sub(t) < time.Minute {
			kept = append(kept, t)
		}
	}
	if len(kept) >= 10 {
		l.attempts[ip] = kept
		return false
	}
	l.attempts[ip] = append(kept, now)
	return true
}
