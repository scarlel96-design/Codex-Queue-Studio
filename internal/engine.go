package internal

import (
	"archive/zip"
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const EngineReleaseAPI = "https://api.github.com/repos/openai/codex/releases/latest"
const MaxEngineDownload = 200 << 20
const MaxEngineUnpacked = 550 << 20

type releaseAsset struct {
	Name     string `json:"name"`
	Download string `json:"browser_download_url"`
	Size     int64  `json:"size"`
	Digest   string `json:"digest"`
}
type codexRelease struct {
	Tag    string         `json:"tag_name"`
	Assets []releaseAsset `json:"assets"`
}

func HTTPClient() *http.Client {
	return &http.Client{Timeout: 3 * time.Minute, CheckRedirect: func(req *http.Request, via []*http.Request) error {
		if req.URL.Scheme != "https" {
			return errors.New("HTTPS가 아닌 리디렉션 차단")
		}
		if len(via) > 8 {
			return errors.New("리디렉션 횟수 초과")
		}
		return nil
	}}
}
func downloadEngine(ctx context.Context, client *http.Client, releaseURL, targetDir string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", releaseURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "CodexQueueStudio/3.0")
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return "", fmt.Errorf("릴리스 조회 HTTP %d", resp.StatusCode)
	}
	var rel codexRelease
	if err = json.NewDecoder(io.LimitReader(resp.Body, 2<<20)).Decode(&rel); err != nil {
		return "", err
	}
	var asset *releaseAsset
	for i := range rel.Assets {
		if rel.Assets[i].Name == "codex-x86_64-pc-windows-msvc.exe.zip" {
			asset = &rel.Assets[i]
			break
		}
	}
	if asset == nil {
		return "", errors.New("공식 Codex Windows x64 엔진 ZIP을 찾지 못했습니다")
	}
	if asset.Size <= 0 || asset.Size > MaxEngineDownload {
		return "", errors.New("엔진 크기가 허용 범위를 벗어났습니다")
	}
	if !strings.HasPrefix(asset.Digest, "sha256:") || len(asset.Digest) != 71 {
		return "", errors.New("공식 릴리스에 SHA-256 검증 정보가 없습니다")
	}
	if !strings.HasPrefix(asset.Download, "https://github.com/openai/codex/releases/download/") {
		return "", errors.New("공식 GitHub 다운로드 URL이 아닙니다")
	}
	if err = os.MkdirAll(targetDir, 0700); err != nil {
		return "", err
	}
	r, err := http.NewRequestWithContext(ctx, "GET", asset.Download, nil)
	if err != nil {
		return "", err
	}
	r.Header.Set("User-Agent", "CodexQueueStudio/3.0")
	dl, err := client.Do(r)
	if err != nil {
		return "", err
	}
	defer dl.Body.Close()
	if dl.StatusCode != 200 {
		return "", fmt.Errorf("다운로드 HTTP %d", dl.StatusCode)
	}
	tmp, err := os.CreateTemp(targetDir, "engine-*.zip")
	if err != nil {
		return "", err
	}
	defer os.Remove(tmp.Name())
	defer tmp.Close()
	hash := sha256.New()
	n, err := io.Copy(io.MultiWriter(tmp, hash), io.LimitReader(dl.Body, MaxEngineDownload+1))
	if err != nil {
		return "", err
	}
	if n > MaxEngineDownload || n != asset.Size {
		return "", errors.New("다운로드 크기가 릴리스 정보와 다릅니다")
	}
	if !strings.EqualFold(hex.EncodeToString(hash.Sum(nil)), strings.TrimPrefix(asset.Digest, "sha256:")) {
		return "", errors.New("Codex 엔진 SHA-256 무결성 검사 실패")
	}
	zi, err := zip.NewReader(tmp, n)
	if err != nil {
		return "", err
	}
	var executable *zip.File
	for _, zf := range zi.File {
		if strings.EqualFold(filepath.Base(zf.Name), "codex-x86_64-pc-windows-msvc.exe") || strings.EqualFold(filepath.Base(zf.Name), "codex.exe") {
			if zf.UncompressedSize64 > MaxEngineUnpacked {
				return "", errors.New("압축 해제 크기 초과")
			}
			executable = zf
			break
		}
	}
	if executable == nil {
		return "", errors.New("공식 ZIP에 Codex 실행 파일이 없습니다")
	}
	src, err := executable.Open()
	if err != nil {
		return "", err
	}
	defer src.Close()
	staging, err := os.CreateTemp(targetDir, "codex-*.exe")
	if err != nil {
		return "", err
	}
	name := staging.Name()
	defer os.Remove(name)
	count, err := io.Copy(staging, io.LimitReader(src, MaxEngineUnpacked+1))
	if err != nil {
		staging.Close()
		return "", err
	}
	if count > MaxEngineUnpacked {
		staging.Close()
		return "", errors.New("엔진 파일이 너무 큽니다")
	}
	if err = staging.Sync(); err != nil {
		staging.Close()
		return "", err
	}
	if err = staging.Close(); err != nil {
		return "", err
	}
	dest := filepath.Join(targetDir, "codex.exe")
	if _, err = os.Stat(dest); err == nil {
		return "", errors.New("이미 설치된 엔진은 실행 중일 수 있습니다. 덮어쓰지 않습니다")
	}
	if err = os.Rename(name, dest); err != nil {
		return "", err
	}
	return rel.Tag, nil
}
func InstallEngine(ctx context.Context, targetDir string) (string, error) {
	return downloadEngine(ctx, HTTPClient(), EngineReleaseAPI, targetDir)
}

