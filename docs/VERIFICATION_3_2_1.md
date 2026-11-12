# Codex Queue Studio v3.2.1 — test and acceptance record

## Observed defect

The existing 3.2.0 updater accepted a matching window handle without verifying that its message pump remained responsive. The user reported that a new version window appeared but displayed “Not Responding.” The cause has not been proven by a crash dump.

## Fixes

1. The installed GUI emits a startup marker only from a Win32 message-loop timer.
2. The updater validates the token and child PID. It then checks `SendMessageTimeoutW` repeatedly.
3. The updater keeps the previous binaries until startup validation completes.
4. The updater stops a failed child, restores backups, and attempts to restart the previous version.
5. The GUI shows official GitHub release notes and streaming download, integrity, and staging progress.
6. A separate native updater window shows file replacement and startup-validation progress.
7. Existing `state.json` and Codex session history are never replaced by the updater.
8. Updates are blocked during active sends and immediately due reservations.

## Checks completed in Linux container

- `go test -count=1 ./...`: PASS.
- `go test -race ./...`: PASS.
- `go vet -unsafeptr=false ./...`: PASS.
- `GOOS=windows GOARCH=amd64 go build` for GUI/updater: PASS.
- App metadata and release-note tests: PASS.
- Download progress and verification fixture: PASS.

## Windows CI gates

- Windows `go test ./...` includes a staged replacement and rollback fixture.
- Windows build includes a Win32 UI smoke test.
- Production GUI must write a readiness marker through its message loop.
- Startup acceptance test keeps the production GUI alive after its marker.
- Tagged release must publish exactly two ZIPs, with per-file SHA-256 verification.

## Unverified

- Reproduction of the user's exact hang without a Windows crash dump.
- Live 3.2.0 -> 3.2.1 update and recovery on the user's PC.
- Appearance on the user's display, custom DPI, accessibility, and theme preferences.
- Actual Codex Desktop message delivery after the update.

**Production 1.0.0: NO-GO** until real PC acceptance and update rollback are verified.
