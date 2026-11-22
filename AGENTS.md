# AGENTS.md

## What this is

`rcontrol` — LAN-only remote for a Windows PC. Go server (screen stream, power,
apps, window focus, lock/unlock) + native Android app (Kotlin + Compose).
No cloud, no TLS in v1.

## Layout

- `apps/server/` — Go module `rcontrol-server`; `cmd/rcontrol-unlock` is the
  optional SYSTEM helper that types into the lock screen.
- `apps/server/internal/tray` — Windows notification-area icon (left click =
  settings, right click = menu with Exit).
- `apps/server/internal/admin` — loopback settings UI (`127.0.0.1:7948`) and
  its API; keep it bound to loopback only.
- `apps/android/` — one Gradle module `:app`, Jetpack Compose UI.
- `protocol/PROTOCOL.md` — wire protocol; the source of truth. Update it with
  any endpoint or message change.
- `brand/` — launcher PNGs (48–512) and the SVG source.

## Build & test

```powershell
cd apps/server
go vet ./... && go test ./... && go build -o bin/rcontrol-server.exe .
go build -o bin/rcontrol-unlock.exe ./cmd/rcontrol-unlock
```

Android (there is no wrapper checked in; use Android Studio or a local Gradle 8.x):

```powershell
$gradle = (Get-ChildItem "$env:USERPROFILE\.gradle\wrapper\dists\gradle-8.14.1-bin\*\gradle-8.14.1\bin\gradle").FullName
& $gradle --no-daemon assembleDebug
```

AGP 8.5.2, Kotlin 2.0.20, JDK 17+. Emulator AVD `rcontrol` (API 33, x86_64);
`adb install -r app/build/outputs/apk/debug/app-debug.apk`; the host is
reachable from the emulator at `10.0.2.2`.

## Conventions

- **UI:** screens carry actions only; anything you type lives in Settings (gear).
  Ice palette, flat fills — **no gradients**. Green/amber/coral only for state
  (online / warning / error).
- **Pairing:** the user types the PIN; the token is internal and must never be
  shown as an editable field. Store it, don't expose it.
- **Server:** everything is token-authed except `/health` and `/pair`; bump
  `PROTOCOL.md` whenever the wire changes.
- **Secrets:** never commit `~/.rcontrol`, the pairing PIN, tokens, or
  `%ProgramData%\rcontrol`. The repo must build from a clean checkout.

## Workflow (required)

After **every** change:

1. run the relevant build and tests,
2. commit with a short imperative message,
3. push to `origin main` (the credential helper is configured).

Do not batch unrelated work; do not leave the tree dirty when you stop.
