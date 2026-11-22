// Package discovery answers LAN auto-discovery broadcasts over UDP.
package discovery

import (
	"encoding/json"
	"net"
)

// Info is the JSON payload sent back to discovery clients.
type Info struct {
	Name    string `json:"name"`
	ID      string `json:"id"`
	Addr    string `json:"addr"`
	Port    int    `json:"port"`
	Version int    `json:"version"`
	Auth    bool   `json:"auth"`
}

// Serve listens on udpPort and replies to "RC_DISCOVER" datagrams.
func Serve(udpPort int, info Info) error {
	addr := net.UDPAddr{Port: udpPort}
	conn, err := net.ListenUDP("udp", &addr)
	if err != nil {
		return err
	}
	defer conn.Close()

	buf := make([]byte, 256)
	for {
		n, remote, err := conn.ReadFromUDP(buf)
		if err != nil {
			continue
		}
		msg := string(buf[:n])
		if len(msg) < len("RC_DISCOVER") || msg[:len("RC_DISCOVER")] != "RC_DISCOVER" {
			continue
		}
		out := info
		out.Addr = localIPFor(remote)
		payload, _ := json.Marshal(out)
		_, _ = conn.WriteToUDP(payload, remote)
	}
}

func localIPFor(remote *net.UDPAddr) string {
	c, err := net.Dial("udp", remote.String())
	if err != nil {
		return ""
	}
	defer c.Close()
	if a, ok := c.LocalAddr().(*net.UDPAddr); ok {
		return a.IP.String()
	}
	return ""
}
