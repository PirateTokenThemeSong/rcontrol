# Getting started

Two pieces: the **server** on the Windows PC, and the **app** on an Android
phone. They talk directly over your LAN.

## 1. The PC

1. Download `rcontrol-server-windows-amd64.zip` from the
   [latest release](https://github.com/PirateTokenThemeSong/rcontrol/releases)
   and extract it somewhere permanent (e.g. `C:\rcontrol`).
2. Run `rcontrol-server.exe`. Windows Firewall will ask for access — allow it on
   **private** networks (it needs TCP 7945 and UDP 7946).
3. A console window prints the pairing PIN and the address. The same PIN is on
   the settings page and in `%USERPROFILE%\.rcontrol\pin`.

   Running with no arguments listens on `0.0.0.0:7945`.

The first time, a sample `%USERPROFILE%\.rcontrol\apps.json` is created with a
couple of entries; edit it to add your own programs (see the
[server guide](server.md#apps)).

> The server needs your **interactive session** (screen capture and app launching
> live there). It starts at login if you enable it in the settings page — see
> [start at login](server.md#settings).

## 2. The phone

1. Download `rcontrol-v0.1.0.apk` from the same release and open it on the phone.
   Allow "install unknown apps" for the browser — the APK is debug-signed for now.
2. Open **rcontrol**. Tap the gear and fill in:
   - **Address** — the PC's LAN address shown in the server settings page
     (e.g. `192.168.1.67`). Over Tailscale, use the Tailscale address instead.
   - **Port** — `7945` unless you changed it.
3. Enter the PIN under **Pairing** and tap **Pair**. The app stores a token; you
   never type the token itself.
4. Go back and tap **Connect**.

That's it — you should see the PC screen, the power menu and the Windows/Apps
switcher.

## 3. Optional

- **Wake the PC from the phone** — configure WoL or an HTTP wake request in
  [the app's settings](android.md#wake).
- **Unlock the PC remotely** — install the optional helper as administrator; see
  [remote unlock](server.md#unlock).
- **Start with Windows** — toggle *Start at login* in the
  [server settings](server.md#settings).

## Next

- [Server guide](server.md)
- [Android guide](android.md)
- [Troubleshooting](troubleshooting.md)
