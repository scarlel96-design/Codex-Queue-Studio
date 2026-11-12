//go:build windows

package main

import (
	"fmt"
	"runtime"
	"sync"
	"syscall"
	"time"
	"unsafe"
)

const (
	updaterWM_CREATE   = 1
	updaterWM_DESTROY  = 2
	updaterWM_PAINT    = 15
	updaterWM_CLOSE    = 16
	updaterWM_TIMER    = 0x113
	updaterWM_APP      = 0x8000
	updaterWM_REFRESH  = updaterWM_APP + 10
	updaterWindowStyle = 0x00CF0000 | 0x10000000
)

type updaterRECT struct{ Left, Top, Right, Bottom int32 }
type updaterPAINT struct {
	Hdc          uintptr
	Erase        int32
	Rect         updaterRECT
	Restore, Inc int32
	Pad          [32]byte
}
type updaterClass struct {
	Size, Style                        uint32
	Proc                               uintptr
	ClassExtra, WindowExtra            int32
	Instance, Icon, Cursor, Background uintptr
	Menu, Name                         *uint16
	SmallIcon                          uintptr
}
type updaterMSG struct {
	Hwnd           uintptr
	Message        uint32
	WParam, LParam uintptr
	Time           uint32
	Pt             struct{ X, Y int32 }
	Private        uint32
}
type updaterView struct {
	mu                             sync.Mutex
	hwnd                           uintptr
	titleFont, bodyFont, smallFont uintptr
	percent                        int
	status                         string
	details                        string
	failed, done                   bool
}

var activeUpdater *updaterView
var updaterUser = syscall.NewLazyDLL("user32.dll")
var updaterGDI = syscall.NewLazyDLL("gdi32.dll")
var updaterKernel = syscall.NewLazyDLL("kernel32.dll")

