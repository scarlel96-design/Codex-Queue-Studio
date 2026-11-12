package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"codexqueuestudiov3/internal"
)

const AppVersion = "3.2.1"
const repoReleases = "https://api.github.com/repos/scarlel96-design/Codex-Queue-Studio/releases?per_page=40"

type Core struct {
	root     string
	store    *internal.Store
	mu       sync.Mutex
	sendMu   sync.Mutex
	sessions []internal.Session
	status   string
	update   *UpdateInfo
	stop     chan struct{}
}
type UpdateInfo struct {
	Notes       string
	Name        string
	PublishedAt string
	Prerelease  bool
	Tag         string
	URL         string
	Digest      string
	Size        int64
}
type updateRelease struct {
	Name        string `json:"name"`
	Body        string `json:"body"`
	PublishedAt string `json:"published_at"`
	Draft       bool   `json:"draft"`
	Prerelease  bool   `json:"prerelease"`
	Tag         string `json:"tag_name"`
	Assets      []struct {
		Name   string `json:"name"`
		URL    string `json:"browser_download_url"`
		Digest string `json:"digest"`
		Size   int64  `json:"size"`
	} `json:"assets"`
}

func defaultHome() string {
	if v := os.Getenv("CODEX_HOME"); v != "" {
		return v
	}
	h, _ := os.UserHomeDir()
	return filepath.Join(h, ".codex")
}
func appData() string {
	if v := os.Getenv("LOCALAPPDATA"); v != "" {
		return filepath.Join(v, "CodexQueueStudio")
	}
	h, _ := os.UserConfigDir()
	return filepath.Join(h, "CodexQueueStudio")
}
func NewCore(root string) (*Core, error) {
	store, err := internal.NewStore(filepath.Join(root, "state.json"), internal.Settings{CodexHome: defaultHome(), Theme: "system"})
	if err != nil {
		return nil, err
	}
	snapshot := store.Snapshot()
	if snapshot.Version < 3 {
		err = store.Update(func(st *internal.State) error {
			for i := range st.Jobs {
				j := &st.Jobs[i]
				if j.State == "pending" && j.Mode != "app-server" {
					j.State = "migration-required"
					j.Error = "v2 예약의 전송 방식이 변경되었습니다. 사용자가 다시 승인해야 합니다."
					internal.AddEvent(st, "migration", j.ID, "v2 CLI 예약을 안전하게 일시 중지했습니다")
				}
			}
			st.Version = 3
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	c := &Core{root: root, store: store, stop: make(chan struct{}), status: "준비 완료"}
	return c, nil
}
func (c *Core) EnginePath() string {
	if exe, err := os.Executable(); err == nil {
		bundled := filepath.Join(filepath.Dir(exe), "engine", "codex.exe")
		if st, err := os.Stat(bundled); err == nil && !st.IsDir() {
			return bundled
		}
	}
	return filepath.Join(c.root, "engine", "codex.exe")
}
func (c *Core) Server() internal.AppServer {
	return internal.AppServer{Executable: c.EnginePath(), CodexHome: c.store.Snapshot().Settings.CodexHome}
}
func (c *Core) EnginePresent() bool {
	st, err := os.Stat(c.EnginePath())
	return err == nil && !st.IsDir()
}
func (c *Core) Status() string     { c.mu.Lock(); defer c.mu.Unlock(); return c.status }
func (c *Core) setStatus(v string) { c.mu.Lock(); c.status = v; c.mu.Unlock() }
func (c *Core) Snapshot() []internal.Session {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]internal.Session(nil), c.sessions...)
}
func (c *Core) RefreshLocal() error {
	home := c.store.Snapshot().Settings.CodexHome
	found, err := internal.ScanSessions(home, 30000)
	if err != nil {
		c.setStatus("로컬 세션 조회 실패: " + err.Error())
		return err
	}
	c.mu.Lock()
	c.sessions = found
	c.mu.Unlock()
	c.setStatus(fmt.Sprintf("로컬 세션 %d개 조회", len(found)))
	return nil
}
func (c *Core) MergeRemote(ctx context.Context) error {
	if !c.EnginePresent() {
		return nil
	}
	rpcCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	remote, err := c.Server().List(rpcCtx, 1500)
	if err != nil {
		c.setStatus("로컬 목록 사용 중 · App Server 조회 실패: " + err.Error())
		return err
	}
	c.mu.Lock()
	c.sessions = internal.MergeSessions(c.sessions, remote)
	count := len(c.sessions)
	c.mu.Unlock()
	c.setStatus(fmt.Sprintf("통합 세션 %d개 조회", count))
	return nil
}
func (c *Core) Refresh(ctx context.Context) error {
	if err := c.RefreshLocal(); err != nil {
		return err
	}
	_ = c.MergeRemote(ctx) // Keep local sessions available if server authentication fails.
	return nil
}
func (c *Core) Schedule(id, title, message string, when time.Time) (string, error) {
	if !internal.ValidSessionID(id) {
		return "", errors.New("세션을 선택하세요")
	}
	if strings.TrimSpace(message) == "" || len([]rune(message)) > 12000 {
		return "", errors.New("메시지 길이는 1~12,000자여야 합니다")
	}
	if when.Before(time.Now().Add(-2 * time.Second)) {
		return "", errors.New("과거 시각으로 예약할 수 없습니다")
	}
	exists := false
	for _, s := range c.Snapshot() {
		if s.ID == id && !s.Archived {
			exists = true
			break
		}
	}
	if !exists {
		return "", errors.New("사용할 수 없는 세션입니다. 먼저 세션 목록을 새로고침하세요")
	}
	cipher, err := internal.SealMessage(message)
	if err != nil {
		return "", err
	}
	idNew := internal.NewID()
	now := time.Now()
	job := internal.Job{ID: idNew, SessionID: id, SessionTitle: title, Cipher: cipher, When: when, CreatedAt: now, UpdatedAt: now, Mode: "app-server", State: "pending"}
	err = c.store.Update(func(st *internal.State) error {
		st.Jobs = append(st.Jobs, job)
		internal.AddEvent(st, "created", idNew, "메시지 예약을 저장했습니다")
		return nil
	})
	return idNew, err
}
func (c *Core) Cancel(id string) error {
	return c.store.Update(func(st *internal.State) error {
		for i := range st.Jobs {
			if st.Jobs[i].ID == id {
				if st.Jobs[i].State != "pending" {
					return errors.New("대기 상태에서만 취소할 수 있습니다")
				}
				st.Jobs[i].State = "canceled"
				st.Jobs[i].UpdatedAt = time.Now()
				internal.AddEvent(st, "canceled", id, "예약 취소")
				return nil
			}
		}
		return errors.New("예약이 없습니다")
	})
}
func (c *Core) Reactivate(id string) error {
	return c.store.Update(func(st *internal.State) error {
		for i := range st.Jobs {
			j := &st.Jobs[i]
			if j.ID != id {
				continue
			}
			if j.State != "migration-required" && j.State != "failed" && j.State != "uncertain" {
				return errors.New("재예약할 수 없는 상태입니다")
			}
			j.Mode = "app-server"
			j.State = "pending"
			if !j.When.After(time.Now()) {
				j.When = time.Now().Add(5 * time.Second)
			}
			j.UpdatedAt = time.Now()
			j.Error = ""
			j.Detail = ""
			internal.AddEvent(st, "reactivated", id, "사용자가 전송 방식과 재예약을 확인했습니다")
			return nil
		}
		return errors.New("예약이 없습니다")
	})
}
func (c *Core) Pending() []internal.JobPublic {
	return internal.SortPublicJobs(c.store.Snapshot().Jobs)
}
func (c *Core) SendDue() {
	if !c.sendMu.TryLock() {
		return
	}
	defer c.sendMu.Unlock()
	for _, job := range c.store.Snapshot().Jobs {
		if job.State != "pending" || job.When.After(time.Now()) {
			continue
		}
		id := job.ID
		err := c.store.Update(func(st *internal.State) error {
			for i := range st.Jobs {
				if st.Jobs[i].ID == id && st.Jobs[i].State == "pending" {
					st.Jobs[i].State = "sending"
					st.Jobs[i].UpdatedAt = time.Now()
					internal.AddEvent(st, "sending", id, "Codex App Server에 실행 요청")
					return nil
				}
			}
			return errors.New("이미 처리된 작업")
		})
		if err != nil {
			continue
		}
		message, e := internal.OpenMessage(job.Cipher)
		detail := ""
		if e == nil {
			if !c.EnginePresent() {
				e = errors.New("내장 Codex 엔진이 설치되지 않았습니다. 설정에서 엔진을 설치하세요")
			} else {
				ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
				detail, e = c.Server().Send(ctx, job.SessionID, message)
				cancel()
			}
		}
		state := "completed"
		if e != nil {
			state = "failed"
			if strings.Contains(detail, "completion unknown") {
				state = "uncertain"
			}
		}
		reason := ""
		if e != nil {
			reason = e.Error()
		}
		_ = c.store.Update(func(st *internal.State) error {
			for i := range st.Jobs {
				if st.Jobs[i].ID == id && st.Jobs[i].State == "sending" {
					st.Jobs[i].State = state
					st.Jobs[i].Error = reason
					st.Jobs[i].Detail = detail
					st.Jobs[i].UpdatedAt = time.Now()
					internal.AddEvent(st, state, id, reason)
					break
				}
			}
			return nil
		})
		c.setStatus("예약 " + id[:8] + " : " + state)
	}
}
func (c *Core) Run() {
	go func() {
		t := time.NewTicker(2 * time.Second)
		defer t.Stop()
		c.SendDue()
		for {
			select {
			case <-t.C:
				c.SendDue()
			case <-c.stop:
				return
			}
		}
	}()
}
func (c *Core) Stop() { close(c.stop) }
func (c *Core) InstallEngine(ctx context.Context) error {
	if c.EnginePresent() {
		return errors.New("Codex 실행 엔진이 이미 설치되어 있습니다")
	}
	c.setStatus("공식 Codex 엔진 다운로드 및 무결성 검사 중")
	tag, err := internal.InstallEngine(ctx, filepath.Join(c.root, "engine"))
	if err != nil {
		c.setStatus("엔진 설치 실패: " + err.Error())
		return err
	}
	c.setStatus("Codex 엔진 설치 완료: " + tag)
	return nil
}
func newerTag(remote, current string) bool {
	parse := func(v string) ([3]int, bool) {
		var out [3]int
		v = strings.TrimPrefix(v, "v")
		parts := strings.Split(v, ".")
		if len(parts) != 3 {
			return out, false
		}
		for i, part := range parts {
			n, err := strconv.Atoi(part)
			if err != nil || n < 0 || n > 999999 {
				return out, false
			}
			out[i] = n
		}
		return out, true
	}
	a, ok := parse(remote)
	if !ok {
		return false
	}
	b, ok := parse(current)
	if !ok {
		return false
	}
	for i := 0; i < 3; i++ {
		if a[i] != b[i] {
			return a[i] > b[i]
		}
	}
	return false
}

// Test versions 3.x can upgrade to later previews or to the first stable 1.x.
// Stable versions never downgrade or select prereleases.
func updatePriority(tag string, preview bool, current string) int64 {
	var major, minor, patch int
	if n, e := fmt.Sscanf(strings.TrimPrefix(tag, "v"), "%d.%d.%d", &major, &minor, &patch); e != nil || n != 3 || major < 0 || minor < 0 || patch < 0 || minor > 99999 || patch > 99999 {
		return -1
	}
	if strings.TrimPrefix(current, "v") == strings.TrimPrefix(tag, "v") {
		return -1
	}
	if strings.HasPrefix(strings.TrimPrefix(current, "v"), "3.") {
		if !preview && major >= 1 && major < 3 {
			return 1000000000000 + int64(major)*10000000000 + int64(minor)*100000 + int64(patch)
		}
		if preview && major == 3 && newerTag(tag, current) {
			return int64(major)*10000000000 + int64(minor)*100000 + int64(patch)
		}
		return -1
	}
	if preview || !newerTag(tag, current) {
		return -1
	}
	return int64(major)*10000000000 + int64(minor)*100000 + int64(patch)
}
func chooseUpdate(releases []updateRelease, current string) (*UpdateInfo, error) {
	var selected *UpdateInfo
	rank := int64(-1)
	var issue error
	for _, rel := range releases {
		if rel.Draft {
			continue
		}
		priority := updatePriority(rel.Tag, rel.Prerelease, current)
		if priority < 0 {
			continue
		}
		full, update := 0, 0
		var url, digest string
		var size int64
		for _, asset := range rel.Assets {
			if asset.Name == "CodexQueueStudio-Windows-x64.zip" && asset.Size > 100000 {
				full++
			}
			if asset.Name == "CodexQueueStudio-update-Windows-x64.zip" {
				update++
				url = asset.URL
				digest = asset.Digest
				size = asset.Size
			}
		}
		valid := full == 1 && update == 1 && size > 100000 && size <= internal.MaxUpdateArchive && len(digest) == 71 && strings.HasPrefix(digest, "sha256:") && strings.HasPrefix(url, "https://github.com/scarlel96-design/Codex-Queue-Studio/releases/download/")
		if valid {
			for _, ch := range digest[7:] {
				if !strings.ContainsRune("0123456789abcdefABCDEF", ch) {
					valid = false
					break
				}
			}
		}
		if !valid {
			if issue == nil {
				issue = fmt.Errorf("%s: 전체 ZIP 또는 업데이트 ZIP의 무결성 정보가 잘못되었습니다", rel.Tag)
			}
			continue
		}
		if priority > rank {
			rank = priority
			selected = &UpdateInfo{Tag: rel.Tag, Name: rel.Name, Notes: releaseNotes(rel.Body), PublishedAt: rel.PublishedAt, URL: url, Digest: digest[7:], Size: size, Prerelease: rel.Prerelease}
		}
	}
	if selected == nil && issue != nil {
		return nil, issue
	}
	return selected, nil
}

// releaseNotes keeps real GitHub release notes and never fabricates a changelog.
func releaseNotes(body string) string {
	const limit = 6000
	b := strings.TrimSpace(body)
	if b == "" {
		return "릴리스에 변경 내역이 등록되지 않았습니다. GitHub에서 확인하세요."
	}
	r := []rune(b)
	if len(r) > limit {
		b = string(r[:limit]) + "\n…"
	}
	return b
}
func (c *Core) CheckUpdate(ctx context.Context) (*UpdateInfo, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", repoReleases, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "CodexQueueStudio/"+AppVersion)
	req.Header.Set("Accept", "application/vnd.github+json")
	resp, err := internal.HTTPClient().Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == 404 {
		return nil, nil
	}
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("GitHub 업데이트 조회 HTTP %d", resp.StatusCode)
	}
	var releases []updateRelease
	if err = json.NewDecoder(io.LimitReader(resp.Body, 3<<20)).Decode(&releases); err != nil {
		return nil, err
	}
	choice, err := chooseUpdate(releases, AppVersion)
	c.mu.Lock()
	c.update = choice
	c.mu.Unlock()
	return choice, err
}
func (c *Core) UpdateAvailable() *UpdateInfo { c.mu.Lock(); defer c.mu.Unlock(); return c.update }
func (c *Core) DownloadUpdate(ctx context.Context, upd UpdateInfo) (string, error) {
	return c.DownloadUpdateProgress(ctx, upd, nil)
}

