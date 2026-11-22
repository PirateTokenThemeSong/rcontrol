# RControl wire protocol — v1 (LAN only)

Transport: HTTP + WebSocket on one TCP port. Default `7945`.
Discovery: UDP port `7946`.

## 1. Discovery (UDP broadcast)

Client broadcasts (255.255.255.255:7946, also 239.0.0.1 multicast-ready):

```
RC_DISCOVER v1
```

Server replies via unicast UDP JSON (one datagram):

```json
{
  "name": "DESKTOP-RICKY",
  "id": "a3f1-...",
  "addr": "192.168.1.20",
  "port": 7945,
  "version": 1,
  "auth": true
}
```

`auth=false` means server runs with `--no-auth` and `/ws` accepts no token.

Android also supports manual IP entry — discovery is convenience only.

## 2. Pairing (HTTP)

Server generates a 6-digit PIN on first run (shown on PC console, stored in
`~/.rcontrol/pin`). Token stored in `~/.rcontrol/token`.

```
POST /pair
Content-Type: application/json

{"pin": "482-913"}
```

Responses:

- `200 {"token":"<opaque>"}` — use as `?token=` on `/ws` and `/stream`
- `401 {"error":"bad pin"}` — wrong PIN
- `423 {"error":"already paired"}` — not used in v1, token never expires

Health (no auth):

```
GET /health → 200 {"ok":true,"version":1,"name":"..."}
```

## 3. Control + screen (WebSocket)

```
GET /ws?token=<token>
```

Auth failure → close code `4401`.

### Server → client

- **Binary messages = one JPEG frame** (latest screen, scaled per quality setting).
  Decode directly, no envelope. First byte is always `0xFF` (JPEG SOI).
- **Text messages = JSON events**, e.g. `{"type":"hello","width":1920,"height":1080,"fps":5}`.

### Client → server (JSON text only)

```json
{"type": "ping"}
{"type": "quality", "fps": 5, "jpeg": 70, "maxWidth": 960}
{"type": "power", "action": "shutdown"}
{"type": "launch", "app": "steam"}
```

| type    | fields | notes |
|---------|--------|-------|
| `ping`  | — | server replies `{"type":"pong"}` |
| `quality` | `fps` 1–15, `jpeg` 30–90, `maxWidth` 320–1920 | live-adjusts capture loop |
| `power` | `action`: `shutdown\|reboot\|sleep\|lock\|display` | `shutdown`/`reboot` have 5 s delay; `display` turns monitors off (any input wakes them); reply `{"type":"power-ack","action":"..."}` |
| `launch` | `app`: allowlist id (see §6) | reply `{"type":"launch-ack","app":"..."}` or `{"type":"error",…}` |
| `windows` | — | reply `{"type":"windows","windows":[…]}` (see §8) |
| `focus` | `id`: window id from `windows` | reply `{"type":"focus-ack","id":"..."}` or `{"type":"error",…}` |

Unknown types → `{"type":"error","error":"unknown type"}`. Never crash on bad input.

## 4. MJPEG fallback (HTTP, optional but implemented)

```
GET /stream?token=<token> → multipart/x-mixed-replace JPEG stream
```

Useful for debugging with `curl` / browser. Android uses WebSocket by default.

## 5. Security model (v1)
- Opaque 32-byte token, constant-time compare.
- No TLS (LAN assumption). Bind `0.0.0.0` but log a warning.
- Rate-limit `/pair` (max 10 attempts/min per IP) — prevents PIN brute force.
- App launching: `apps.json` allowlist plus Start Menu search. Start Menu ids
  are validated to stay inside the real Start Menu / applications dirs; the
  phone sees ids + labels, never command paths.
- Future: TLS, per-device tokens, mouse/keyboard (v2).

## 6. App launching

Allowlist file `~/.rcontrol/apps.json` (override with server flag `--apps`;
a sample is created on first run):

```json
{
  "steam":   {"label": "Steam",   "windows": "C:\\Program Files (x86)\\Steam\\steam.exe", "linux": "steam"},
  "notepad": {"label": "Notepad", "windows": "notepad.exe"}
}
```

Per entry: `label`, one command per OS (`windows`/`linux`/`darwin`),
optional `args` array and `dir` working directory. Unknown ids and ids
without a command for the server OS are rejected without executing anything.

