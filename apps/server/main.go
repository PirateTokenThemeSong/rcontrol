// Command rcontrol-server runs the LAN PC agent: screen streaming + power actions + app launching.
//
// Usage:
//
//	rcontrol-server [--port 7945] [--no-auth] [--apps path/to/apps.json]
//
// The server shows a tray icon (Windows): left click opens the local settings
// page, right click has Settings / Open data folder / Exit.
//
// First authenticated run prints a pairing PIN (stored in ~/.rcontrol).
// Apps allowlist: ~/.rcontrol/apps.json (sample created on first run).
package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
	"syscall"
	"time"

	"rcontrol-server/internal/admin"
	"rcontrol-server/internal/auth"
	"rcontrol-server/internal/config"
	"rcontrol-server/internal/desktop"
	"rcontrol-server/internal/discovery"
	"rcontrol-server/internal/launch"
	"rcontrol-server/internal/power"
	"rcontrol-server/internal/screen"
	"rcontrol-server/internal/startup"
	"rcontrol-server/internal/tray"
	"rcontrol-server/internal/transport"
	"rcontrol-server/internal/unlock"
)

const version = "0.1.0"

// adminAddr is the loopback settings UI opened by the tray icon.
const adminAddr = "127.0.0.1:7948"

// startupSvc adapts the startup package to the admin interface.
type startupSvc struct{}

func (startupSvc) Enabled() bool     { return startup.Enabled() }
func (startupSvc) Set(b bool) error  { return startup.Set(b) }

func main() {
	cfg := config.Load()

	port := flag.Int("port", 0, "TCP port for HTTP+WebSocket (default: config or 7945)")
	discoveryPort := flag.Int("discovery-port", 7946, "UDP port for LAN discovery")
	noAuth := flag.Bool("no-auth", false, "disable token auth (home LAN testing only)")
	appsPath := flag.String("apps", "", "path to apps allowlist JSON (default ~/.rcontrol/apps.json)")
	flag.Parse()
	if *port == 0 {
		*port = cfg.Port
	}

	store, err := auth.LoadOrCreate()
	if err != nil {
		log.Fatalf("auth store: %v", err)
	}
	if *noAuth {
		fmt.Println("WARNING: running with --no-auth, anyone on LAN can control this PC")
	} else {
		fmt.Printf("Pairing PIN: %s  (config dir: %s)\n", store.PIN(), store.Dir())
	}

	launcher, err := launch.LoadOrSample(*appsPath)
	if err != nil {
		log.Fatalf("apps allowlist: %v", err)
	}
	fmt.Printf("Configured apps: %d (edit %s to add your own)\n", len(launcher.List()), launch.DefaultPath())

	hostname, _ := os.Hostname()
	if hostname == "" {
		hostname = "rcontrol-pc"
	}

	capturer := screen.NewCapturer()
	powerSvc := power.New()
	windowSvc := desktop.New()

	mux := http.NewServeMux()
	transport.RegisterRoutes(mux, transport.Deps{
		HostName: hostname,
		NoAuth:   *noAuth,
		Auth:     store,
		Capturer: capturer,
		Power:    powerSvc,
		Launcher: launcher,
		Windows:  windowSvc,
		Unlocker: unlock.Helper{},
		Port:     *port,
	})

	addr := fmt.Sprintf("0.0.0.0:%d", *port)
	var lis net.Listener
	for attempt := 0; attempt < 20; attempt++ {
		lis, err = net.Listen("tcp", addr)
		if err == nil {
			break
		}
		time.Sleep(300 * time.Millisecond) // a restarting instance may still hold the port
	}
	if err != nil {
		log.Fatalf("listen %s: %v", addr, err)
	}
	srv := &http.Server{Handler: mux}
	go func() {
		if err := srv.Serve(lis); err != nil && err != http.ErrServerClosed {
			log.Fatalf("serve: %v", err)
		}
	}()
	fmt.Printf("rcontrol server on http://%s (ws /ws, mjpeg /stream)\n", lis.Addr())

	// UDP discovery responder (best-effort, never fatal).
	go func() {
		info := discovery.Info{
			Name:    hostname,
			ID:      store.ServerID(),
			Port:    *port,
			Version: 1,
			Auth:    !*noAuth,
		}
		if err := discovery.Serve(*discoveryPort, info); err != nil {
			log.Printf("discovery: %v", err)
		}
	}()

	var adminSrv *http.Server
	quit := func() {
		if adminSrv != nil {
			_ = adminSrv.Close()
		}
		_ = srv.Close()
		tray.Stop()
		os.Exit(0)
	}

	restart := func() error {
		exe, err := os.Executable()
		if err != nil {
			return err
		}
		cmd := exec.Command(exe)
		if err := cmd.Start(); err != nil {
			return err
		}
		go func() {
			time.Sleep(200 * time.Millisecond)
			quit()
		}()
		return nil
	}

	started := time.Now()
	for attempt := 0; attempt < 20; attempt++ {
		adminSrv, err = admin.Serve(adminAddr, admin.Deps{
			HostName:   hostname,
			Version:    version,
			Port:       *port,
			Started:    started,
			DataDir:    store.Dir(),
			Auth:       store,
			Helper:     unlock.Available,
			Startup:    startupSvc{},
			Quit:       quit,
			Restart:    restart,
			OpenFolder: func() { openPath(store.Dir()) },
		})
		if err == nil {
			break
		}
		time.Sleep(300 * time.Millisecond)
	}
	if err != nil {
		log.Printf("settings UI: %v", err)
	} else {
		fmt.Printf("settings UI on http://%s\n", adminAddr)
	}

	openSettings := func() { openURL("http://" + adminAddr + "/") }
	if err := tray.Start(openSettings, func() { openPath(store.Dir()) }, quit); err != nil {
		log.Printf("tray: %v (left-click settings unavailable)", err)
	}

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop
	fmt.Println("\nshutting down")
	tray.Stop()
	_ = adminSrv.Close()
	_ = srv.Close()
}

func openURL(u string) {
	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", u)
	} else if runtime.GOOS == "darwin" {
		cmd = exec.Command("open", u)
	} else {
		cmd = exec.Command("xdg-open", u)
	}
	if err := cmd.Start(); err != nil {
		log.Printf("open %s: %v", u, err)
	}
}

func openPath(p string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("explorer", p)
	case "darwin":
		cmd = exec.Command("open", p)
	default:
		cmd = exec.Command("xdg-open", p)
	}
	_ = cmd.Start()
}
