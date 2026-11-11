# Codex Queue Studio v3.0.0

A native Windows 11 session explorer and scheduler for OpenAI Codex.
This is an independent companion app. It is not an official OpenAI product.

## What changed from v2

- Native Win32 GUI with Windows 11 minimize, maximize, restore, and close controls.
- Native common controls and app icon resources. No Edge, Chrome, WebView2,
  Qt, Electron, Python, or BAT file is required to open the GUI.
- Virtualized, compact session list. Search by name, project, path, or UUID.
- Preferred session naming uses explicit human names. Generic titles such as
  `Guardian review` are replaced by a meaningful user message when available.
- Local JSONL session discovery works without an installed Codex CLI.
- Optional App Server `thread/list` integration supplements local session data.
- The managed Codex runtime supports `thread/resume` + `turn/start` and records
  terminal outcomes. Users do not need to install `codex` on PATH.
- Scheduled work is stored in the same user profile with Windows DPAPI encryption.
- Old v2 pending CLI-queue jobs are paused for user review before conversion.
- GitHub release update checks verify archive SHA-256 before installation.

## Get started

1. Download the release ZIP from GitHub Releases.
2. Extract the full ZIP to a writable folder. Keep the included files together.
3. Start `CodexQueueStudio.exe`. Do not launch an old v2 background instance.
4. Choose a session from the compact explorer.
5. Review its transcript. Enter a message and select a date and time.
6. Select **지금 전송** or **예약 등록**.
7. Keep Windows running. Closing the window moves the app into the tray.
   Double-click the tray icon to reopen it. Select **완전히 종료** to stop it.

The release ZIP includes `engine/codex.exe` from the verified official Codex
release. If only the standalone GUI executable is available, use **환경 설정 →
Codex 엔진 설치**. This downloads the official runtime after confirmation.
Installation requires internet access and enough free disk space.

**Important:** Codex Desktop and a separate App Server are different runtime
processes. Resuming a stored local session is supported by the Codex protocol.
Injecting text into the *currently open desktop input field* is not supported.
Concurrent editing of the same thread can cause conflicts. Test on a non-critical
session before enabling scheduled unattended commands.

**Approval safety:** Scheduled work does not automatically approve interactive
security prompts. Requests needing approval can fail. Background completion
cannot be guaranteed when the PC is off, asleep, logged out, or disconnected.

## Build from source

Go 1.23 or later is required for development.

```powershell
go test ./...
go vet ./...
go build -ldflags "-H windowsgui -s -w" -o CodexQueueStudio.exe .
go build -ldflags "-H windowsgui -s -w" -o CodexQueueUpdater.exe ./cmd/updater
```

Build on Windows. Keep `CodexQueueUpdater.exe` beside the application for
in-place updates. No BAT file is required. The `.syso` resource embeds a Windows
application icon and a Common Controls v6 manifest.

For reproducible icon resources, run `python assets/build_resources.py`.
The script uses the Python standard library. It is not required at runtime.

## Settings and storage

- Session data: `%USERPROFILE%\.codex\sessions` and
  `%USERPROFILE%\.codex\archived_sessions`, or `CODEX_HOME`.
- Reservation state: `%LOCALAPPDATA%\CodexQueueStudio\state.json`.
- Managed runtime: `%LOCALAPPDATA%\CodexQueueStudio\engine\codex.exe`.
- Staged updates: `%LOCALAPPDATA%\CodexQueueStudio\updates`.
- Program settings: theme, Codex home directory, and existing scheduled tasks.

## Automatic updates

The application checks the repository's latest published stable GitHub release
at startup. A checked update is installed only after user confirmation.
The updater stages the ZIP, checks its SHA-256 digest, backs up the previous
binary, replaces the GUI executable, and launches the updated GUI.

Create a signed release by pushing a version tag (`v3.0.1`, for example).
`.github/workflows/release.yml` builds the GUI and updater, downloads the
official Codex Windows binary, verifies it, and publishes a ZIP and `.sha256`.
The current updater replaces the **main GUI binary**. To update the engine or
updater helper itself, reinstall the full release ZIP.

## Verification

Run `go test ./...` and check `docs/VERIFICATION.md`. CI validates both Windows and Ubuntu.
The current non-Windows build environment can cross-compile Windows PE binaries.
It cannot verify actual GUI rendering or a live Codex Desktop session.
A successful simulated App Server test is not proof of live server compatibility.

## Security

- Messages are encrypted at rest using Windows DPAPI per user.
- The app uses an authenticated local Codex user profile. It stores no API key.
- Engine downloads are restricted to official OpenAI Codex release assets.
- Release ZIPs are restricted to this repository and require SHA-256 verification.
- Interactive approvals are not silently accepted.
- Interrupted tasks are marked uncertain instead of silently being replayed.
- No localhost HTTP server, browser cookie, or browser extension is required.

See `docs/AUDIT.md` for remaining risks and `docs/WORK_ORDERS.md` for scope.