func upCall(lib *syscall.LazyDLL, name string, args ...uintptr) uintptr {
	r, _, _ := lib.NewProc(name).Call(args...)
	return r
}
func upString(s string) uintptr {
	p, _ := syscall.UTF16PtrFromString(s)
	return uintptr(unsafe.Pointer(p))
}
func upRGB(r, g, b uint32) uintptr { return uintptr(r | (g << 8) | (b << 16)) }
func upFill(hdc uintptr, r updaterRECT, color uintptr) {
	brush := upCall(updaterGDI, "CreateSolidBrush", color)
	upCall(updaterUser, "FillRect", hdc, uintptr(unsafe.Pointer(&r)), brush)
	upCall(updaterGDI, "DeleteObject", brush)
}
func upRound(hdc uintptr, r updaterRECT, color uintptr, rad uintptr) {
	br := upCall(updaterGDI, "CreateSolidBrush", color)
	pen := upCall(updaterGDI, "CreatePen", 0, 1, color)
	oldBrush := upCall(updaterGDI, "SelectObject", hdc, br)
	oldPen := upCall(updaterGDI, "SelectObject", hdc, pen)
	upCall(updaterGDI, "RoundRect", hdc, uintptr(r.Left), uintptr(r.Top), uintptr(r.Right), uintptr(r.Bottom), rad, rad)
	upCall(updaterGDI, "SelectObject", hdc, oldBrush)
	upCall(updaterGDI, "SelectObject", hdc, oldPen)
	upCall(updaterGDI, "DeleteObject", br)
	upCall(updaterGDI, "DeleteObject", pen)
}
func upText(hdc uintptr, font uintptr, txt string, r updaterRECT, col uintptr, flags uintptr) {
	upCall(updaterGDI, "SetBkMode", hdc, 1)
	upCall(updaterGDI, "SetTextColor", hdc, col)
	old := upCall(updaterGDI, "SelectObject", hdc, font)
	upCall(updaterUser, "DrawTextW", hdc, upString(txt), ^uintptr(0), uintptr(unsafe.Pointer(&r)), flags)
	upCall(updaterGDI, "SelectObject", hdc, old)
}
func (u *updaterView) redraw(hdc uintptr) {
	u.mu.Lock()
	pct, phase, details, failed, done := u.percent, u.status, u.details, u.failed, u.done
	u.mu.Unlock()
	bg := upRGB(15, 19, 29)
	card := upRGB(26, 33, 48)
	fg := upRGB(238, 244, 253)
	muted := upRGB(152, 172, 197)
	accent := upRGB(79, 143, 252)
	upFill(hdc, updaterRECT{0, 0, 662, 328}, bg)
	upRound(hdc, updaterRECT{26, 21, 76, 71}, accent, 18)
	upText(hdc, u.titleFont, "↻", updaterRECT{37, 23, 70, 68}, fg, 0x20|0x4|1)
	upText(hdc, u.titleFont, "Codex Queue Studio", updaterRECT{92, 20, 627, 60}, fg, 0x20|0x4)
	upText(hdc, u.smallFont, "SOFTWARE UPDATE  ·  VERIFIED INSTALLATION", updaterRECT{95, 64, 632, 84}, muted, 0x20)
	upRound(hdc, updaterRECT{24, 104, 636, 299}, card, 20)
	title := "업데이트를 안전하게 설치하고 있습니다"
	if failed {
		title = "업데이트에 실패했습니다 · 복구 결과를 확인하세요"
	}
	if done && !failed {
		title = "새 버전 실행 검증이 완료되었습니다"
	}
	upText(hdc, u.bodyFont, title, updaterRECT{46, 123, 615, 157}, fg, 0x20)
	if phase == "" {
		phase = "업데이트 준비 중"
	}
	upText(hdc, u.smallFont, phase, updaterRECT{46, 164, 590, 194}, muted, 0x20|0x8000)
	upText(hdc, u.smallFont, fmt.Sprintf("%d%%", pct), updaterRECT{557, 164, 604, 194}, fg, 0x20|1)
	upRound(hdc, updaterRECT{46, 207, 602, 220}, upRGB(56, 68, 88), 12)
	if pct > 0 {
		w := int32(556 * pct / 100)
		if w < 13 {
			w = 13
		}
		upRound(hdc, updaterRECT{46, 207, 46 + w, 220}, accent, 12)
	}
	if failed {
		upText(hdc, u.smallFont, details, updaterRECT{46, 230, 596, 290}, upRGB(251, 166, 167), 0x10|0x8000)
	} else {
		upText(hdc, u.smallFont, "설치 중 예약 데이터와 Codex 세션 파일은 변경하지 않습니다.", updaterRECT{46, 235, 608, 267}, muted, 0x20)
	}
}
func (u *updaterView) set(percent int, phase string) {
	if percent < 0 {
		percent = 0
	}
	if percent > 100 {
		percent = 100
	}
	u.mu.Lock()
	u.percent = percent
	u.status = phase
	u.mu.Unlock()
	if u.hwnd != 0 {
		upCall(updaterUser, "PostMessageW", u.hwnd, updaterWM_REFRESH, 0, 0)
	}
}
func updaterProc(hwnd, msg, w, l uintptr) uintptr {
	u := activeUpdater
	if u == nil {
		return upCall(updaterUser, "DefWindowProcW", hwnd, msg, w, l)
	}
	switch uint32(msg) {
	case updaterWM_PAINT:
		var ps updaterPAINT
		hdc := upCall(updaterUser, "BeginPaint", hwnd, uintptr(unsafe.Pointer(&ps)))
		u.redraw(hdc)
		upCall(updaterUser, "EndPaint", hwnd, uintptr(unsafe.Pointer(&ps)))
		return 0
	case updaterWM_REFRESH:
		upCall(updaterUser, "InvalidateRect", hwnd, 0, 0)
		return 0
	case updaterWM_TIMER:
		upCall(updaterUser, "KillTimer", hwnd, 501)
		upCall(updaterUser, "DestroyWindow", hwnd)
		return 0
	case updaterWM_CLOSE:
		u.mu.Lock()
		done := u.done
		u.mu.Unlock()
		if !done {
			return 0
		} // The helper cannot be interrupted mid-replacement.
		upCall(updaterUser, "DestroyWindow", hwnd)
		return 0
	case updaterWM_DESTROY:
		upCall(updaterUser, "PostQuitMessage", 0)
		return 0
	}
	return upCall(updaterUser, "DefWindowProcW", hwnd, msg, w, l)
}
func runUpdaterWindow(task func(func(int, string)) error) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	u := &updaterView{status: "설치 준비", percent: 0}
	activeUpdater = u
	const className = "CodexQueueStudioUpdaterV2"
	inst := upCall(updaterKernel, "GetModuleHandleW", 0)
	u.titleFont = upCall(updaterGDI, "CreateFontW", uintptr(^uint32(28)+1), 0, 0, 0, 650, 0, 0, 0, 1, 0, 0, 5, 0, upString("Segoe UI Variable Display"))
	u.bodyFont = upCall(updaterGDI, "CreateFontW", uintptr(^uint32(19)+1), 0, 0, 0, 600, 0, 0, 0, 1, 0, 0, 5, 0, upString("Segoe UI Variable Text"))
	u.smallFont = upCall(updaterGDI, "CreateFontW", uintptr(^uint32(15)+1), 0, 0, 0, 400, 0, 0, 0, 1, 0, 0, 5, 0, upString("Segoe UI Variable Text"))
	cls := updaterClass{Size: uint32(unsafe.Sizeof(updaterClass{})), Proc: syscall.NewCallback(updaterProc), Instance: inst, Cursor: upCall(updaterUser, "LoadCursorW", 0, 32512), Name: (*uint16)(unsafe.Pointer(upString(className)))}
	if upCall(updaterUser, "RegisterClassExW", uintptr(unsafe.Pointer(&cls))) == 0 {
		return
	}
	hwnd := upCall(updaterUser, "CreateWindowExW", 0, upString(className), upString("Codex Queue Studio · 업데이트"), updaterWindowStyle, 0x80000000, 0x80000000, 662, 352, 0, 0, inst, 0)
	if hwnd == 0 {
		return
	}
	u.hwnd = hwnd
	upCall(updaterUser, "ShowWindow", hwnd, 5)
	go func() {
		err := task(u.set)
		u.mu.Lock()
		u.done = true
		if err != nil {
			u.failed = true
			u.status = "자동 복구를 시도했습니다"
			u.details = err.Error()
		} else {
			u.percent = 100
			u.status = "정상 실행이 확인되었습니다"
		}
		u.mu.Unlock()
		upCall(updaterUser, "PostMessageW", hwnd, updaterWM_REFRESH, 0, 0)
		if err == nil {
			upCall(updaterUser, "SetTimer", hwnd, 501, 1900, 0)
		}
	}()
	var msg updaterMSG
	for upCall(updaterUser, "GetMessageW", uintptr(unsafe.Pointer(&msg)), 0, 0, 0) > 0 {
		upCall(updaterUser, "TranslateMessage", uintptr(unsafe.Pointer(&msg)))
		upCall(updaterUser, "DispatchMessageW", uintptr(unsafe.Pointer(&msg)))
	}
	for _, f := range []uintptr{u.titleFont, u.bodyFont, u.smallFont} {
		if f != 0 {
			upCall(updaterGDI, "DeleteObject", f)
		}
	}
	// Make time explicit in diagnostics, without leaking scheduled message content.
	_ = time.Now()
}
