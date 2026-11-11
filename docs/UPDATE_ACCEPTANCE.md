# Update acceptance plan — test channel 3.1.0 to 3.2.0

## Version policy

- 3.1.0: repaired test baseline for the updater.
- 3.2.0: intentionally distinct test release to validate in-app upgrade.
- 1.0.0: reserved for the first production release after Windows acceptance. Do not tag it early.
- Both 3.x builds are marked GitHub prereleases. The 3.x application checks prereleases.
- 1.x stable builds do not install prereleases.
- A stable 1.x release is promoted above 3.x test releases even though its number is smaller.

## Strict GitHub release contract

Each prerelease has exactly two uploaded release assets:
1. CodexQueueStudio-Windows-x64.zip (full app, updater, icon, docs and a verified official Codex runtime).
2. CodexQueueStudio-update-Windows-x64.zip (app EXE, updater EXE, update-manifest.json with SHA-256).

Tag and packaged version must match. Candidate publication is triggered by changing the .release-candidate file on main. GitHub Actions tests the code, builds both executables, verifies file digests, then publishes the two assets. Source archives automatically shown by GitHub are not upload assets.

## Desktop acceptance procedure

1. Download the 3.1.0 full package from its GitHub prerelease.
2. Extract it into a user-writable directory. Save the existing %LOCALAPPDATA%/CodexQueueStudio/state.json.
3. Start CodexQueueStudio.exe, inspect the app icon, session count, reserved messages, and log.
4. Open Settings and select Update Check. Confirm it detects v3.2.0.
5. Select Update Install. Allow the program to exit and restart itself.
6. Confirm the application reports version 3.2.0 and no longer offers v3.2.0.
7. Verify the pending schedule, encrypted message and history survived without duplicate sends.
8. Repeat with a second test schedule, using a noncritical Codex session.
9. Test with network disconnected, corrupted update archive and no write access. Confirm no silent update.
10. Inspect update logs and backup restoration if the new process fails to show its window.
11. Test Windows 11 100%, 125%, and 150% scaling in light and dark themes.
12. Check that tray behavior, Start menu entry (if manually created), taskbar icon, focus and minimum window size remain usable.

## Evidence required before public 1.0.0

- Recorded Windows upgrade from 3.1.0 to 3.2.0, including build hashes.
- GitHub release API confirms exactly two assets on each tag and valid SHA-256.
- GUI opens after replacement and reports 3.2.0.
- State file and selected session continuity verified, no duplicate send.
- Error and rollback tests on real Windows.
- Code signing and Defender/SmartScreen evaluation, or an explicit unsigned-build warning.
- Live Codex Desktop integration and scheduling regression tests.

Do not claim production GO until these are completed.
