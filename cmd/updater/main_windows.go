//go:build windows

// Native updater validates a staged package and watches the new GUI's message pump.
package main

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
	"unsafe"

	"codexqueuestudiov3/internal"
)

func wait(pid int) error {
	kernel := syscall.NewLazyDLL("kernel32.dll")
	h, _, err := kernel.NewProc("OpenProcess").Call(0x00100000, 0, uintptr(pid))
	if h == 0 {
		if errors.Is(err, syscall.Errno(87)) {
			return nil
		} // Parent already exited.
		return fmt.Errorf("기존 프로세스 대기 오류: %w", err)
	}
	defer kernel.NewProc("CloseHandle").Call(h)
	result, _, callErr := kernel.NewProc("WaitForSingleObject").Call(h, 90000)
	if result != 0 {
		return fmt.Errorf("기존 프로그램 종료 대기 실패: %w", callErr)
	}
	return nil
}
func verifyStage(stage string) error {
	b, err := os.ReadFile(filepath.Join(stage, "update-manifest.json"))
	if err != nil {
		return err
	}
	var m internal.UpdateManifest
	if err = json.Unmarshal(b, &m); err != nil {
		return err
	}
	if m.Version == "" || len(m.Files) != 2 {
		return errors.New("업데이트 매니페스트 형식이 잘못되었습니다")
	}
	for _, name := range []string{"CodexQueueStudio.exe", "CodexQueueUpdater.exe"} {
		data, err := os.ReadFile(filepath.Join(stage, name))
		if err != nil {
			return err
		}
		if len(data) < 100000 || len(data) > internal.MaxUpdateBinary {
			return fmt.Errorf("파일 크기가 올바르지 않습니다: %s", name)
		}
		hash := sha256.Sum256(data)
		if !strings.EqualFold(hex.EncodeToString(hash[:]), m.Files[name]) {
			return fmt.Errorf("SHA-256 불일치: %s", name)
		}
	}
	return nil
}
func atomicWriteExecutable(directory, name string, data []byte) error {
	f, err := os.CreateTemp(directory, ".cqs-update-*.tmp")
	if err != nil {
		return err
	}
	temp := f.Name()
	defer os.Remove(temp)
	if err = f.Chmod(0700); err != nil {
		_ = f.Close()
		return err
	}
	if _, err = f.Write(data); err != nil {
		_ = f.Close()
		return err
	}
	if err = f.Sync(); err != nil {
		_ = f.Close()
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	return os.Rename(temp, filepath.Join(directory, name))
}
func newNonce() string {
	b := make([]byte, 18)
	if _, err := rand.Read(b); err != nil {
		return ""
	}
	return hex.EncodeToString(b)
}

func processAlive(pid int) bool {
	kernel := syscall.NewLazyDLL("kernel32.dll")
	h, _, _ := kernel.NewProc("OpenProcess").Call(0x00100000, 0, uintptr(pid))
	if h == 0 {
		return false
	}
	defer kernel.NewProc("CloseHandle").Call(h)
	code, _, _ := kernel.NewProc("WaitForSingleObject").Call(h, 0)
	return code != 0
}
func responsiveWindow(pid int) bool {
	user := syscall.NewLazyDLL("user32.dll")
	title, _ := syscall.UTF16PtrFromString("CodexQueueStudioNativeV3")
	hwnd, _, _ := user.NewProc("FindWindowW").Call(uintptr(unsafe.Pointer(title)), 0)
	if hwnd == 0 {
		return false
	}
	var found uint32
	user.NewProc("GetWindowThreadProcessId").Call(hwnd, uintptr(unsafe.Pointer(&found)))
	if found != uint32(pid) {
		return false
	}
	var result uintptr
	// SMTO_ABORTIFHUNG prevents an unresponsive child from blocking this updater.
	ok, _, _ := user.NewProc("SendMessageTimeoutW").Call(hwnd, 0, 0, 0, 0x0002, 1700, uintptr(unsafe.Pointer(&result)))
	return ok != 0
}
func awaitHealthyStart(proc *os.Process, marker, nonce string, report func(int, string), timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	observed := false
	successfulPings := 0
	firstPing := time.Time{}
	for time.Now().Before(deadline) {
		if !processAlive(proc.Pid) {
			return errors.New("새 버전이 시작 직후 종료되었습니다")
		}
		b, e := os.ReadFile(marker)
		if e == nil && string(b) == nonce+"\n"+strconv.Itoa(proc.Pid) {
			observed = true
		}
		if observed && responsiveWindow(proc.Pid) {
			successfulPings++
			if firstPing.IsZero() {
				firstPing = time.Now()
			}
			if successfulPings >= 4 && time.Since(firstPing) >= 5*time.Second {
				return nil
			}
		} else {
			successfulPings = 0
			firstPing = time.Time{}
		}
		if report != nil {
			report(91+min(successfulPings, 4)*2, "새 버전 창의 응답 상태 확인 중")
		}
		time.Sleep(500 * time.Millisecond)
	}
	if !observed {
		return errors.New("새 버전이 UI 준비 신호를 보내지 않았습니다")
	}
	return errors.New("새 버전이 응답하지 않습니다. 이전 실행 파일을 복구합니다")
}

func install(pid int, target, stage string, report func(int, string)) error {
	step := func(pct int, status string) {
		if report != nil {
			report(pct, status)
		}
	}
	if pid <= 0 || !filepath.IsAbs(target) || !filepath.IsAbs(stage) {
		return errors.New("업데이트 경로 또는 PID가 올바르지 않습니다")
	}
	if !strings.EqualFold(filepath.Base(target), "CodexQueueStudio.exe") {
		return errors.New("업데이트 대상 실행 파일이 올바르지 않습니다")
	}
	step(3, "다운로드한 패키지의 무결성을 확인하고 있습니다")
	if err := verifyStage(stage); err != nil {
		return err
	}
	step(12, "기존 프로그램을 안전하게 종료하고 있습니다")
	if err := wait(pid); err != nil {
		return err
	}
	directory := filepath.Dir(target)
	names := []string{"CodexQueueStudio.exe", "CodexQueueUpdater.exe"}
	backed := []string{}
	replaced := []string{}
	rollback := func() {
		step(35, "기존 버전을 복원하고 있습니다")
		for _, name := range replaced {
			_ = os.Remove(filepath.Join(directory, name))
		}
		for _, name := range backed {
			_ = os.Rename(filepath.Join(directory, name)+".backup", filepath.Join(directory, name))
		}
	}
	for i, name := range names {
		step(19+i*21, "이전 실행 파일을 백업하고 있습니다")
		dst := filepath.Join(directory, name)
		if _, err := os.Stat(dst); err == nil {
			backup := dst + ".backup"
			_ = os.Remove(backup)
			var moveErr error
			for k := 0; k < 24; k++ {
				moveErr = os.Rename(dst, backup)
				if moveErr == nil {
					break
				}
				time.Sleep(250 * time.Millisecond)
			}
			if moveErr != nil {
				rollback()
				return fmt.Errorf("파일 백업 실패: %s: %w", name, moveErr)
			}
			backed = append(backed, name)
		}
		data, err := os.ReadFile(filepath.Join(stage, name))
		if err != nil {
			rollback()
			return err
		}
		step(35+i*21, "새 버전을 설치하고 있습니다")
		if err = atomicWriteExecutable(directory, name, data); err != nil {
			rollback()
			return err
		}
		replaced = append(replaced, name)
	}
	nonce := newNonce()
	if nonce == "" {
		rollback()
		return errors.New("건강 검사 토큰을 만들 수 없습니다")
	}
	marker := filepath.Join(stage, "startup-"+nonce+".ok")
	defer os.Remove(marker)
	step(80, "새 버전을 실행하고 있습니다")
	cmd := exec.Command(target, "--post-update-health-file", marker, "--post-update-health-nonce", nonce)
	if err := cmd.Start(); err != nil {
		rollback()
		return fmt.Errorf("프로그램 재실행 실패: %w", err)
	}
	if err := awaitHealthyStart(cmd.Process, marker, nonce, step, 38*time.Second); err != nil {
		_ = cmd.Process.Kill()
		_, _ = cmd.Process.Wait()
		rollback()
		// Restore the old version after stopping the unresponsive child.
		if old, launchErr := os.StartProcess(target, []string{target}, &os.ProcAttr{Dir: directory, Files: []*os.File{}}); launchErr == nil {
			_ = old.Release()
		}
		return fmt.Errorf("새 프로그램 실행 검증 실패: %w", err)
	}
	_ = cmd.Process.Release()
	step(100, "새 버전이 정상적으로 응답합니다")
	for _, name := range backed {
		_ = os.Remove(filepath.Join(directory, name) + ".backup")
	}
	// Do not delete stage here: the updater executable itself runs from this folder.
	return nil
}
func main() {
	parent := flag.Int("wait-pid", 0, "parent process")
	target := flag.String("target", "", "target GUI executable")
	source := flag.String("source", "", "verified stage directory")
	flag.Parse()
	runUpdaterWindow(func(report func(int, string)) error {
		return install(*parent, *target, *source, report)
	})
}
