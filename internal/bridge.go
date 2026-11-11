package internal

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

type Bridge struct {
	Executable string
	Prefix     []string
	Label      string
}

func pathExists(p string) bool { info, err := os.Stat(p); return err == nil && !info.IsDir() }
func candidateExe(candidate string) (Bridge, bool) {
	candidate = strings.TrimSpace(strings.Trim(candidate, "\""))
	if candidate == "" {
		return Bridge{}, false
	}
	if strings.EqualFold(filepath.Ext(candidate), ".cmd") || strings.EqualFold(filepath.Ext(candidate), ".bat") {
		return Bridge{}, false
	}
	if filepath.Ext(candidate) == "" {
		if p, err := exec.LookPath(candidate); err == nil {
			candidate = p
		}
	}
	if !pathExists(candidate) {
		return Bridge{}, false
	}
	return Bridge{Executable: candidate, Label: candidate}, true
}
func DetectBridge(custom string) (Bridge, error) {
	if custom != "" {
		if b, ok := candidateExe(custom); ok {
			return b, nil
		}
		return Bridge{}, fmt.Errorf("설정한 실행 파일이 없거나 CMD/BAT입니다: %s", custom)
	}
	for _, name := range []string{"codex.exe", "codex"} {
		if p, err := exec.LookPath(name); err == nil {
			if b, ok := candidateExe(p); ok {
				return b, nil
			}
		}
	}
	home, _ := os.UserHomeDir()
	appdata := os.Getenv("APPDATA")
	local := os.Getenv("LOCALAPPDATA")
	for _, path := range []string{filepath.Join(home, ".cargo", "bin", "codex.exe"), filepath.Join(local, "Microsoft", "WinGet", "Links", "codex.exe"), filepath.Join(local, "Programs", "Codex", "codex.exe"), filepath.Join(local, "Programs", "Codex", "resources", "codex.exe"), filepath.Join(local, "Microsoft", "WindowsApps", "codex.exe")} {
		if b, ok := candidateExe(path); ok {
			return b, nil
		}
	}
	// npm creates a codex.cmd wrapper. Run its JavaScript entry with node.exe to avoid shell injection.
	npmJS := filepath.Join(appdata, "npm", "node_modules", "@openai", "codex", "bin", "codex.js")
	if pathExists(npmJS) {
		if node, err := exec.LookPath("node.exe"); err == nil {
			return Bridge{Executable: node, Prefix: []string{npmJS}, Label: "Node + @openai/codex"}, nil
		}
	}
	if runtime.GOOS != "windows" {
		if b, ok := candidateExe("codex"); ok {
			return b, nil
		}
	}
	return Bridge{}, errors.New("Codex CLI 실행 파일을 찾지 못했습니다. 세션 조회는 가능하지만 메시지 전송에는 Codex CLI가 필요합니다.")
}
func (b Bridge) Send(ctx context.Context, sessionID, message, mode string) (string, error) {
	if b.Executable == "" {
		return "", errors.New("Codex 실행 파일이 지정되지 않았습니다")
	}
	if !ValidSessionID(sessionID) {
		return "", errors.New("세션 ID 형식이 올바르지 않습니다")
	}
	if strings.TrimSpace(message) == "" {
		return "", errors.New("메시지가 비어 있습니다")
	}
	args := append([]string(nil), b.Prefix...)
	args = append(args, "queue", "--thread", sessionID, "--message", message)
	if mode == "queue-resume" {
		args = append(args, "--resume")
	} else if mode != "queue" {
		return "", errors.New("지원하지 않는 전송 방식")
	}
	cmd := exec.CommandContext(ctx, b.Executable, args...)
	configureHiddenProcess(cmd)
	cmd.Env = append(os.Environ(), "NO_COLOR=1")
	var stdout, stderr limitedBuffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	start := time.Now()
	err := cmd.Run()
	output := strings.TrimSpace(stdout.String() + "\n" + stderr.String())
	output = clipLine(output, 800)
	if err != nil {
		return output, fmt.Errorf("Codex 전송 실패 (%s): %w", time.Since(start).Round(time.Millisecond), err)
	}
	return output, nil
}

type limitedBuffer struct{ b bytes.Buffer }

func (w *limitedBuffer) Write(p []byte) (int, error) {
	n := len(p)
	space := 32768 - w.b.Len()
	if space > 0 {
		if len(p) > space {
			p = p[:space]
		}
		_, _ = w.b.Write(p)
	}
	return n, nil
}
func (w *limitedBuffer) String() string { return w.b.String() }
func ValidSessionID(id string) bool {
	if len(id) != 36 {
		return false
	}
	for i, c := range id {
		if i == 8 || i == 13 || i == 18 || i == 23 {
			if c != '-' {
				return false
			}
			continue
		}
		if !strings.ContainsRune("0123456789abcdefABCDEF", c) {
			return false
		}
	}
	return true
}