// AppServer uses Codex's documented JSON-RPC app-server transport.
// The binary can be a verified app-managed runtime; a separately installed CLI is not required.
type AppServer struct {
	Executable string
	CodexHome  string
}
type rpcReply struct {
	ID     json.RawMessage `json:"id"`
	Method string          `json:"method"`
	Params json.RawMessage `json:"params"`
	Result json.RawMessage `json:"result"`
	Error  *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}
type rpcProcess struct {
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	lines  chan rpcReply
	errors chan error
	stderr *limitedBuffer
}

func (s AppServer) connect(ctx context.Context) (*rpcProcess, error) {
	if !pathExists(s.Executable) {
		return nil, errors.New("Codex 실행 엔진이 없습니다. 설정에서 엔진을 설치하세요")
	}
	cmd := exec.CommandContext(ctx, s.Executable, "app-server")
	configureHiddenProcess(cmd)
	cmd.Env = append(os.Environ(), "NO_COLOR=1", "CODEX_HOME="+s.CodexHome)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	stderr := new(limitedBuffer)
	cmd.Stderr = stderr
	if err = cmd.Start(); err != nil {
		return nil, err
	}
	p := &rpcProcess{cmd: cmd, stdin: stdin, lines: make(chan rpcReply, 32), errors: make(chan error, 1), stderr: stderr}
	go func() {
		defer close(p.lines)
		scanner := bufio.NewScanner(stdout)
		scanner.Buffer(make([]byte, 64<<10), 16<<20)
		for scanner.Scan() {
			var msg rpcReply
			if json.Unmarshal(scanner.Bytes(), &msg) == nil {
				select {
				case p.lines <- msg:
				case <-ctx.Done():
					return
				}
			}
		}
		p.errors <- scanner.Err()
	}()
	return p, nil
}
func (p *rpcProcess) send(obj any) error {
	raw, err := json.Marshal(obj)
	if err != nil {
		return err
	}
	raw = append(raw, '\n')
	_, err = p.stdin.Write(raw)
	return err
}
func (p *rpcProcess) close() { _ = p.stdin.Close(); _ = p.cmd.Process.Kill(); _ = p.cmd.Wait() }
func (p *rpcProcess) await(ctx context.Context, id string) (json.RawMessage, error) {
	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case ev, ok := <-p.lines:
			if !ok {
				return nil, fmt.Errorf("Codex 프로세스가 종료되었습니다: %s", clipLine(p.stderr.String(), 300))
			}
			if string(ev.ID) == id {
				if ev.Error != nil {
					return nil, fmt.Errorf("Codex RPC %d: %s", ev.Error.Code, ev.Error.Message)
				}
				return ev.Result, nil
			}
			if ev.Method != "" && len(ev.ID) > 0 {
				_ = p.send(map[string]any{"id": json.RawMessage(ev.ID), "error": map[string]any{"code": -32601, "message": "Headless client does not support interactive approvals"}})
			}
		}
	}
}
func (p *rpcProcess) initialize(ctx context.Context) error {
	if err := p.send(map[string]any{"id": 1, "method": "initialize", "params": map[string]any{"clientInfo": map[string]any{"name": "codex_queue_studio", "title": "Codex Queue Studio", "version": "3.0.0"}}}); err != nil {
		return err
	}
	if _, err := p.await(ctx, "1"); err != nil {
		return err
	}
	return p.send(map[string]any{"method": "initialized", "params": map[string]any{}})
}
func (s AppServer) Send(ctx context.Context, id, message string) (string, error) {
	if !ValidSessionID(id) {
		return "", errors.New("잘못된 세션 UUID")
	}
	if strings.TrimSpace(message) == "" {
		return "", errors.New("메시지가 비어 있습니다")
	}
	p, err := s.connect(ctx)
	if err != nil {
		return "", err
	}
	defer p.close()
	if err = p.initialize(ctx); err != nil {
		return "", err
	}
	if err = p.send(map[string]any{"id": 2, "method": "thread/resume", "params": map[string]any{"threadId": id, "excludeTurns": true}}); err != nil {
		return "", err
	}
	if _, err = p.await(ctx, "2"); err != nil {
		return "", fmt.Errorf("세션 재개 실패: %w", err)
	}
	if err = p.send(map[string]any{"id": 3, "method": "turn/start", "params": map[string]any{"threadId": id, "input": []map[string]string{{"type": "text", "text": message}}}}); err != nil {
		return "", err
	}
	// A successful turn/start response confirms acceptance. Wait for terminal status.
	started := false
	for {
		select {
		case <-ctx.Done():
			if started {
				return "accepted; completion unknown", fmt.Errorf("응답 대기 중 연결 종료: %w", ctx.Err())
			}
			return "", ctx.Err()
		case ev, ok := <-p.lines:
			if !ok {
				if started {
					return "accepted; completion unknown", fmt.Errorf("Codex 서버가 요청 수락 후 종료되었습니다: %s", clipLine(p.stderr.String(), 300))
				}
				return "", fmt.Errorf("Codex 서버 종료: %s", clipLine(p.stderr.String(), 300))
			}
			if string(ev.ID) == "3" {
				if ev.Error != nil {
					return "", fmt.Errorf("turn/start 거절: %s", ev.Error.Message)
				}
				started = true
			}
			if ev.Method == "turn/completed" {
				var obj struct {
					Turn struct {
						ID     string `json:"id"`
						Status string `json:"status"`
					} `json:"turn"`
				}
				_ = json.Unmarshal(ev.Params, &obj)
				if obj.Turn.Status == "completed" {
					return "turn/completed: " + obj.Turn.ID, nil
				}
				return "", fmt.Errorf("Codex 턴 종료 상태: %s", obj.Turn.Status)
			}
			if len(ev.ID) > 0 && ev.Method != "" {
				_ = p.send(map[string]any{"id": json.RawMessage(ev.ID), "error": map[string]any{"code": -32601, "message": "No interactive approval during scheduled execution"}})
			}
		}
	}
}

