# Third-party components and sources

- **Go standard library**: Official open-source Go runtime. The application uses
  Go for storage, networking, JSON, archive validation, and Windows interoperability.
  License: https://go.dev/LICENSE
- **Microsoft Win32 APIs and Windows common controls**: Operating-system APIs.
  No browser engine, Qt runtime, or Electron runtime is embedded.
  API reference: https://learn.microsoft.com/windows/win32/api/
- **OpenAI Codex App Server**: The separate Codex engine is obtained from OpenAI's
  official GitHub release and verified with the release SHA-256 digest.
  License and notices belong to the upstream release.
  https://github.com/openai/codex
- **GitHub Actions**: `actions/checkout`, `actions/setup-go`, and
  `actions/upload-artifact` are used for automated validation and builds.
- **softprops/action-gh-release**: Used by the release workflow to publish
  GitHub release assets. Upstream: https://github.com/softprops/action-gh-release

No downloaded upstream executable is committed to this source repository.
The release archive includes the verified Codex runtime under `engine/`.
Codex Queue Studio is an independent companion application, not an official
OpenAI or Microsoft product. It must not imitate their trademarks.
