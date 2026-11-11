# Codex Queue Studio v3.2.0 — preview update trial

## Verified in source and automated tests
- Version is 3.2.0. This is a preview release, not public v1.0.0.
- Application checks both preview 3.x releases and later stable 1.x promotions.
- The upgrade zip includes app, updater and SHA-256 manifest.
- SHA-256 checks reject corrupt ZIP entries and unexpected files.
- The updater uses a temporary file and atomic rename to replace each executable.
- If launch or native-window verification fails, the updater attempts to restore backups.
- Updating is blocked while a scheduled Codex turn is being submitted.
- Session selection persists across explorer refresh when the session remains present.
- Session list remains compact; secondary card borders improve hierarchy.
- Windows 11 standard window caption buttons, taskbar and notification icon resources remain unchanged.

## Windows acceptance, still pending
1. Install the v3.1.0 full ZIP to a writable directory.
2. Confirm the displayed version and current tasks.
3. Use Settings > Update Check and confirm that v3.2.0 is offered.
4. Approve and install the update from GitHub Releases.
5. Verify application and updater binary SHA-256 against the v3.2.0 manifest.
6. Confirm the restarted process displays 3.2.0 and is not offered again.
7. Verify a scheduled task is retained without duplicate submission.
8. Test interrupted downloads, invalid hash, failed launch and rollback.
9. Inspect Windows scaling, native window controls, dark/light contrast and icon.
10. Validate a noncritical live Codex Desktop scheduled send.

## Release gate
Do not publish v1.0.0 until the above acceptance is complete.
Existing v2 user report: a scheduled send succeeded. This does not establish live v3.x functionality.
