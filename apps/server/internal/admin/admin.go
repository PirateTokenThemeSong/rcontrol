// Package admin serves the local settings UI (loopback only) that the tray
// icon opens in the browser.
package admin

import (
	"embed"
	"encoding/json"
	"net"
	"net/http"
	"time"

	"rcontrol-server/internal/config"
)

//go:embed index.html
var assets embed.FS

// Auth is the credential store.
type Auth interface {
	PIN() string
	RotatePIN() (string, error)
	RotateToken() (string, error)
}

// Startup controls the login entry.
type Startup interface {
	Enabled() bool
	Set(bool) error
}

// Deps is what the settings UI can read and change.
type Deps struct {
	HostName   string
	Version    string
	Port       int
	Started    time.Time
	DataDir    string
	Auth       Auth
	Helper     func() bool
	Startup    Startup
	Quit       func()
	Restart    func() error
	OpenFolder func()
}

// Serve starts the settings server on addr (loopback only).
func Serve(addr string, d Deps) (*http.Server, error) {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		b, err := assets.ReadFile("index.html")
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write(b)
	})
	mux.HandleFunc("/api/state", func(w http.ResponseWriter, r *http.Request) {
		jsonOut(w, 200, d.state())
	})
	mux.HandleFunc("/api/pin", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			jsonOut(w, 405, map[string]any{"error": "POST only"})
			return
		}
		pin, err := d.Auth.RotatePIN()
		if err != nil {
			jsonOut(w, 500, map[string]any{"error": err.Error()})
			return
		}
		jsonOut(w, 200, map[string]any{"pin": pin})
	})
	mux.HandleFunc("/api/unpair", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			jsonOut(w, 405, map[string]any{"error": "POST only"})
			return
		}
		if _, err := d.Auth.RotateToken(); err != nil {
			jsonOut(w, 500, map[string]any{"error": err.Error()})
			return
		}
		jsonOut(w, 200, map[string]any{"ok": true})
	})
	mux.HandleFunc("/api/startup", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			jsonOut(w, 405, map[string]any{"error": "POST only"})
			return
		}
		var body struct {
			Enabled bool `json:"enabled"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			jsonOut(w, 400, map[string]any{"error": "bad request"})
			return
		}
		if err := d.Startup.Set(body.Enabled); err != nil {
			jsonOut(w, 500, map[string]any{"error": err.Error()})
			return
		}
		jsonOut(w, 200, map[string]any{"ok": true, "enabled": d.Startup.Enabled()})
	})
	mux.HandleFunc("/api/port", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			jsonOut(w, 405, map[string]any{"error": "POST only"})
			return
		}
		var body struct {
			Port int `json:"port"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Port < 1 || body.Port > 65535 {
			jsonOut(w, 400, map[string]any{"error": "invalid port"})
			return
		}
		if err := config.Save(config.Config{Port: body.Port}); err != nil {
			jsonOut(w, 500, map[string]any{"error": err.Error()})
			return
		}
		jsonOut(w, 200, map[string]any{"ok": true})
	})
	mux.HandleFunc("/api/restart", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || d.Restart == nil {
			jsonOut(w, 405, map[string]any{"error": "not available"})
			return
		}
		if err := d.Restart(); err != nil {
			jsonOut(w, 500, map[string]any{"error": err.Error()})
			return
		}
		jsonOut(w, 200, map[string]any{"ok": true})
	})
	mux.HandleFunc("/api/open", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || d.OpenFolder == nil {
			jsonOut(w, 405, map[string]any{"error": "not available"})
			return
		}
		d.OpenFolder()
		jsonOut(w, 200, map[string]any{"ok": true})
	})
	mux.HandleFunc("/api/quit", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || d.Quit == nil {
			jsonOut(w, 405, map[string]any{"error": "not available"})
			return
		}
		jsonOut(w, 200, map[string]any{"ok": true})
		go func() {
			time.Sleep(150 * time.Millisecond)
			d.Quit()
		}()
	})

	srv := &http.Server{Addr: addr, Handler: mux}
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, err
	}
	go func() { _ = srv.Serve(ln) }()
	return srv, nil
}

func (d Deps) state() map[string]any {
	s := map[string]any{
		"host":    d.HostName,
		"version": d.Version,
		"port":    d.Port,
		"uptime":  time.Since(d.Started).Round(time.Second).String(),
		"pin":       d.Auth.PIN(),
		"startup":   d.Startup != nil && d.Startup.Enabled(),
		"dataDir":   d.DataDir,
		"helper":    d.Helper != nil && d.Helper(),
		"addresses": addresses(),
	}
	return s
}

// Addr is one IPv4 address the server can be reached on.
type Addr struct {
	Name string `json:"name"`
	IP   string `json:"ip"`
	Kind string `json:"kind"`
}

// addresses lists every non-loopback IPv4 interface, LAN first.
func addresses() []Addr {
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil
	}
	var lan, tailscale, other []Addr
	for _, i := range ifaces {
		if i.Flags&net.FlagUp == 0 || i.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, err := i.Addrs()
		if err != nil {
			continue
		}
		for _, a := range addrs {
			var ip net.IP
			switch v := a.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			}
			if ip == nil || ip.To4() == nil {
				continue
			}
			entry := Addr{Name: i.Name, IP: ip.String(), Kind: "other"}
			switch {
			case ip[12] == 100 && ip[13] >= 64 && ip[13] <= 127:
				entry.Kind = "tailscale"
				tailscale = append(tailscale, entry)
			case ip.IsPrivate():
				entry.Kind = "lan"
				lan = append(lan, entry)
			default:
				other = append(other, entry)
			}
		}
	}
	return append(append(lan, tailscale...), other...)
}

func jsonOut(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}