func (s AppServer) List(ctx context.Context, limit int) ([]Session, error) {
	if limit <= 0 {
		limit = 1000
	}
	p, err := s.connect(ctx)
	if err != nil {
		return nil, err
	}
	defer p.close()
	if err = p.initialize(ctx); err != nil {
		return nil, err
	}
	var result []Session
	var cursor *string
	for page := 0; page < 20 && len(result) < limit; page++ {
		args := map[string]any{"limit": 100}
		if cursor != nil {
			args["cursor"] = *cursor
		}
		id := fmt.Sprintf("%d", 10+page)
		if err = p.send(map[string]any{"id": 10 + page, "method": "thread/list", "params": args}); err != nil {
			return result, err
		}
		raw, err := p.await(ctx, id)
		if err != nil {
			return result, err
		}
		var parsed struct {
			Data []struct {
				ID        string          `json:"id"`
				Name      *string         `json:"name"`
				Preview   string          `json:"preview"`
				CWD       string          `json:"cwd"`
				UpdatedAt int64           `json:"updatedAt"`
				Source    json.RawMessage `json:"source"`
			} `json:"data"`
			NextCursor *string `json:"nextCursor"`
		}
		if err = json.Unmarshal(raw, &parsed); err != nil {
			return result, err
		}
		for _, t := range parsed.Data {
			if !ValidSessionID(t.ID) {
				continue
			}
			name := t.Preview
			source := "preview"
			if t.Name != nil && !genericTitle(*t.Name) {
				name = *t.Name
				source = "app-server"
			} else if genericTitle(name) {
				name = "제목 없음"
			}
			tm := time.Unix(t.UpdatedAt, 0)
			result = append(result, Session{ID: t.ID, Title: clipLine(name, 100), Project: filepath.Base(t.CWD), Path: t.CWD, Source: "App Server", UpdatedAt: tm, TitleSource: source})
		}
		if parsed.NextCursor == nil || *parsed.NextCursor == "" {
			break
		}
		cursor = parsed.NextCursor
	}
	return result, nil
}
func MergeSessions(local, remote []Session) []Session {
	byID := make(map[string]Session, len(local)+len(remote))
	for _, s := range local {
		byID[s.ID] = s
	}
	for _, r := range remote {
		if l, ok := byID[r.ID]; ok {
			if (genericTitle(l.Title) || l.TitleSource == "unresolved") && !genericTitle(r.Title) {
				l.Title = r.Title
				l.TitleSource = r.TitleSource
			}
			if l.Path == "" {
				l.Path = r.Path
			}
			if l.Source == "로컬" {
				l.Source = r.Source
			}
			byID[r.ID] = l
		} else {
			byID[r.ID] = r
		}
	}
	result := make([]Session, 0, len(byID))
	for _, s := range byID {
		result = append(result, s)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].UpdatedAt.After(result[j].UpdatedAt) })
	return result
}