// DownloadUpdateProgress streams bounded download progress without blocking the UI thread.
func (c *Core) DownloadUpdateProgress(ctx context.Context, upd UpdateInfo, progress func(stage string, percent int, received, total int64)) (string, error) {
	report := func(stage string, percent int, received int64) {
		if progress != nil {
			progress(stage, percent, received, upd.Size)
		}
	}
	report("다운로드 준비", 0, 0)
	req, err := http.NewRequestWithContext(ctx, "GET", upd.URL, nil)
	if err != nil {
		return "", err
	}
	resp, err := internal.HTTPClient().Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return "", fmt.Errorf("업데이트 다운로드 HTTP %d", resp.StatusCode)
	}
	var buffer bytes.Buffer
	block := make([]byte, 64*1024)
	var received int64
	for {
		count, readErr := resp.Body.Read(block)
		if count > 0 {
			received += int64(count)
			if received > int64(internal.MaxUpdateArchive) {
				return "", errors.New("업데이트 ZIP이 제한 크기를 초과했습니다")
			}
			if _, err = buffer.Write(block[:count]); err != nil {
				return "", err
			}
			pct := int(received * 72 / upd.Size)
			if pct > 72 {
				pct = 72
			}
			report("업데이트 다운로드 중", pct, received)
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			return "", readErr
		}
		if ctx.Err() != nil {
			return "", ctx.Err()
		}
	}
	raw := buffer.Bytes()
	report("패키지 무결성 검증", 79, received)
	if int64(len(raw)) != upd.Size {
		return "", errors.New("업데이트 크기 불일치")
	}
	sum := sha256.Sum256(raw)
	if !strings.EqualFold(hex.EncodeToString(sum[:]), upd.Digest) {
		return "", errors.New("업데이트 SHA256 검증 실패")
	}
	report("안전한 설치 준비", 88, int64(len(raw)))
	stage, err := internal.StageUpdateArchive(raw, filepath.Join(c.root, "updates"), upd.Tag)
	if err != nil {
		return "", err
	}
	report("검증 완료 · 설치 준비", 92, int64(len(raw)))
	return stage, nil
}

// CanInstallUpdate prevents an executable replacement during Codex delivery.
func (c *Core) CanInstallUpdate() error {
	if !c.sendMu.TryLock() {
		return errors.New("현재 Codex 메시지를 전송 중입니다. 전송 완료 후 업데이트하세요")
	}
	defer c.sendMu.Unlock()
	for _, job := range c.store.Snapshot().Jobs {
		if job.State == "sending" {
			return errors.New("전송 진행 상태가 남아 있습니다. 예약 관리에서 상태를 확인하세요")
		}
		if job.State == "pending" && !job.When.After(time.Now().Add(8*time.Second)) {
			return errors.New("곧 실행할 예약이 있습니다. 예약이 끝나면 업데이트하세요")
		}
	}
	return nil
}
