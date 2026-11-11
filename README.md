# Codex Queue Studio 3.1.0

Independent Windows companion application for Codex, not an official OpenAI product.

## New in 3.1.0

- Codex-only explorer excludes explicit ChatGPT/Atlas histories, subagents and unknown origins.
- Title priority: authoritative App Server name, local session_index.jsonl name, first relevant user prompt, then 제목 미지정.
- Native virtualized compact list shows title, project, source and modification time.
- Windows 11 Win32 window controls, fixed light/dark contrast, multi-resolution icon and tray support.
- GitHub Releases are built with exactly two files: complete version and separate update.
- Update ZIPs are verified using GitHub SHA-256 and a per-executable manifest.
- Staged update helper replaces the app and helper; backup restoration is attempted on error.

## Use

1. Extract the complete Windows x64 release ZIP into a writable folder.
2. Run CodexQueueStudio.exe, not a BAT launcher.
3. Choose an eligible Codex work session and inspect its history.
4. Enter a message and select immediate delivery or a scheduled date and time.
5. Keep the PC awake and the app running in the notification area.

The managed Codex engine is installed with the complete release. It can also be installed from settings. Codex Desktop and this application run as different processes. Concurrent editing of one thread may conflict.

## Session filtering and title rules

Session eligibility is based on Codex source metadata. The accepted sources include cli, vscode, app, appServer, exec and Codex-named custom sources. ChatGPT and Atlas chat histories, subagent threads and unknown source kinds are not shown. This strict policy can hide genuine historical Codex sessions with missing metadata.

A meaningful name from App Server takes precedence over the latest meaningful name in session_index.jsonl. If neither exists, a short user-message-based title is derived. This is not an official title or a rename. Generic Guardian review labels are not shown.

## Release contract

Two and only two ZIP assets are uploaded by a successful tagged release:

- CodexQueueStudio-Windows-x64.zip — complete installation, verified official Codex runtime, icon, README and licenses.
- CodexQueueStudio-update-Windows-x64.zip — GUI, updater, and update-manifest.json containing file SHA-256 values.

The version tag vX.Y.Z must match AppVersion in core.go. Missing or invalid assets cause the workflow to fail.

The update helper runs from a verified staging directory. It keeps executable backups and attempts rollback if replacement or restart fails. Users of old v3 installations must perform a one-time full-package upgrade because the original updater only replaced the main binary.

## Build from source

Install Go 1.23+ and Python 3. Install the build-only Pillow library (Pillow>=11,<13). Run assets/build_resources.py before compiling both Go executables with the Windows GUI linker option. Python/Pillow are not needed at runtime.

## Limitations

- Session files: CODEX_HOME or the user profile's .codex directory.
- Schedule state: LOCALAPPDATA/CodexQueueStudio/state.json, with Windows DPAPI encryption.
- Cloud-only ChatGPT conversations are not listed.
- No automatic security approvals are granted.
- No guaranteed delivery while sleeping, powered off, logged out or offline.
- Real Windows 11 rendering, live Codex Desktop work, Authenticode signing and update rollback still require acceptance tests.

See docs/CHANGES_3_1.md and docs/VERIFICATION.md.
