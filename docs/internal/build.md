# Internal: build, test, CI

Everything here is for people changing the code. Users want
[Getting started](../getting-started.md).

## Prerequisites

- Go **1.22+** (module `rcontrol-server`)
- JDK **17+**
- Android SDK with platform 34 for the APK (Android Studio, or the command-line
  tools)

## Server

```powershell
cd apps/server
go vet ./...
go test ./...
go build -o bin/rcontrol-server.exe .
go build -o bin/rcontrol-unlock.exe ./cmd/rcontrol-unlock
```

`cmd/rcontrol-unlock` is Windows-only; on other platforms it compiles to a stub.

Run from source:

```powershell
go run . --port 7945          # or --no-auth for LAN testing
```

| Flag | Default | Meaning |
|---|---|---|
| `--port` | config / `7945` | HTTP + WebSocket port |
| `--discovery-port` | `7946` | UDP discovery |
| `--no-auth` | off | disable the token (testing only) |
| `--apps` | `~/.rcontrol/apps.json` | allowlist path |

The port can also be saved in `~/.rcontrol/config.json` from the settings page;
the flag wins when both are set.

## Android

The Gradle wrapper is committed (Gradle 8.14.1, AGP 8.5.2, Kotlin 2.0.20):

```powershell
cd apps/android
.\gradlew.bat --no-daemon assembleDebug
# APK: app/build/outputs/apk/debug/app-debug.apk
```

Emulator (AVD `rcontrol`, API 33, x86_64):

```powershell
emulator -avd rcontrol -no-snapshot
adb install -r app/build/outputs/apk/debug/app-debug.apk
```

The host is reachable from the emulator at `10.0.2.2`, so pairing works with
address `10.0.2.2` and port `7945`.

## CI

Two workflows, both on Ubuntu runners, both building the server for
`windows/amd64` (cross-compiled) and the debug APK:

- `.github/workflows/release.yml` — on tags `v*`; runs the Go tests, builds
  `rcontrol-server.exe` + `rcontrol-unlock.exe` into
  `rcontrol-server-windows-amd64.zip`, builds the APK, and publishes a GitHub
  release with auto-generated notes.
- `.github/workflows/nightly.yml` — cron `0 3 * * *` plus manual dispatch;
  uploads `rcontrol-nightly` as a workflow artifact with 14-day retention.

Cut a release:

```powershell
git tag v0.1.1
git push origin v0.1.1
```

Notes:

- The APK has **no release signing config**, so CI ships the debug build
  (`rcontrol-<tag>.apk`). Adding a keystore as repository secrets plus a
  `signingConfig` is the next step for proper releases.
- Actions are pinned to current majors (checkout v7, setup-go v7, setup-java v6,
  setup-gradle v6, upload-artifact v7, action-gh-release v3).
- Scheduled workflows on public repos are disabled after 60 days without
  repository activity; a push re-arms them and the nightly can be run manually.

## Repo hygiene

- Do not commit `~/.rcontrol`, `%ProgramData%\rcontrol`, tokens, PINs, build
  output (`dist/`), Gradle caches, or `local.properties`.
- `brand/` holds the launcher exports; the app icon itself is an adaptive icon
  (`res/mipmap-anydpi-v26`) with a vector foreground and monochrome layer.
