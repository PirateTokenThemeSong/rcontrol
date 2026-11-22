# Android guide

The app is a native Android client (minSdk 26). It talks to the server over your
LAN or Tailscale.

## Install

Grab `rcontrol-*.apk` from the [latest release](https://github.com/PirateTokenThemeSong/rcontrol/releases)
and open it on the phone. Android will ask you to allow installing from the
browser. Releases are debug-signed for now, so a build from a different CI run
may need the old app uninstalled first.

No special permissions: plain internet access is enough.

## Home

- **Your PC** — the saved target, its address and whether it answers `/health`
  (`online` / `offline`).
- **Connect** — opens the control screen.
- **Wake** — opens the wake screen (needs a wake method, see below).
- **On your network** — **Scan** broadcasts a discovery probe (UDP 7946) and
  lists PCs that answer. Tap one to connect; its address is remembered.
- **Gear** — settings.

If nothing is set up yet, the card offers **Set up PC**, which jumps to settings.

## Control

- **Stream** — the live desktop.
- **Power** — one row that opens a menu: Sleep, Lock, Turn displays off, then
  Shut down / Reboot (both ask for confirmation).
- **Windows / Apps** — a segmented switcher:
  - *Windows* — open windows, tap to focus; the front one is marked `active`.
  - *Apps* — search the Start Menu, tap to launch, pin the ones you use.
  - Six rows show at a time; **Show all** expands.
- **Locked card** — appears instead of the switcher when Windows is locked; type
  the PIN or password to unlock (requires the
  [helper](server.md#unlock)).
- **Disconnect** — top right.

## Settings

### Connection

- **PC name** — a label for the Home card.
- **Address** and **Port** — where the server listens (usually the LAN address
  shown in the server settings, port `7945`). Over Tailscale, use the Tailscale
  address and the same port.
- **Pairing** — type the six-digit PIN from the PC and tap **Pair**. The app
  stores a token; the token is never displayed or edited. **Unpair** forgets it.

### Stream

- **Frames per second** — 1–15.
- **JPEG quality** — 30–90.
- **Max width** — 320–1920 px; the frame is scaled down before encoding.

Lower values are smoother on weak Wi-Fi. They are applied when the session
starts.

### Wake on LAN

- **MAC address**, **Broadcast** (e.g. `192.168.1.255`) and **Port** (9). Leave
  the MAC blank to skip the magic packet.
- **Custom wake request** — a fully configurable HTTP request (method, URL,
  headers, body) sent before waiting. Useful for Home Assistant, a router API or
  a Raspberry Pi that wakes the PC.

Example for Home Assistant (create a long-lived token in your HA profile and
point the request at a button or script):

```
Method:  POST
URL:     http://homeassistant.local:8123/api/services/button/press
Headers: Authorization: Bearer <token>
         Content-Type: application/json
Body:    {"entity_id":"button.my_pc"}
```

After sending the signal, the app polls `GET /health` every 2.5 s for up to five
minutes and opens the control screen when the server answers.

### About

Version and a reminder that the app is LAN-only.

## Locked screen

When the PC is locked the stream shows the lock screen until the sign-in desktop
takes over, at which point the video freezes (that desktop cannot be captured).
Use the unlock card: the helper types the PIN/password and the stream returns
when the desktop does.

## Notes

- The phone and the PC must be able to reach each other. On a normal LAN that is
  automatic; with Tailscale, both devices need to be in the same tailnet.
- Discovery uses UDP broadcast and does not cross subnets or Tailscale — enter
  the address manually in those cases.
- The app keeps working while the phone is on cellular **only** if the PC is
  reachable through a VPN such as Tailscale.
