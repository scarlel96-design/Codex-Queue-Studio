# Verification — v3.1.0

## Verified locally

- Go unit tests: PASS.
- Go race detector on Linux: PASS.
- Go vet: PASS.
- Windows x64 GUI and updater cross-compilation: PASS.
- Codex versus ChatGPT session source fixtures: PASS.
- Exact title priority and Guardian fallback fixtures: PASS.
- Update archive tamper, version and unexpected-path rejection tests: PASS.
- Multi-resolution icon asset and Windows resource generation: PASS.

## Not verified

- Actual Windows 11 rendering and taskbar/tray icon.
- User's real Codex Desktop session provenance and names.
- Live scheduled turn submission and concurrent Desktop behavior.
- Windows DPAPI, two-binary replacement, rollback and SmartScreen.
- Successful GitHub Release publication with exactly two ZIP assets.

Release gate: Do not tag a production release until these high-risk acceptance checks pass.

## Windows UI thread hang regression

- A suspected cause of launch-time 'Not Responding' was identified: the Win32 window and GetMessageW loop were not explicitly pinned to one OS thread.
- The native entry point now calls runtime.LockOSThread before window and message-loop initialization.
- --ui-smoke-test creates the real Win32 GUI, skips network and scheduled sends, and exits from a WM_TIMER callback.
- Windows CI must complete this smoke test within 15 seconds. This still does not substitute for a user-PC test.
- --safe-mode skips automatic session loading while retaining scheduled delivery. Do not delete state.json to troubleshoot UI responsiveness.
