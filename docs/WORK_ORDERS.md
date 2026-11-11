# v3.0 Work Orders and Release Gates

The full v3.0 implementation is available as a source bundle in the originating ChatGPT conversation. This repository is not yet a complete, buildable mirror. Do not tag or publish a stable release until source parity is verified.

| ID | Requirement | Current state | Release acceptance |
|---|---|---|---|
| WO-01 | Independent native Windows GUI | Implemented locally; cross-build PASS | Windows 11 UI smoke test |
| WO-02 | Native minimize/maximize/close/tray | Implemented locally | DPI, keyboard and tray tests |
| WO-03 | Light/dark contrast | Dedicated Win32 colors | Contrast checks on Windows 11 |
| WO-04 | Compact sessions | Virtual ListView | 20 rows visible at 1350×870 |
| WO-05 | Guardian review title recovery | Fallback to meaningful user message | Check real affected threads |
| WO-06 | Desktop session discovery without CLI | Local JSONL and App Server | Test genuine Desktop profile |
| WO-07 | Message dispatch without installed CLI | Managed official Codex runtime | Live App Server run and auth |
| WO-08 | Durable scheduler | DPAPI, atomic state, crash uncertainty | Forced shutdown test |
| WO-09 | Auto-update | GitHub release check and SHA-256 updater | Real signed update and rollback |
| WO-10 | Application icon | ICO/manifest in local build | Explorer and taskbar visual check |
| WO-11 | OSS and maintainability | Go, Win32, official Codex engine | Review third-party notices |
| WO-12 | Publish complete source | **NOT COMPLETE ON GITHUB** | Exact source tree committed and CI green |

## Critical release limitations
- A Linux-to-Windows cross-build does not validate rendering.
- A simulated App Server is not a live Codex Desktop interoperability test.
- Managed engine absence prevents sending until it is installed.
- Sending to a concurrently active Desktop session can conflict.
- The app cannot guarantee scheduled delivery when Windows is off or asleep.
- An interrupted send must not be silently replayed.
- Download integrity alone does not establish Authenticode publisher identity.
- The current updater replaces the GUI EXE; engine and updater need full package replacement.

## Final acceptance sequence
1. Commit the complete source and generated icon resources.
2. Run `go test ./...` and Windows CI.
3. Test a clean Windows 11 host without CLI, browser runtime, Python or Node.
4. Confirm sessions, titles, transcript and search with a real Desktop profile.
5. Send a harmless live Codex turn, then test active-thread conflicts.
6. Test one scheduled send, cancellation, interruption and duplicate prevention.
7. Publish a signed prerelease and verify update, rollback and health checks.
8. Release stable only after all criteria pass.
