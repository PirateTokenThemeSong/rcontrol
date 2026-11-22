# Server guide

The server is a single Windows executable that sits in the tray and serves the
phone over your LAN. This page covers everything you can do with it.

## Tray icon

The icon lives in the notification area. Windows 11 may tuck new icons under the
`^` overflow — drag it onto the taskbar to pin it.

- **Left click** — open the settings page in your browser
  (`http://127.0.0.1:7948`).
- **Right click** — `Settings…`, `Open data folder`, `Exit`.

The settings page only listens on loopback, so other devices on the network
cannot open it.

## Settings

### Status

PC name, how long the server has been running, whether the unlock helper is
installed, and **every address** the server is reachable on — Ethernet, Wi-Fi,
Tailscale and so on — each with a **Copy** button. Use one of those addresses in
the app.

### Pairing

- **PIN** — the six digits you type once on the phone. **New PIN** rotates it.
  Rotating the PIN does not disconnect phones that are already paired.
- **Unpair all devices** — rotates the access token. Every phone must pair again
  with the current PIN.

### Server

- **Port** — the TCP port the phone connects to (default `7945`). **Save &
  restart** writes the new port to the config and restarts the server.
- **Start at login** — creates a hidden login entry so the server comes up with
  your session. It needs your interactive session for screen capture and app
  launching, so it cannot start before you sign in.
- **Data folder** — opens `%USERPROFILE%\.rcontrol`.
- **Exit rcontrol** — stops the server and removes the tray icon.

## Power actions

| Action | What happens |
|---|---|
| Shut down | `shutdown /s /t 5` — five seconds to change your mind |
| Reboot | `shutdown /r /t 5` |
| Sleep | `rundll32 powrprof.dll,SetSuspendState` |
| Lock | locks the workstation |
| Turn displays off | screens sleep, the PC stays awake |

Shut down and reboot ask for confirmation in the app. The others act
immediately.

## Windows

The app lists every visible top-level window in front-to-back order, with the
current one marked **active** and minimised ones marked **min**. Tap a row to
bring that window to the front; the list refreshes automatically after each
switch. Restoring a minimised window works too.

## Apps

The **Apps** tab in the phone has two sources:

1. **Start Menu search** — search any installed program by name. Results are
   limited to shortcuts inside the real Start Menu folders.
2. **Allowlist** — `%USERPROFILE%\.rcontrol\apps.json`, a sample is created on
   first run:

```json
{
  "steam":   {"label": "Steam",   "windows": "C:\\Program Files (x86)\\Steam\\steam.exe", "linux": "steam"},
  "notepad": {"label": "Notepad", "windows": "notepad.exe"}
}
```

Each entry needs a `label` and a command for your OS (`windows`, `linux` or
`darwin`), plus optional `args` and `dir`. Unknown ids are rejected; the phone
only ever sees ids and labels, never command paths.

> There is no editor for this file in the settings page — edit it in a text
> editor and restart the server (or just use Start Menu search).

## Unlock

Windows blocks synthetic input on the lock screen, so unlocking needs an optional
SYSTEM helper. The release zip includes it (`rcontrol-unlock.exe`).

**Install once, as administrator:**

```powershell
.\rcontrol-unlock.exe -install
```

It copies itself to `%ProgramData%\rcontrol`, registers a SYSTEM service that
starts automatically, and creates a shared token. Remove it any time with
`.\rcontrol-unlock.exe -uninstall` (as administrator).

**How it works in the app:** when Windows is locked, the control screen shows a
**PC is locked** card with a PIN/password field. Type the Windows PIN (or
password) and press **Unlock**; the helper reveals the sign-in screen, types the
secret and presses Enter.

Things to know:

- The stream **freezes** while the sign-in screen is up — the secure desktop
  cannot be captured from the user session. It resumes once the unlock succeeds.
- Use whatever the lock screen asks for: if it shows a PIN pad, type the PIN; a
  password needs *Sign-in options* at the keyboard.
- If the helper is not installed, the button reports that instead of failing
  silently. The helper's log is `%ProgramData%\rcontrol\helper.log`.

## Wake

The PC cannot serve anything while asleep, so waking starts on the phone:

- the phone sends a Wake-on-LAN magic packet directly, and/or
- it fires an HTTP request at your router, Home Assistant or an always-on Pi,
- then polls `GET /health` until the server answers.

WoL must be enabled in the BIOS and the network adapter ("allow this device to
wake the computer"). Configure it in the app — see
[the Android guide](android.md#wake).

## Data files

| Path | What |
|---|---|
| `%USERPROFILE%\.rcontrol\pin` | pairing PIN |
| `%USERPROFILE%\.rcontrol\token` | access token (rotate via *Unpair*) |
| `%USERPROFILE%\.rcontrol\server_id` | stable id shown in discovery |
| `%USERPROFILE%\.rcontrol\apps.json` | app allowlist |
| `%USERPROFILE%\.rcontrol\config.json` | saved port |
| `%ProgramData%\rcontrol\` | unlock helper, its token and log |

Delete the folder to reset the server (you will need to pair again).

## Security

- Every endpoint except `/health` and `/pair` requires the 32-byte token; `/pair`
  is rate-limited to 10 attempts per minute per IP.
- There is **no TLS** in v1. Use it on a network you trust, or over Tailscale.
- The settings page is loopback-only; the unlock helper listens on loopback with
  its own token.
- `--no-auth` exists for testing and disables the token entirely — anyone on the
  LAN can then control the PC.