- `GET /apps?token=…` → `200 {"apps":[{"id":"steam","label":"Steam"}]}`
- `POST /launch?token=…` body `{"app":"steam"}` → `200 {"type":"launch-ack","app":"steam"}`
- WS `{"type":"launch","app":"steam"}` → `{"type":"launch-ack","app":"steam"}`
- `GET /search?token=…&q=steam` → installed Start Menu apps matching the query
  (Windows `.lnk`, Linux `.desktop`), capped at 25. Launch them with the same
  `/launch` / WS message using the returned `startmenu:…` id. Paths are
  containment-checked server-side.

## 7. Wake flow (client-side, phone polls until online)

The sleeping PC can't run the server, so waking is initiated by the phone:

1. (option A) Phone sends a WoL magic packet directly (UDP, default
   `255.255.255.255:9`) to the PC's MAC.
2. (option B) Phone fires a **fully user-configurable HTTP request** —
   method, URL, headers, body — at whatever wakes the PC on your network
   (router API, Home Assistant, always-on Pi). Both can be used together.
3. Phone polls `GET /health` every ~2.5 s (5 min timeout) until `200 {"ok":true}`,
   then opens the control session.

All wake settings (MAC, broadcast, HTTP request) are stored on-device and
editable in the app's Wake screen. Requires WoL enabled in BIOS + NIC
("allow this device to wake the computer").

## 8. Open windows (focus switcher)

Windows-only in v1 (`EnumWindows` + `SetForegroundWindow`, with restore for
minimized windows). The list is in z-order, front-most first; `id` is an opaque
handle that only means anything while that window exists.

- `GET /windows?token=…` → `200 {"windows":[{"id":"68058","title":"notes.txt — Editor","active":true,"minimized":false}]}`
- `POST /focus?token=…` body `{"id":"68058"}` → `200 {"type":"focus-ack","id":"68058"}`
  (400 if the window is gone or Windows refuses the switch)
- WS `{"type":"windows"}` → `{"type":"windows","windows":[…]}`
- WS `{"type":"focus","id":"68058"}` → `{"type":"focus-ack","id":"68058"}`

Tool windows and owned popups are filtered out, and at most 64 windows are
listed. The phone never sees process ids or handles beyond these opaque ids.

## 8b. Icons

`GET /icon?token=…&kind=app|window&id=…` → `image/png` (64×64), the real icon:
apps resolve to their exe (allowlist) or `.lnk` (Start Menu) and go through
`SHGetFileInfo`; windows go through `WM_GETICON` with the owning process exe as
fallback. `404` when there is none — the app then shows its letter/glyph
placeholder.

## 9. Locked session

`GET /lockstate?token=…` → `{"locked":true}`. The WebSocket pushes
`{"type":"locked","locked":…}` whenever the state changes (checked once a
second), and answers `{"type":"lockstate"}` with the current value. The window
list at `GET /windows` also carries `locked`.

Unlocking needs the optional SYSTEM helper (`apps/server/cmd/rcontrol-unlock`),
installed once as admin:

```
rcontrol-unlock.exe -install      # copies to %ProgramData%
control, starts service
rcontrol-unlock.exe -uninstall
```

It listens on `127.0.0.1:7947` with a random token in
`%ProgramData%
control\helper.token` (ACLs: SYSTEM/Admins full, Users read).
On request it spawns a typist in the active session with the winlogon token,
sets `TokenUIAccess` on it (without it `SendInput` returns Access denied on the
secure desktop), switches the thread to the input desktop (GENERIC_WRITE + the
TigerVNC desktop mask), presses F15, **re-attaches** to the input desktop
(pressing a key moves the session from the lock wallpaper to the secure
sign-in desktop), types the secret and presses Enter.

- `POST /unlock?token=…` body `{"secret":"1234"}` → `200 {"type":"unlock-ack"}`,
  `400 {"error":"…"}` (helper missing/failed), `429` after 10 attempts/min per IP.
- The secret is never stored; the app clears it when `locked` flips to false.

Note: while the secure sign-in desktop is up, screen capture (which runs in
the user session on the default desktop) returns no frames, so the stream
freezes until the unlock succeeds or the user signs in at the keyboard.
