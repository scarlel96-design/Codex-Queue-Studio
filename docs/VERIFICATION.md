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
