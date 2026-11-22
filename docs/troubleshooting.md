# Troubleshooting

## The phone can't find the PC

- Both devices must be on the **same network**. Discovery uses UDP broadcast on
  port **7946** and does not cross subnets or Tailscale — type the address
  manually instead.
- Allow the server through Windows Firewall on **private** networks (TCP 7945,
  UDP 7946).
- The server must be running (tray icon present) and you must be signed in — it
  needs your interactive session.
- Over Tailscale, use the Tailscale address shown in the server settings and
  make sure both devices are in the same tailnet.

## Connect fails

- Check the address and port in the app against the server settings page.
- If the app says the PC needs pairing (or the status mentions `4401`), open
  Settings and pair with the current PIN. After **Unpair all devices** every
  phone must pair again.
- Is the server running with `--no-auth`? Then no token is needed; leave the PIN
  empty and just connect.

## The stream is black, frozen or cropped

- **Frozen on the sign-in screen** — expected. The secure desktop cannot be
  captured; the stream returns when the desktop does.
- **Cropped or squashed** — on Windows the server now declares DPI awareness, so
  the full desktop is captured. If you run an older build, update.
- **Black** — the PC may be on a lock/sign-in screen, the display is off, or the
  session has no desktop. Check the PC.
- **Laggy** — lower *JPEG quality* or *Max width* in the app's settings; the
  stream is re-encoded at 1–15 fps.
- Ports: make sure the port in the app matches the server (default `7945`).

## Unlock doesn't work

- The optional helper must be installed **as administrator**:
  `rcontrol-unlock.exe -install`. The settings page shows whether it is
  installed.
- Use what the lock screen asks for: a PIN pad needs the PIN; a password needs
  *Sign-in options* at the keyboard first.
- Read `%ProgramData%\rcontrol\helper.log` — it records whether the typist got
  the input desktop and typed.
- The stream freezes while the sign-in desktop is up; that is not a failure.

## Wake doesn't work

- Enable Wake-on-LAN in the BIOS **and** in the adapter settings ("allow this
  device to wake the computer").
- Use the PC's MAC and the LAN broadcast address (e.g. `192.168.1.255`), not
  `255.255.255.255` on networks that drop directed broadcast.
- For a custom HTTP request, test the URL from a browser/curl first; the app
  reports the status code in the wake status.
- If the PC is fully powered off (not sleeping), WoL must be supported by the
  board in that state (S5).

## The APK won't install

- Releases are **debug-signed**. A build from a different machine/CI run may
  have a different signature: uninstall the old app first.
- Allow "install unknown apps" for whichever app opens the APK.

## The tray icon is missing

- New tray icons are hidden under the `^` overflow on Windows 11 — drag it onto
  the taskbar.
- If the server was started hidden, it is still running; check the settings page
  at `http://127.0.0.1:7948`.

## The settings page doesn't open

- It listens on loopback only: `http://127.0.0.1:7948`. If another process holds
  that port, the server logs it and the tray's Settings entry will not work.
- Restarting the server re-creates it.

## Logs

- The server logs to its console. If it was started by the login entry, the
  window is hidden — start it manually from a terminal to see the output.
- The unlock helper writes to `%ProgramData%\rcontrol\helper.log`.
