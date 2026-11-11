# Codex Queue Studio v3.0.0

A native Windows 11 companion app for Codex session discovery and scheduled messages.

The v3 rewrite replaces browser-hosted UI with native Win32 controls. It adds compact virtualized session lists, conversation previews, `Guardian review` title recovery, managed Codex App Server delivery, DPAPI-encrypted reservations, and a verified GitHub-release update path.

**Status:** Windows cross-build and core unit tests pass. Live Windows 11 UI and Codex Desktop compatibility still require validation. Do not treat this as a stable release.

Developers: Go 1.23+, `go test ./...`, then `go build -ldflags "-H windowsgui -s -w" -o CodexQueueStudio.exe .`.

Codex Queue Studio is independent and not an official OpenAI or Microsoft product.
