# Codex Queue Studio 3.4.0 — Windows native companion

Independent community application. Not an official OpenAI product.

## What this patch changes

- Only Codex sessions appear in the explorer. Explicit ChatGPT, Atlas, unknown-source, and child-agent sessions are excluded.
- A manually assigned session name remains unchanged. An App Server authoritative name takes precedence over a stale local index.
- If a session has no meaningful stored name, the first relevant user message is shown as a **derived title**.
- Codex-only source classification is based on session metadata. Search text never controls session eligibility.
- Virtualized sessions occupy one line each. The list shows the name, project, origin, and modification time.
- The executable embeds a new Windows icon. The native window retains Windows 11 title bar buttons.
- GitHub Releases must include exactly two assets: the complete application ZIP and an independent update ZIP.
- The new updater verifies the GitHub release SHA-256 and the update manifest before replacing files.
- The helper updates the main executable and the updater itself. Failed replacements attempt a rollback.

## Requirements and launch

1. Unpack the complete Windows x64 release ZIP to a writable folder.
2. Run `CodexQueueStudio.exe` directly. Python, browser runtime, and BAT files are not required.
3. Use the Codex installation from the full official release or the managed engine installer.
4. Select a Codex session, enter a message, and schedule the submission.
5. Keep Windows awake and the app running for timed delivery.

Codex Desktop and this companion use separate runtime processes. An active Desktop session may conflict with an external `thread/resume` operation. Scheduled requests that need interactive approval are not automatically approved.

## Session title rules

1. Use a meaningful `thread.name` returned by Codex App Server.
2. Otherwise, use the latest meaningful `thread_name` in `session_index.jsonl`.
3. Otherwise, derive a title from the first relevant user message.
4. Otherwise, display `제목 미지정`.

Derived titles do not claim to be official generated names. Known `Guardian review` placeholders are not used as normal task names.

## Release contract

Tags use `vX.Y.Z`, where the version must match `AppVersion` in `core.go`.

| GitHub Releases file | Contents | Purpose |
|---|---|---|
| `CodexQueueStudio-Windows-x64.zip` | Main EXE, updater EXE, verified official Codex engine, icons, license, documentation | Complete installation |
| `CodexQueueStudio-update-Windows-x64.zip` | Main EXE, updater EXE, `update-manifest.json` with SHA-256 for both executables | Automatic update |

Both ZIP files are mandatory. The workflow checks their existence before publishing.

The app downloads only the update ZIP. It checks the GitHub asset's SHA-256 digest, ZIP member names, uncompressed sizes, release version, and per-file hashes. The updater runs from the verified staging directory after the app exits.

The old v3 updater only updated the main executable. Upgrading an existing v3 installation to this version can update the main executable but retain the old helper until the next release. For a clean v3.1 deployment, extract the complete ZIP.

## Build and test

```powershell
go test ./...
go vet -unsafeptr=false ./...
python assets/build_resources.py
go build -ldflags "-H windowsgui -s -w" -o CodexQueueStudio.exe .
go build -ldflags "-H windowsgui -s -w" -o CodexQueueUpdater.exe ./cmd/updater
```

The UI uses native Win32 and Windows Common Controls. The resources contain a multi-resolution application icon and Common Controls v6 manifest.

## Data and limitations

- Session records: `CODEX_HOME` or `%USERPROFILE%\.codex`.
- Local task data: `%LOCALAPPDATA%\CodexQueueStudio\state.json`.
- Messages: protected by Windows DPAPI for the current account.
- Unknown-source sessions: excluded. This may omit legitimate historical Codex threads without source metadata.
- Cloud-only ChatGPT conversations: not included.
- No Windows Task Scheduler service: no delivery while the computer is off, sleeping, or the user session is unavailable.
- No automatic approval of sensitive operations.
- Windows 11 UI, real Codex Desktop interaction, code signing and in-place rollback still require live acceptance testing.

See `docs/AUDIT.md` and `docs/VERIFICATION.md` for unresolved risks and test evidence.

## v3.2.1 Win32 UI and update hotfix

This preview fixes the UI hang diagnosis and adds native progress views for update discovery, downloading and replacement. The in-app update center now shows the GitHub Release body without inventing changes. The updater requires a readiness marker written from the new Win32 message loop and repeated `SendMessageTimeoutW` responsiveness checks. It attempts rollback if the new GUI fails.

The session and job persistence format is unchanged. Do not delete `state.json` when upgrading. Previous versions can be repaired with the full ZIP by replacing the two application EXE files while all processes are stopped.

Actual user hardware validation is required before any stable v1.0.0 release. Windows 11 typography and perceived visual quality have not been verified on the user's machine. Build and acceptance details are in `docs/RELEASE_3_2_1.md`.


## 3.3.0 Smart Relay preview

- A separate native Win32 Smart Relay workspace supports Codex completion monitoring and API conversation reviews.
- Use the chosen API Conversation ID (conv_...) or leave the field empty to create a new conversation.
- Existing ChatGPT web/app chat IDs cannot receive messages through the OpenAI API.
- Optional reply-to-Codex is disabled by default. User approval is checked by default.
- Each link runs at most three review cycles; uncertain network outcomes are not automatically retried.
- Set OPENAI_API_KEY before launching the app. OpenAI API billing is separate from ChatGPT plans.
- The session explorer now uses row-level refresh and does not force a complete repaint every four seconds.

Read docs/SMART_RELAY_PREVIEW.md before enabling automation. The feature remains preview until a real Codex Desktop end-to-end test succeeds.


## v3.4.0 native session explorer and provider adapters

- Use two explorer tabs: **Codex 작업** for real Codex threads and **GPT 채팅** for locally registered OpenAI API conversations.
- View the stored messages of a registered GPT API conversation through the official Conversations Items endpoint. Existing consumer ChatGPT chats are not available.
- Create an API conversation inside the GPT tab; it is saved locally for subsequent selection.
- In Smart Relay, select the Codex work session and GPT conversation from native combo boxes. No manual session UUID is required.
- Review models are fetched from your API account on demand. The model list excludes obvious audio, image, embedding, and moderation models but does not guarantee Responses API support.
- The native UI now has denser panels, improved dual-theme contrast, a 210 ms easing tab indicator, and a 175 ms relay window entrance. Windows reduced-motion settings are respected.
- The provider registry separates model, conversation, history and review adapters for later Gemini, Claude and Grok integrations. Those provider adapters are not yet implemented.
- The Windows smoke test includes tab switching and Smart Relay control creation.

Consult [v3.4 architecture and acceptance criteria](docs/ARCHITECTURE_3_4.md) and [v3.4 release notes](docs/RELEASE_3_4.md).
