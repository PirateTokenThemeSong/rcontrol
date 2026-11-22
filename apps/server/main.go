// Command rcontrol-server runs the LAN PC agent: screen streaming + power actions + app launching.
//
// Usage:
//
//	go run . --port 7945 [--no-auth] [--apps path/to/apps.json]
//
// First authenticated run prints a pairing PIN (stored in ~/.rcontrol).
// Pair once: POST /pair {"pin":"..."} -> {"token":"..."}.
// Apps allowlist: ~/.rcontrol/apps.json (sample created on first run).
package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"rcontrol-server/internal/auth"
	"rcontrol-server/internal/desktop"
	"rcontrol-server/internal/discovery"
	"rcontrol-server/internal/launch"
	"rcontrol-server/internal/power"
	"rcontrol-server/internal/screen"
	"rcontrol-server/internal/transport"
	"rcontrol-server/internal/unlock"
)

func main() {
	port := flag.Int("port", 7945, "TCP port for HTTP+WebSocket")
	discoveryPort := flag.Int("discovery-port", 7946, "UDP port for LAN discovery")
	noAuth := flag.Bool("no-auth", false, "disable token auth (home LAN testing only)")
	appsPath := flag.String("apps", "", "path to apps allowlist JSON (default ~/.rcontrol/apps.json)")
	flag.Parse()

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

	addr := fmt.Sprintf("0.0.0.0:%d", *port)
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatalf("listen %s: %v", addr, err)
	}
	fmt.Printf("rcontrol server on http://%s (ws /ws, mjpeg /stream)\n", lis.Addr())

	srv := &http.Server{Handler: mux}
	go func() {
		if err := srv.Serve(lis); err != nil && err != http.ErrServerClosed {
			log.Fatalf("serve: %v", err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop
	fmt.Println("\nshutting down")
	_ = srv.Close()
}
