# v2 to v3 engineering audit

## Confirmed issues in the previous v2 source

| Severity | Issue | Evidence | v3 action |
|---|---|---|---|
| P0 | GUI used a browser application window and local HTTP server | v2 `main_windows.go` opened Edge/Chrome with `--app=` | Replaced with a native Win32 window and HWND controls |
| P0 | A missing Codex CLI prevented sending | v2 `DetectBridge` reported an error without a local executable | Added a managed, SHA-256-verified Codex runtime and App Server RPC |
| P1 | Session titles could all appear as `Guardian review` | v2 `ScanSessions` preferred any `session_index.jsonl` title over the user message | Reject generated review titles as labels; use meaningful user input |
| P1 | One session occupied a large card | v2 `ui` displayed multi-line item cards | Replaced cards with a virtual ListView, one row per session |
| P1 | Color contrasts differed between custom themes | v2 CSS theme colors were insufficiently isolated | Explicit per-theme Win32 GDI foreground and background colors |
| P1 | No verifiable release update path | v2 did not implement a GitHub release updater | Added manifest, SHA-256, updater helper, backup, and release CI |
| P1 | Duplicate send was possible when multiple executables used the same JSON state | v2 single-instance detection only protected its local HTTP port | Added a Windows named mutex for GUI instances |
| P1 | A v2 pending `queue` task could change behavior under v3 | Different submission methods have different semantics | Pause old pending tasks until manual reactivation |
| P2 | App name, title, and taskbar icon were inconsistent | v2 did not embed an icon resource | Added ICO, RT_GROUP_ICON resources, and a manifest |

## New architecture

- UI: pure Win32, common controls v6, GDI drawing and DWM caption attributes.
- Storage: existing JSON state schema, updated to v3, DPAPI message encryption.
- Session index: local rollout JSONL metadata plus App Server `thread/list` merge.
- Session history: bounded, lazy rollout parsing with duplicate-message suppression.
- Scheduler: serialized, durable transitions `pending → sending → completed|failed|uncertain`.
- Engine: a separate official Codex runtime invoked with `app-server` over JSON-RPC stdio.
- Updater: stable GitHub release, verified ZIP asset, separate Windows updater helper.

## Unresolved risks and limitations

**P0 — No live Windows verification.** The application was cross-compiled on
Linux. The native UI, tray behavior, Windows DPAPI, App Server authentication,
and actual Codex Desktop integration require a Windows acceptance run.

**P0 — Active thread conflicts.** An App Server process cannot guarantee that a
turn is safe to append while another Codex Desktop process actively edits the
same thread. The application does not inject text into the Desktop textbox.
Test concurrent sessions before enabling unattended work.

**P1 — Headless approval.** The app rejects interactive approval requests.
An unattended turn requiring approval can fail or remain incomplete.

**P1 — Update trust and code signing.** The update and engine ZIP downloads use
GitHub TLS and the GitHub release asset SHA-256 digest. The application does
not currently verify a separate Authenticode publisher certificate. SmartScreen
may warn about unsigned builds. Add release signing before general distribution.

**P1 — Interrupted updates.** The helper preserves the previous EXE as a
`.backup` file. It attempts restoration when file replacement fails. Automatic
health-check rollback after a successful restart is not implemented.

**P1 — Engine and helper updates.** The self-updater swaps only
`CodexQueueStudio.exe`. A full release ZIP reinstall is required when the
bundled engine or `CodexQueueUpdater.exe` changes.

**P2 — Cloud-only threads.** JSONL scanning cannot find conversations that have
no local state. App Server results can include additional threads, subject to
authentication, source filtering, and server-side availability.

**P2 — Native theming.** Standard common-control scrollbars and header surfaces
still use Windows theme defaults. Validate all controls in the user's
high-contrast theme and with Windows text scaling.

**P2 — Performance limits.** Local scanning is bounded at 30,000 files, and the
virtual list shows the first 5,000 filtered items. Bounded transcript views
retain recent messages instead of loading entire multi-gigabyte logs.

**P2 — No task scheduler service.** Jobs run when the application is active in
the user session. Sleep, shutdown, logout, and network outages can prevent
on-time execution. Do not claim guaranteed delivery.

## Security review checklist

- No hard-coded access tokens or passwords: source inspection complete.
- Local storage encryption: Windows DPAPI used; Linux tests are stand-ins.
- Network whitelist: official GitHub endpoints for runtime and update assets.
- Verification: download size and SHA-256 must match release asset metadata.
- Elevation: manifest requests standard user privileges, not administrator.
- Runtime sandbox: Codex enforces its own configuration and approval policy.
- Data preservation: previous v2 `state.json` is not erased by migration.
- Rollback: old executable is kept as a backup by the update helper.
