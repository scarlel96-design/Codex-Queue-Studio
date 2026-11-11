# Verification report — 2026-11-11

## Executed checks

| Check | Result | Environment |
|---|---|---|
| `go test ./...` | PASS; 12 tests | Linux amd64 / Go 1.23.2 |
| `GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build ... .` | PASS | Linux cross-compiler |
| GitHub Actions Windows + Ubuntu CI | PASS on commit `8979fe21505968e15df920f4a79c0f77161d1af0` | Hosted CI, tests + native Windows build |
| `GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build ... ./cmd/updater` | PASS | Linux cross-compiler |
| `.rsrc` icon and manifest resource structure | PASS, COFF resource inspection | GNU objdump |
| Source-only session parsing, `Guardian review` fallback | PASS, fixture | Go tests |
| App Server `thread/list` and `thread/resume` + `turn/start` | PASS, *simulated server only* | Go tests |
| State file encryption and crash recovery | PASS, unit test | Go tests |
| Windows 11 native window interaction | NOT RUN | Windows unavailable |
| Live Codex Desktop session and transcript | NOT RUN | User profile unavailable |
| Live Codex engine installation/download | NOT RUN | Internet and Windows unavailable |
| Live scheduled message send | NOT RUN | Real Codex unavailable |
| GitHub Actions | PASS on verified prior commit | Latest post-test run pending |
| Signed GitHub Release | NOT RUN | Needs real Windows acceptance |

The test count excludes Windows native UI smoke tests. A successful cross-build
proves compile compatibility, not correct rendering or runtime behavior.

## Reproduction

```powershell
go test -v ./...
go vet -unsafeptr=false ./...
go build -ldflags "-H windowsgui -s -w" -o CodexQueueStudio.exe .
go build -ldflags "-H windowsgui -s -w" -o CodexQueueUpdater.exe ./cmd/updater
```
