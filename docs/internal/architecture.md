# Internal: architecture

## Shape

```
rcontrol/
  apps/server/          Go server (Windows-first)
    cmd/rcontrol-unlock optional SYSTEM helper for the lock screen
    internal/
      admin/            loopback settings UI (127.0.0.1:7948) + /api/*
      auth/             PIN + token store, rotation
      config/           ~/.rcontrol/config.json
      desktop/          window list, focus, lock detection, window icons
      discovery/        UDP responder
      icons/            HICON → PNG extraction
      launch/           allowlist + Start Menu search + launch
      power/            shutdown/reboot/sleep/lock/display
      screen/           GDI capture (Windows), scrot/grim (Linux)
      startup/          "start at login" entry
      transport/        HTTP + WebSocket routes
      tray/             Win32 notification-area icon
      unlock/           client for the helper
  apps/android/         Kotlin + Compose app (:app)
  protocol/PROTOCOL.md  wire format (source of truth)
  brand/                launcher PNG exports + SVG
  docs/                 user docs; docs/internal/ for this
  .github/workflows/    release + nightly
```

## Transport

One TCP port (default 7945) carries everything:

- `GET /health` — unauthenticated liveness, used by the wake poller.
- `POST /pair` — PIN → token, rate-limited.
- `GET /apps`, `/search`, `/launch`, `/windows`, `/focus`, `/icon`, `/unlock`,
  `/lockstate` — token-authenticated JSON/PNG.
- `GET /ws?token=…` — the session: binary messages are JPEG frames, text
  messages are JSON events (`hello`, `quality`, `power`, `launch`, `windows`,
  `focus`, `locked`, acks, errors).
- `GET /stream?token=…` — MJPEG fallback.

The WebSocket writer serialises all frames/acks through a mutex (Gorilla allows
one writer). Discovery is a separate UDP responder on 7946.

`protocol/PROTOCOL.md` is the source of truth for payloads; update it with any
wire change.

## Server lifecycle

1. Load `~/.rcontrol` credentials (server id, PIN, token) and the apps
   allowlist (sample created on first run).
2. Bind the HTTP listener, start the UDP responder.
3. Start the settings server on loopback and the tray icon.
4. Block until Ctrl-C/SIGTERM; the tray's Exit and the settings page's Exit call
   the same shutdown path.

Restarting writes the port to `config.json`, spawns a fresh process and exits;
the child retries the listener for a few seconds to survive the handover.

## Auth

- 6-digit PIN (`auth.Store.CheckPIN`), normalised for dashes/spaces.
- 32-byte token, constant-time compare, stored `0600`.
- `/pair` is limited to 10 attempts/minute per IP.
- One token for all devices; **Unpair all** rotates it.
- No TLS in v1: the token is a bearer secret on the LAN.

## Windows-specific behaviour

- **Capture** — GDI `BitBlt` of the primary display at true physical pixels.
  The process declares per-monitor DPI awareness in an `init()`, otherwise
  `GetSystemMetrics` returns scaled dimensions and the frame is a cropped corner
  on a 125%/150% display.
- **Windows list** — `EnumWindows`, filtered to visible, unowned, non-tool
  windows, in z-order. Focus restores minimised windows and uses
  `AttachThreadInput`/`BringWindowToTop`/`SetForegroundWindow`, with an Alt
  fallback.
- **Icons** — apps resolve to their exe or `.lnk` and go through
  `SHGetFileInfo`; windows use `WM_GETICON`, falling back to the owning
  process's exe. `HICON` is drawn into a 32-bit DIB and encoded as PNG.
- **Lock detection** — the foreground process is `LockApp.exe` / `LogonUI.exe`.
- **Unlock** — the helper runs as a SYSTEM service, spawns a typist in the active
  session with the winlogon token, sets `TokenUIAccess` (without it `SendInput`
  is denied on the secure desktop), switches to the input desktop with the
  TigerVNC rights mask, presses F15 and **re-attaches** (that key moves the
  session to the secure desktop), then types the secret and Enter.
- **Tray** — a message-only Win32 window + `Shell_NotifyIcon`; left click opens
  the settings URL, right click builds a popup menu with Settings / Open data
  folder / Exit. The icon is rendered from the transparent PNG and cropped to
  its alpha bounds so it fills the notification area.
- **Startup** — a hidden `.vbs` in the user's Startup folder runs the exe.

## Data

| Path | Contents |
|---|---|
| `~/.rcontrol/server_id` | stable discovery id |
| `~/.rcontrol/pin` | pairing PIN |
| `~/.rcontrol/token` | access token |
| `~/.rcontrol/apps.json` | allowlist |
| `~/.rcontrol/config.json` | saved port |
| `%ProgramData%\rcontrol\` | helper exe, helper token, helper log |

## Platform support

| Capability | Windows | Linux | macOS |
|---|---|---|---|
| Stream | GDI | `scrot`/`import`/`grim` | no |
| Power | full | systemctl/loginctl/xset | display only |
| Launch | allowlist + Start Menu | allowlist + `.desktop` | allowlist only |
| Windows / icons / lock / unlock | yes | no | no |

The network, auth and protocol layers are portable; OS features are behind
build tags with stubs.

## Android

- One module, Compose, no XML layouts. Strings live in
  `res/values/strings.xml`; add `values-<lang>/strings.xml` for translations.
- Theme is the "Ice" palette: flat fills only, no gradients.
- Screens: Home (actions), Control (stream/power/switcher), Wake, Settings
  (everything typed lives here).
- `net/` holds plain suspend functions over OkHttp; `RControlClient` owns the
  WebSocket and exposes frames/lock state as Compose state.
- The APK is debug-signed in CI for now.

## Known limitations

- No TLS, one shared token, no per-device revocation (Unpair rotates all).
- No mouse/keyboard input — the stream is view-only apart from the unlock text.
- The secure sign-in desktop cannot be captured, so the stream freezes there.
- One session per PC; no multi-monitor selection.
