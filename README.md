# rcontrol — LAN PC remote (monorepo)

Native Android app + Go PC server. LAN only, v1 scope: **screen streaming + power actions**.

```
rcontrol/
  apps/server/    Go server — runs on your PC (Windows/Linux), captures screen, executes power actions
  apps/android/   Native Android — Kotlin + Jetpack Compose, live screen + power buttons
  protocol/       Shared wire protocol (single source of truth)
```

## Quick start

### 1. PC server (Go 1.22+)

```powershell
cd apps/server
go mod tidy
go run . --port 7945
# first run prints a pairing PIN, e.g. 482-913
# POST /pair with the PIN to get a token, or pass --no-auth for home LAN testing
go run . --port 7945 --no-auth
```

Build a single binary:

```powershell
go build -o bin/rcontrol-server.exe .
.\bin\rcontrol-server.exe --port 7945
```

Run the API tests (httptest-based e2e, no real shutdowns/launches — side
effects are stubbed):

```powershell
go test ./...
```

Allowlisted apps (`~/.rcontrol/apps.json`, sample auto-created — edit it to
add your programs, e.g. Steam, Kodi). Phone lists them via `GET /apps` and
launches via `POST /launch` or WS `{"type":"launch","app":"steam"}`.

Windows power actions use `shutdown /s /t 5`, `/r`, `rundll32 powrprof.dll,SetSuspendState`, `rundll32 user32.dll,LockWorkStation`.
Linux uses `systemctl poweroff/reboot/suspend`, `loginctl lock-session`.

### Tray & settings (Windows)

The server puts an icon in the notification area:

- **left click** → opens the local settings page (`http://127.0.0.1:7948`)
- **right click** → Settings…, Open data folder, Exit

The settings page covers status (every interface address — LAN, Tailscale, etc.
— listed with a copy button), the pairing PIN (show / rotate), unpairing all
devices, the server port (save & restart), start at login, the data folder and
Exit. Anything that survives restarts lives in
`~/.rcontrol/config.json`. Windows 11 may tuck new tray icons under the `^`
overflow — drag it onto the taskbar to pin it.

### 2. Android app

Open `apps/android` in Android Studio (Hedgehog+), run on a physical device
on the **same Wi-Fi** as your PC. The first screen is action-only: tap **Set
up PC** (gear icon) once to enter the address / token or pair with the PIN,
then **Connect** or **Wake**. Screen quality and all wake options also live
in Settings — the main screens never ask you to type config.

## Protocol (v1)

See [`protocol/PROTOCOL.md`](protocol/PROTOCOL.md). Summary:

- Discovery: UDP broadcast port `7946` — client sends `RC_DISCOVER`, server replies JSON.
- Pairing: `POST /pair {"pin":"482-913"}` → `{"token":"..."}`
- Open windows: `GET /windows` + `POST /focus` (focus switcher, Windows v1)
- Locked session: `GET /lockstate`, `POST /unlock` (needs the optional SYSTEM helper, see `protocol/PROTOCOL.md` §9)
- Streaming + control: `GET /ws?token=...` (WebSocket)
  - Server → client: binary JPEG frames (decode with `BitmapFactory`)
  - Client → server JSON: `{"type":"power","action":"shutdown|reboot|sleep|lock"}`, `{"type":"quality","fps":5,"jpeg":70}`, `{"type":"launch","app":"steam"}`, `{"type":"windows"}`, `{"type":"focus","id":"…"}`, `{"type":"ping"}`
- Health: `GET /health`

## Repo layout / tooling

- No JS toolchain at root — each app builds with its native toolchain.
- `make server` / `make android-debug` shortcuts in `Makefile` (optional).

## Security notes (LAN v1)

- Token auth on HTTP+WS. For home LAN, `--no-auth` is fine for testing.
- No TLS in v1 (LAN only). Don't expose port `7945` to the internet.
- V2 ideas: TLS + per-device certs, mouse/keyboard HID, WebRTC, relay for remote access.
