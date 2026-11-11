# Codex Queue Studio v3 — Work orders

The statuses below describe implemented source code, not Windows UI acceptance.
An item is not fully verified until its acceptance test passes on Windows 11.

| ID | Requirement | Implementation | Verification | Remaining |
|---|---|---|---|---|
| WO-01 | Native UI without browser/Qt/Electron | Go Win32 native HWND controls and GDI drawing | Windows cross-build PASS | Windows UI smoke test |
| WO-02 | Windows 11 frame actions | WS_OVERLAPPEDWINDOW; window close to tray; single instance | Cross-build PASS | DPI and screen-reader tests |
| WO-03 | Theme contrast | Separate background, foreground, accent and field colors | Palette math inspected | Test high-contrast mode on Windows |
| WO-04 | Compact session explorer | LVS_OWNERDATA virtualized list, search, two-pane history viewer | Cross-build PASS | UI visual test with thousands of sessions |
| WO-05 | Guardian review title problem | Ignore generated review titles; prefer first useful user message | Unit test PASS | Test real affected sessions |
| WO-06 | Codex Desktop/CLI session discovery | JSONL parser; App Server thread/list optional merge | Unit + mock App Server PASS | Verify on live Desktop installation |
| WO-07 | Scheduled messaging without installed CLI | Managed official engine; app-server thread/resume and turn/start | Mock App Server PASS | Live Codex auth and turn test |
| WO-08 | Robust reservation lifecycle | DPAPI; atomic write; crash state; explicit migration approval | Unit test PASS | Forced-exit Windows test |
| WO-09 | Auto updates | GitHub release metadata; hash verification; staged backup and updater | Cross-build PASS | Signed release and live update test |
| WO-10 | App icon | Custom multi-resolution ICO plus PE resource group and manifest | PE resource inspection PASS | Windows Explorer and taskbar visual test |
| WO-11 | OSS and maintainability | Go standard library; official open-source Codex engine; GitHub CI | Manifest and license added | Dependency license review |
| WO-12 | GitHub source distribution | Source tree and CI/release workflows | Pending GitHub commit verification | Verify commits and Actions |

## Release gate

Do not publish a `stable` release until all of the following pass on Windows 11:

1. Start the EXE on a clean Windows 11 user profile with no Python, Node, or Codex CLI.
2. Verify Windows minimize, maximize, restore, close-to-tray, reopen, and exit.
3. Verify white text on dark mode and dark text on light mode in every control.
4. Verify at least 20 session rows are visible in a 1350 × 870 window.
5. Verify a genuine Codex Desktop thread title and transcript, including a previously mislabeled `Guardian review` session.
6. Verify the managed Codex engine is found inside the release archive.
7. Send a harmless message through the live Codex App Server.
8. Test conflicts with an active desktop session and confirm they do not cause silent duplicates.
9. Create a future reservation, close to tray, and verify that it executes once.
10. Simulate a mid-send failure and confirm automatic replay is blocked.
11. Publish a signed `v3.0.1` update and verify update, restart, and recovery.
12. Run GitHub Actions and review all failures and warnings.
