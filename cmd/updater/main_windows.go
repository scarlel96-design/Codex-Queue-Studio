//go:build windows

// Codex Queue Studio update helper. Replaces only a verified, staged executable.
//
package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"
	"unsafe"
)

func message(s string) {
	u := syscall.NewLazyDLL("user32.dll").NewProc("MessageBoxW")
	p, _ := syscall.UTF16PtrFromString(s)
	t, _ := syscall.UTF16PtrFromString("Codex Queue Studio Updater")
	u.Call(0, uintptr(unsafe.Pointer(p)), uintptr(unsafe.Pointer(t)), 0x10)
}
func wait(pid int) error {
	kernel := syscall.NewLazyDLL("kernel32.dll")
	open := kernel.NewProc("OpenProcess")
	h, _, err := open.Call(0x00100000, 0, uintptr(pid))
	if h == 0 {
		return fmt.Errorf("프로세스를 열지 못했습니다: %w", err)
	}
	defer kernel.NewProc("CloseHandle").Call(h)
	r, _, e := kernel.NewProc("WaitForSingleObject").Call(h, 90000)
	if r != 0 {
		return fmt.Errorf("기존 프로그램 종료 대기 실패: %w", e)
	}
	return nil
}
func install(pid int, orig, stage string) error {
	if pid <= 0 || !filepath.IsAbs(orig) || !filepath.IsAbs(stage) {
		return errors.New("업데이트 경로 또는 PID가 올바르지 않습니다")
	}
	if !strings.EqualFold(filepath.Base(orig), "CodexQueueStudio.exe") || !strings.EqualFold(filepath.Base(stage), "CodexQueueStudio.new.exe") {
		return errors.New("예상하지 못한 업데이트 파일명")
	}
	if filepath.Clean(orig) == filepath.Clean(stage) {
		return errors.New("기존 실행 파일과 새 파일이 같습니다")
	}
	if st, err := os.Stat(stage); err != nil || st.Size() < 100000 || st.Size() > 50<<20 {
		return errors.New("스테이징 파일이 없거나 크기가 유효하지 않습니다")
	}
	if err := wait(pid); err != nil {
		return err
	}
	backup := orig + ".backup"
	_ = os.Remove(backup)
	var err error
	for i := 0; i < 30; i++ {
		err = os.Rename(orig, backup)
		if err == nil {
			break
		}
		time.Sleep(500 * time.Millisecond)
	}
	if err != nil {
		return fmt.Errorf("기존 파일 백업 실패: %w", err)
	}
	if err = os.Rename(stage, orig); err != nil {
		_ = os.Rename(backup, orig)
		return fmt.Errorf("파일 교체 실패, 백업 복원 시도: %w", err)
	}
	cmd := exec.Command(orig)
	if err = cmd.Start(); err != nil {
		return fmt.Errorf("업데이트 적용 후 재실행 실패: %w", err)
	}
	_ = cmd.Process.Release()
	return nil
}
func main() {
	pid := flag.Int("wait-pid", 0, "")
	target := flag.String("target", "", "")
	source := flag.String("source", "", "")
	flag.Parse()
	if err := install(*pid, *target, *source); err != nil {
		message(err.Error())
	}
}
