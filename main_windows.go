//go:build windows

package main

import (
	"context"
	_ "embed"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"syscall"
	"time"
	"unsafe"

	"codexqueuestudiov3/internal"
)

//go:embed assets/icon.ico
var iconBytes []byte

const (
	WS_OVERLAPPEDWINDOW          = 0x00CF0000
	WS_VISIBLE                   = 0x10000000
	WS_CHILD                     = 0x40000000
	WS_TABSTOP                   = 0x00010000
	WS_VSCROLL                   = 0x00200000
	WS_HSCROLL                   = 0x00100000
	WS_BORDER                    = 0x00800000
	ES_MULTILINE                 = 0x0004
	ES_READONLY                  = 0x0800
	ES_AUTOVSCROLL               = 0x0040
	ES_WANTRETURN                = 0x1000
	LVS_REPORT                   = 1
	LVS_SINGLESEL                = 4
	LVS_SHOWSELALWAYS            = 8
	LVS_OWNERDATA                = 0x1000
	LVM_FIRST                    = 0x1000
	LVM_GETNEXTITEM              = LVM_FIRST + 12
	LVM_SETITEMCOUNT             = LVM_FIRST + 47
	LVM_INSERTCOLUMNW            = LVM_FIRST + 97
	LVM_SETEXTENDEDLISTVIEWSTYLE = LVM_FIRST + 54
	LVM_SETTEXTCOLOR             = LVM_FIRST + 36
	LVM_SETBKCOLOR               = LVM_FIRST + 1
	LVM_SETTEXTBKCOLOR           = LVM_FIRST + 38
	LVS_EX_FULLROWSELECT         = 0x20
	LVS_EX_DOUBLEBUFFER          = 0x10000
	WM_CREATE                    = 1
	WM_DESTROY                   = 2
	WM_SIZE                      = 5
	WM_PAINT                     = 15
	WM_ERASEBKGND                = 20
	WM_CLOSE                     = 16
	WM_COMMAND                   = 273
	WM_NOTIFY                    = 78
	WM_DRAWITEM                  = 43
	WM_CTLCOLORSTATIC            = 0x0138
	WM_CTLCOLOREDIT              = 0x0133
	WM_SETFONT                   = 0x30
	WM_SETICON                   = 0x80
	WM_LBUTTONDBLCLK             = 0x0203
	WM_APP                       = 0x8000
	WM_TRAY                      = WM_APP + 20
	BN_CLICKED                   = 0
	EN_CHANGE                    = 0x300
	LVN_ITEMCHANGED              = -101
	LVN_GETDISPINFOW             = -177
	IDC_ARROW                    = 32512
	IMAGE_ICON                   = 1
	LR_LOADFROMFILE              = 0x10
	LR_DEFAULTSIZE               = 0x40
	SW_HIDE                      = 0
	SW_SHOW                      = 5
	SW_RESTORE                   = 9
	COLOR_WINDOW                 = 5
	WM_GETMINMAXINFO             = 0x0024
	WM_SETTINGCHANGE             = 0x001A
	WM_TIMER                     = 0x0113
	DTM_GETSYSTEMTIME            = 0x1001
	DTM_SETSYSTEMTIME            = 0x1002
	DTM_SETFORMATW               = 0x1032
	DTS_TIMEFORMAT               = 0x0009
	TRANSPARENT                  = 1
	DT_VCENTER                   = 4
	DT_SINGLELINE                = 0x20
	DT_CENTER                    = 1
	DT_END_ELLIPSIS              = 0x8000
	ID_SESSIONS                  = 101
	ID_JOBS                      = 102
	ID_SETTINGS                  = 103
	ID_THEME                     = 104
	ID_REFRESH                   = 105
	ID_SEARCH                    = 106
	ID_SESSLIST                  = 107
	ID_HISTORY                   = 108
	ID_MESSAGE                   = 109
	ID_WHEN                      = 110
	ID_SENDNOW                   = 111
	ID_SCHEDULE                  = 112
	ID_JOBLIST                   = 113
	ID_CANCEL                    = 114
	ID_ENGINE                    = 115
	ID_CHECK                     = 116
	ID_UPDATE                    = 117
	ID_HOMEPATH                  = 118
	ID_SAVE                      = 119
	ID_EXIT                      = 120
	ID_DATE                      = 121
	ID_TIME                      = 122
	ID_REACTIVATE                = 123
)

type RECT struct{ Left, Top, Right, Bottom int32 }
type POINT struct{ X, Y int32 }
type SYSTEMTIME struct{ Year, Month, DayOfWeek, Day, Hour, Minute, Second, Milliseconds uint16 }
type MINMAXINFO struct{ Reserved, MaxSize, MaxPosition, MinTrackSize, MaxTrackSize POINT }
type MSG struct {
	Hwnd           uintptr
	Message        uint32
	WParam, LParam uintptr
	Time           uint32
	Pt             POINT
	LPrivate       uint32
}
type WNDCLASSEX struct {
	Size, Style                        uint32
	WndProc                            uintptr
	ClsExtra, WndExtra                 int32
	Instance, Icon, Cursor, Background uintptr
	MenuName, ClassName                *uint16
	SmallIcon                          uintptr
}
type PAINTSTRUCT struct {
	Hdc                uintptr
	Erase              int32
	RcPaint            RECT
	Restore, IncUpdate int32
	Reserved           [32]byte
}
type INITCOMMONCONTROLSEX struct{ Size, ICC uint32 }
type LVCOLUMNW struct {
	Mask                               uint32
	Fmt                                int32
	CX                                 int32
	Text                               *uint16
	TextMax                            int32
	SubItem                            int32
	Image                              int32
	Order                              int32
	MinWidth, DefaultWidth, IdealWidth int32
}
type NMHDR struct {
	HwndFrom uintptr
	IDFrom   uintptr
	Code     int32
	Pad      uint32
}
type LVITEMW struct {
	Mask      uint32
	IItem     int32
	ISubItem  int32
	State     uint32
	StateMask uint32
	Text      *uint16
	TextMax   int32
	Image     int32
	LParam    uintptr
	Indent    int32
	GroupID   int32
	Columns   uint32
	PColumns  uintptr
	ColFmt    uintptr
	Group     int32
}
type NMLVDISPINFOW struct {
	Hdr  NMHDR
	Item LVITEMW
}
type DRAWITEMSTRUCT struct {
	CtlType, CtlID, ItemID, ItemAction, ItemState uint32
	Hwnd, Hdc                                     uintptr
	Rect                                          RECT
	ItemData                                      uintptr
}
type NOTIFYICONDATAW struct {
	Size                        uint32
	Hwnd                        uintptr
	UID, Flags, CallbackMessage uint32
	Icon                        uintptr
	Tip                         [128]uint16
	State, StateMask            uint32
	Info                        [256]uint16
	Timeout                     uint32
	InfoTitle                   [64]uint16
	InfoFlags                   uint32
	Guid                        [16]byte
	BalloonIcon                 uintptr
}

type Palette struct{ bg, side, card, field, text, muted, accent, accentText, border uint32 }

func signed(v int32) uintptr    { return uintptr(uint64(int64(v))) }
func rgb(r, g, b uint32) uint32 { return r | (g << 8) | (b << 16) }
func palette(dark bool) Palette {
	if dark {
		return Palette{rgb(17, 21, 29), rgb(24, 30, 41), rgb(33, 41, 54), rgb(30, 38, 50), rgb(239, 243, 251), rgb(160, 173, 193), rgb(41, 108, 199), rgb(255, 255, 255), rgb(59, 70, 85)}
	}
	return Palette{rgb(243, 246, 251), rgb(231, 237, 247), rgb(255, 255, 255), rgb(250, 252, 255), rgb(31, 44, 66), rgb(93, 111, 136), rgb(33, 112, 223), rgb(255, 255, 255), rgb(210, 221, 235)}
}

var u32 = syscall.NewLazyDLL("user32.dll")
var g32 = syscall.NewLazyDLL("gdi32.dll")
var k32 = syscall.NewLazyDLL("kernel32.dll")
var c32 = syscall.NewLazyDLL("comctl32.dll")
var dwm = syscall.NewLazyDLL("dwmapi.dll")
var shell = syscall.NewLazyDLL("shell32.dll")

func proc(lib *syscall.LazyDLL, n string) *syscall.LazyProc { return lib.NewProc(n) }
func call(lib *syscall.LazyDLL, n string, args ...uintptr) uintptr {
	r, _, _ := proc(lib, n).Call(args...)
	return r
}
func utf(s string) *uint16    { p, _ := syscall.UTF16PtrFromString(s); return p }
func strptr(s string) uintptr { return uintptr(unsafe.Pointer(utf(s))) }
func send(hwnd uintptr, msg uint32, w, l uintptr) uintptr {
	return call(u32, "SendMessageW", hwnd, uintptr(msg), w, l)
}
func show(h uintptr, on bool) {
	n := uintptr(SW_HIDE)
	if on {
		n = SW_SHOW
	}
	call(u32, "ShowWindow", h, n)
}
func move(h uintptr, x, y, w, ht int32) {
	call(u32, "MoveWindow", h, uintptr(x), uintptr(y), uintptr(w), uintptr(ht), 1)
}
func settext(h uintptr, text string) { call(u32, "SetWindowTextW", h, strptr(text)) }
func gettext(h uintptr) string {
	n := int(call(u32, "GetWindowTextLengthW", h))
	if n <= 0 {
		return ""
	}
	buf := make([]uint16, n+2)
	call(u32, "GetWindowTextW", h, uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)))
	return syscall.UTF16ToString(buf)
}
func createWindow(class, title string, style, exstyle, parent uintptr, id int, x, y, w, h int32) uintptr {
	return call(u32, "CreateWindowExW", exstyle, strptr(class), strptr(title), style, uintptr(x), uintptr(y), uintptr(w), uintptr(h), parent, uintptr(id), call(k32, "GetModuleHandleW", 0), 0)
}
func setFont(h, f uintptr)     { send(h, WM_SETFONT, f, 1) }
func brush(col uint32) uintptr { return call(g32, "CreateSolidBrush", uintptr(col)) }
func fill(hdc uintptr, r RECT, c uint32) {
	b := brush(c)
	call(u32, "FillRect", hdc, uintptr(unsafe.Pointer(&r)), b)
	call(g32, "DeleteObject", b)
}
func drawText(hdc uintptr, text string, r RECT, c uint32, font uintptr, flags uint32) {
	call(g32, "SetBkMode", hdc, TRANSPARENT)
	call(g32, "SetTextColor", hdc, uintptr(c))
	old := call(g32, "SelectObject", hdc, font)
	call(u32, "DrawTextW", hdc, strptr(text), ^uintptr(0), uintptr(unsafe.Pointer(&r)), uintptr(flags))
	call(g32, "SelectObject", hdc, old)
}
func roundBox(hdc uintptr, r RECT, c uint32) {
	b := brush(c)
	p := call(g32, "CreatePen", 0, 1, uintptr(c))
	ob := call(g32, "SelectObject", hdc, b)
	op := call(g32, "SelectObject", hdc, p)
	call(g32, "RoundRect", hdc, uintptr(r.Left), uintptr(r.Top), uintptr(r.Right), uintptr(r.Bottom), 16, 16)
	call(g32, "SelectObject", hdc, ob)
	call(g32, "SelectObject", hdc, op)
	call(g32, "DeleteObject", b)
	call(g32, "DeleteObject", p)
}

var gui *GUI

type GUI struct {
	core                           *Core
	hwnd                           uintptr
	font, small, bold, icon        uintptr
	dark                           bool
	page                           int
	W, H                           int32
	pal                            Palette
	controls                       map[int]uintptr
	sessions                       []internal.Session
	visible                        []internal.Session
	selected                       int
	selectedJob                    int
	jobs                           []internal.JobPublic
	historySeq                     atomic.Uint64
	history                        string
	result                         string
	updateTarget                   string
	brushCard, brushField, brushBG uintptr
	tray                           bool
	quitting                       bool
}

func (g *GUI) ctl(id int) uintptr { return g.controls[id] }
func (g *GUI) add(id int, class, title string, style uintptr) {
	h := createWindow(class, title, style|WS_CHILD|WS_VISIBLE, 0, g.hwnd, id, 0, 0, 100, 30)
	g.controls[id] = h
	setFont(h, g.font)
}
func (g *GUI) button(id int, name string) { g.add(id, "BUTTON", name, WS_TABSTOP|0x0000000B) } // BS_OWNERDRAW
func (g *GUI) edit(id int, multi bool) {
	style := uintptr(WS_TABSTOP | 0x0080)
	if multi {
		style = WS_TABSTOP | WS_VSCROLL | ES_MULTILINE | ES_AUTOVSCROLL | ES_WANTRETURN
	}
	g.add(id, "EDIT", "", style)
}
func (g *GUI) list(id int, headers []struct {
	title string
	width int32
}) {
	g.add(id, "SysListView32", "", WS_TABSTOP|LVS_REPORT|LVS_SINGLESEL|LVS_SHOWSELALWAYS|LVS_OWNERDATA)
	h := g.ctl(id)
	send(h, LVM_SETEXTENDEDLISTVIEWSTYLE, 0, LVS_EX_FULLROWSELECT|LVS_EX_DOUBLEBUFFER)
	for i, c := range headers {
		title := utf(c.title)
		col := LVCOLUMNW{Mask: 0x7, CX: c.width, Text: title, SubItem: int32(i)}
		send(h, LVM_INSERTCOLUMNW, uintptr(i), uintptr(unsafe.Pointer(&col)))
	}
}
func (g *GUI) initControls() {
	g.controls = map[int]uintptr{}
	for _, b := range []struct {
		id    int
		title string
	}{{ID_SESSIONS, "세션 탐색기"}, {ID_JOBS, "예약 관리"}, {ID_SETTINGS, "환경 설정"}, {ID_THEME, "테마 전환"}, {ID_REFRESH, "새로고침"}, {ID_SENDNOW, "지금 전송"}, {ID_SCHEDULE, "예약 등록"}, {ID_CANCEL, "예약 취소"}, {ID_REACTIVATE, "선택 예약 재개"}, {ID_ENGINE, "Codex 엔진 설치"}, {ID_CHECK, "업데이트 확인"}, {ID_UPDATE, "업데이트 설치"}, {ID_SAVE, "경로 적용"}, {ID_EXIT, "완전히 종료"}} {
		g.button(b.id, b.title)
	}
	g.edit(ID_SEARCH, false)
	g.edit(ID_HISTORY, true)
	g.edit(ID_MESSAGE, true)
	g.add(ID_DATE, "SysDateTimePick32", "", WS_TABSTOP)
	g.add(ID_TIME, "SysDateTimePick32", "", WS_TABSTOP|DTS_TIMEFORMAT)
	send(g.ctl(ID_DATE), DTM_SETFORMATW, 0, strptr("yyyy-MM-dd"))
	send(g.ctl(ID_TIME), DTM_SETFORMATW, 0, strptr("HH:mm"))
	g.edit(ID_HOMEPATH, false)
	g.list(ID_SESSLIST, []struct {
		title string
		width int32
	}{{"세션 제목", 260}, {"프로젝트", 140}, {"수정일", 135}})
	g.list(ID_JOBLIST, []struct {
		title string
		width int32
	}{{"상태", 94}, {"세션 제목", 290}, {"예약 시각", 180}, {"메시지", 330}})
	send(g.ctl(ID_HISTORY), 0x00CF, 1, 0) // EM_SETREADONLY
	future := time.Now().Add(10 * time.Minute)
	stamp := SYSTEMTIME{Year: uint16(future.Year()), Month: uint16(future.Month()), Day: uint16(future.Day()), Hour: uint16(future.Hour()), Minute: uint16(future.Minute())}
	send(g.ctl(ID_DATE), DTM_SETSYSTEMTIME, 0, uintptr(unsafe.Pointer(&stamp)))
	send(g.ctl(ID_TIME), DTM_SETSYSTEMTIME, 0, uintptr(unsafe.Pointer(&stamp)))
	settext(g.ctl(ID_HOMEPATH), g.core.store.Snapshot().Settings.CodexHome)
	g.updateColors()
	g.layout()
}
func (g *GUI) updateColors() {
	g.pal = palette(g.dark)
	for _, h := range []uintptr{g.brushBG, g.brushCard, g.brushField} {
		if h != 0 {
			call(g32, "DeleteObject", h)
		}
	}
	g.brushBG = brush(g.pal.bg)
	g.brushCard = brush(g.pal.card)
	g.brushField = brush(g.pal.field)
	dark := uintptr(0)
	if g.dark {
		dark = 1
	}
	call(dwm, "DwmSetWindowAttribute", g.hwnd, 20, uintptr(unsafe.Pointer(&dark)), unsafe.Sizeof(uint32(0)))
	for _, id := range []int{ID_SESSLIST, ID_JOBLIST} {
		h := g.ctl(id)
		send(h, LVM_SETTEXTCOLOR, 0, uintptr(g.pal.text))
		send(h, LVM_SETBKCOLOR, 0, uintptr(g.pal.card))
		send(h, LVM_SETTEXTBKCOLOR, 0, uintptr(g.pal.card))
	}
	call(u32, "InvalidateRect", g.hwnd, 0, 1)
}
func (g *GUI) layout() {
	w, h := g.W, g.H
	if w < 800 {
		w = 800
	}
	if h < 580 {
		h = 580
	}
	g.W = w
	g.H = h
	side := int32(208)
	x := side + 25
	area := w - x - 24
	for i, id := range []int{ID_SESSIONS, ID_JOBS, ID_SETTINGS} {
		move(g.ctl(id), 14, 112+int32(i)*51, 180, 40)
	}
	move(g.ctl(ID_THEME), 14, h-130, 180, 38)
	move(g.ctl(ID_EXIT), 14, h-78, 180, 38)
	all := []int{ID_SEARCH, ID_REFRESH, ID_SESSLIST, ID_HISTORY, ID_MESSAGE, ID_DATE, ID_TIME, ID_SENDNOW, ID_SCHEDULE, ID_JOBLIST, ID_CANCEL, ID_REACTIVATE, ID_ENGINE, ID_CHECK, ID_UPDATE, ID_HOMEPATH, ID_SAVE}
	for _, id := range all {
		show(g.ctl(id), false)
	}
	switch g.page {
	case 0:
		left := area * 45 / 100
		if left < 330 {
			left = 330
		}
		right := area - left - 18
		move(g.ctl(ID_SEARCH), x, 124, left-126, 35)
		move(g.ctl(ID_REFRESH), x+left-116, 124, 116, 35)
		move(g.ctl(ID_SESSLIST), x, 173, left, h-211)
		xr := x + left + 18
		move(g.ctl(ID_HISTORY), xr, 236, right, h-538)
		move(g.ctl(ID_MESSAGE), xr, h-251, right, 117)
		move(g.ctl(ID_DATE), xr, h-117, right-365, 36)
		move(g.ctl(ID_TIME), xr+right-356, h-117, 95, 36)
		move(g.ctl(ID_SENDNOW), xr+right-251, h-117, 119, 36)
		move(g.ctl(ID_SCHEDULE), xr+right-124, h-117, 124, 36)
		for _, id := range []int{ID_SEARCH, ID_REFRESH, ID_SESSLIST, ID_HISTORY, ID_MESSAGE, ID_DATE, ID_TIME, ID_SENDNOW, ID_SCHEDULE} {
			show(g.ctl(id), true)
		}
	case 1:
		move(g.ctl(ID_JOBLIST), x, 138, area, h-247)
		move(g.ctl(ID_CANCEL), x+area-155, h-89, 155, 40)
		move(g.ctl(ID_REACTIVATE), x+area-326, h-89, 160, 40)
		show(g.ctl(ID_JOBLIST), true)
		show(g.ctl(ID_CANCEL), true)
		show(g.ctl(ID_REACTIVATE), true)
	case 2:
		move(g.ctl(ID_HOMEPATH), x+22, 176, area-44, 39)
		move(g.ctl(ID_SAVE), x+22, 230, 170, 39)
		move(g.ctl(ID_ENGINE), x+22, 343, 200, 41)
		move(g.ctl(ID_CHECK), x+237, 343, 190, 41)
		move(g.ctl(ID_UPDATE), x+442, 343, 190, 41)
		for _, id := range []int{ID_HOMEPATH, ID_SAVE, ID_ENGINE, ID_CHECK, ID_UPDATE} {
			show(g.ctl(id), true)
		}
	}
}
func (g *GUI) setPage(p int) {
	g.page = p
	g.layout()
	if p == 1 {
		g.refreshJobs()
	}
	call(u32, "InvalidateRect", g.hwnd, 0, 1)
}
func (g *GUI) refreshSessions() {
	search := strings.ToLower(strings.TrimSpace(gettext(g.ctl(ID_SEARCH))))
	all := g.core.Snapshot()
	g.sessions = all
	g.visible = g.visible[:0]
	for _, s := range all {
		if search == "" || strings.Contains(strings.ToLower(s.Title+" "+s.Project+" "+s.ID+" "+s.Path), search) {
			g.visible = append(g.visible, s)
		}
	}
	if len(g.visible) > 5000 {
		g.visible = g.visible[:5000]
	}
	send(g.ctl(ID_SESSLIST), LVM_SETITEMCOUNT, uintptr(len(g.visible)), 0)
	g.selected = -1
	call(u32, "InvalidateRect", g.ctl(ID_SESSLIST), 0, 1)
	call(u32, "InvalidateRect", g.hwnd, 0, 1)
}
func (g *GUI) refreshJobs() {
	g.jobs = g.core.Pending()
	send(g.ctl(ID_JOBLIST), LVM_SETITEMCOUNT, uintptr(len(g.jobs)), 0)
	call(u32, "InvalidateRect", g.ctl(ID_JOBLIST), 0, 1)
	call(u32, "InvalidateRect", g.hwnd, 0, 1)
}
func (g *GUI) asyncRefresh() {
	g.core.setStatus("세션 검색 중")
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if err := g.core.RefreshLocal(); err == nil {
			call(u32, "PostMessageW", g.hwnd, WM_APP+1, 0, 0)
			_ = g.core.MergeRemote(ctx)
		}
		call(u32, "PostMessageW", g.hwnd, WM_APP+1, 0, 0)
	}()
}
func (g *GUI) selectedSession() (internal.Session, bool) {
	if g.selected < 0 || g.selected >= len(g.visible) {
		return internal.Session{}, false
	}
	return g.visible[g.selected], true
}
func (g *GUI) selectSession(idx int) {
	g.selected = idx
	seq := g.historySeq.Add(1)
	s, ok := g.selectedSession()
	if !ok {
		return
	}
	g.history = "불러오는 중…"
	settext(g.ctl(ID_HISTORY), g.history)
	go func() {
		var output strings.Builder
		output.WriteString("세션: " + s.ID + "\r\n프로젝트: " + s.Path + "\r\n\r\n")
		if s.File != "" {
			hist, err := internal.ReadHistory(s.File, 150)
			if err != nil {
				output.WriteString("기록 오류: " + err.Error())
			} else {
				for _, h := range hist {
					output.WriteString("[" + h.Role + "]\r\n" + h.Text + "\r\n\r\n")
				}
			}
		} else {
			output.WriteString("App Server 전용 세션입니다. 기록 파일이 확인되지 않았습니다.")
		}
		if g.historySeq.Load() != seq {
			return
		}
		g.muHistory(output.String())
		call(u32, "PostMessageW", g.hwnd, WM_APP+2, 0, 0)
	}()
	call(u32, "InvalidateRect", g.hwnd, 0, 1)
}
func (g *GUI) muHistory(s string) { g.core.mu.Lock(); g.history = s; g.core.mu.Unlock() }
func (g *GUI) sendMessage(now bool) {
	s, ok := g.selectedSession()
	if !ok {
		g.alert("먼저 세션을 선택하세요.")
		return
	}
	message := gettext(g.ctl(ID_MESSAGE))
	when := time.Now()
	if !now {
		var d, clock SYSTEMTIME
		if send(g.ctl(ID_DATE), DTM_GETSYSTEMTIME, 0, uintptr(unsafe.Pointer(&d))) != 0 || send(g.ctl(ID_TIME), DTM_GETSYSTEMTIME, 0, uintptr(unsafe.Pointer(&clock))) != 0 {
			g.alert("날짜와 시간을 선택하세요.")
			return
		}
		when = time.Date(int(d.Year), time.Month(d.Month), int(d.Day), int(clock.Hour), int(clock.Minute), 0, 0, time.Local)
	}
	_, err := g.core.Schedule(s.ID, s.Title, message, when)
	if err != nil {
		g.alert(err.Error())
		return
	}
	settext(g.ctl(ID_MESSAGE), "")
	g.core.setStatus("예약이 등록되었습니다")
	if now {
		go g.core.SendDue()
	}
	g.refreshJobs()
	call(u32, "InvalidateRect", g.hwnd, 0, 1)
}
func (g *GUI) alert(text string) {
	call(u32, "MessageBoxW", g.hwnd, strptr(text), strptr("Codex Queue Studio"), 0x40)
}
func (g *GUI) ask(text string) bool {
	return call(u32, "MessageBoxW", g.hwnd, strptr(text), strptr("Codex Queue Studio"), 0x24) == 6
}
func (g *GUI) handleButton(id int) {
	switch id {
	case ID_SESSIONS:
		g.setPage(0)
	case ID_JOBS:
		g.setPage(1)
	case ID_SETTINGS:
		g.setPage(2)
	case ID_THEME:
		g.dark = !g.dark
		chosen := "light"
		if g.dark {
			chosen = "dark"
		}
		_ = g.core.store.Update(func(st *internal.State) error { st.Settings.Theme = chosen; return nil })
		g.updateColors()
	case ID_REFRESH:
		g.asyncRefresh()
	case ID_SENDNOW:
		g.sendMessage(true)
	case ID_SCHEDULE:
		g.sendMessage(false)
	case ID_CANCEL:
		jobs := g.jobs
		idx := int(send(g.ctl(ID_JOBLIST), LVM_GETNEXTITEM, ^uintptr(0), 2))
		if idx < 0 || idx >= len(jobs) {
			g.alert("취소할 예약을 선택하세요.")
			return
		}
		if err := g.core.Cancel(jobs[idx].ID); err != nil {
			g.alert(err.Error())
		}
		g.refreshJobs()
	case ID_REACTIVATE:
		idx := int(send(g.ctl(ID_JOBLIST), LVM_GETNEXTITEM, ^uintptr(0), 2))
		if idx < 0 || idx >= len(g.jobs) {
			g.alert("재개할 예약을 선택하세요.")
			return
		}
		if !g.ask("선택한 예약을 App Server 방식으로 실행하도록 다시 승인할까요?\n기존 실행 상태가 불명확한 경우 중복 요청이 발생할 수 있습니다.") {
			return
		}
		if err := g.core.Reactivate(g.jobs[idx].ID); err != nil {
			g.alert(err.Error())
		}
		g.refreshJobs()
	case ID_ENGINE:
		if g.core.EnginePresent() {
			g.alert("Codex 엔진이 이미 설치되었습니다.")
			return
		}
		if !g.ask("공식 Codex Windows 엔진을 다운로드합니다. 약 100MB 이상의 다운로드가 필요합니다. 계속할까요?") {
			return
		}
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
			defer cancel()
			err := g.core.InstallEngine(ctx)
			if err != nil {
				g.core.setStatus(err.Error())
			}
			call(u32, "PostMessageW", g.hwnd, WM_APP+1, 0, 0)
		}()
	case ID_CHECK:
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
			defer cancel()
			upd, err := g.core.CheckUpdate(ctx)
			if err != nil {
				g.core.setStatus(err.Error())
			} else if upd == nil {
				g.core.setStatus("현재 최신 버전입니다. GitHub 릴리스가 아직 없다면 업데이트가 표시되지 않습니다.")
			} else {
				g.core.setStatus("새 버전: " + upd.Tag)
			}
			call(u32, "PostMessageW", g.hwnd, WM_APP+1, 0, 0)
		}()
	case ID_UPDATE:
		upd := g.core.UpdateAvailable()
		if upd == nil {
			g.alert("먼저 업데이트를 확인하세요.")
			return
		}
		if !g.ask("검증된 업데이트 " + upd.Tag + "를 설치할까요?") {
			return
		}
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 4*time.Minute)
			defer cancel()
			path, err := g.core.DownloadUpdate(ctx, *upd)
			if err != nil {
				g.core.setStatus("업데이트 실패: " + err.Error())
			} else {
				g.core.mu.Lock()
				g.updateTarget = path
				g.core.mu.Unlock()
				g.core.setStatus("업데이트 검증 완료. 재시작 준비 중")
			}
			call(u32, "PostMessageW", g.hwnd, WM_APP+3, 0, 0)
		}()
	case ID_SAVE:
		path := strings.TrimSpace(gettext(g.ctl(ID_HOMEPATH)))
		if st, err := os.Stat(path); err != nil || !st.IsDir() {
			g.alert("Codex 홈 폴더가 없습니다.")
			return
		}
		err := g.core.store.Update(func(st *internal.State) error {
			st.Settings.CodexHome = path
			internal.AddEvent(st, "settings", "", "Codex 홈을 변경했습니다")
			return nil
		})
		if err != nil {
			g.alert(err.Error())
			return
		}
		g.asyncRefresh()
	case ID_EXIT:
		g.quitting = true
		call(u32, "DestroyWindow", g.hwnd)
	}
}
func (g *GUI) draw(hdc uintptr) {
	p := g.pal
	fill(hdc, RECT{0, 0, g.W, g.H}, p.bg)
	fill(hdc, RECT{0, 0, 208, g.H}, p.side)
	drawText(hdc, "◈", RECT{25, 26, 65, 64}, p.accent, g.bold, DT_SINGLELINE|DT_VCENTER)
	drawText(hdc, "Codex Queue", RECT{58, 28, 199, 56}, p.text, g.bold, DT_SINGLELINE|DT_VCENTER)
	drawText(hdc, "STUDIO  3.0", RECT{26, 65, 180, 91}, p.muted, g.small, DT_SINGLELINE)
	title := "세션 탐색기"
	sub := "Codex Desktop · CLI · App Server 통합"
	if g.page == 1 {
		title = "예약 작업"
		sub = "예약 실행 상태와 실패 이력을 관리합니다"
	} else if g.page == 2 {
		title = "환경 설정"
		sub = "독립 Codex 엔진 · 업데이트 · 세션 경로"
	}
	drawText(hdc, title, RECT{234, 28, g.W - 30, 65}, p.text, g.bold, DT_SINGLELINE)
	drawText(hdc, sub, RECT{234, 74, g.W - 30, 105}, p.muted, g.font, DT_SINGLELINE)
	status := g.core.Status()
	drawText(hdc, status, RECT{234, g.H - 30, g.W - 20, g.H - 9}, p.muted, g.small, DT_SINGLELINE|DT_END_ELLIPSIS)
	switch g.page {
	case 0:
		x := int32(233)
		area := g.W - x - 24
		left := area * 45 / 100
		if left < 330 {
			left = 330
		}
		xr := x + left + 18
		right := area - left - 18
		roundBox(hdc, RECT{x - 6, 165, x + left + 6, g.H - 36}, p.card)
		roundBox(hdc, RECT{xr - 6, 165, xr + right + 6, g.H - 36}, p.card)
		drawText(hdc, fmt.Sprintf("세션 %d개 / 검색 결과 %d개", len(g.sessions), len(g.visible)), RECT{x + 6, 110, x + left, 130}, p.muted, g.small, DT_SINGLELINE)
		heading := "세션을 선택하세요"
		if s, ok := g.selectedSession(); ok {
			heading = s.Title
		}
		drawText(hdc, heading, RECT{xr + 7, 171, xr + right - 8, 204}, p.text, g.bold, DT_SINGLELINE|DT_END_ELLIPSIS)
		drawText(hdc, "세션 대화 기록", RECT{xr + 4, 207, xr + right, 230}, p.muted, g.small, DT_SINGLELINE)
		drawText(hdc, "예약할 메시지", RECT{xr + 4, g.H - 276, xr + right, g.H - 250}, p.text, g.font, DT_SINGLELINE)
		drawText(hdc, "예약 날짜와 시간", RECT{xr + 4, g.H - 141, xr + right, g.H - 114}, p.muted, g.small, DT_SINGLELINE)
	case 1:
		roundBox(hdc, RECT{226, 123, g.W - 16, g.H - 115}, p.card)
		drawText(hdc, "취소는 대기 상태에서만 가능합니다. 실행 상태가 불명확하면 자동 재전송하지 않습니다.", RECT{245, g.H - 111, g.W - 190, g.H - 84}, p.muted, g.small, DT_SINGLELINE)
	case 2:
		roundBox(hdc, RECT{226, 127, g.W - 16, g.H - 54}, p.card)
		drawText(hdc, "Codex 홈 디렉터리", RECT{254, 144, g.W - 30, 173}, p.text, g.bold, DT_SINGLELINE)
		drawText(hdc, "Codex Desktop과 CLI의 로컬 세션 데이터를 이 폴더에서 조회합니다.", RECT{254, 279, g.W - 30, 312}, p.muted, g.font, DT_SINGLELINE)
		drawText(hdc, "Codex 실행 엔진 및 프로그램 업데이트", RECT{254, 310, g.W - 30, 344}, p.text, g.bold, DT_SINGLELINE)
		badge := "Codex 엔진 미설치 · 별도 CLI 설치 불필요"
		if g.core.EnginePresent() {
			badge = "내장 Codex 엔진 설치됨 · 별도 CLI 설치 불필요"
		}
		drawText(hdc, badge, RECT{254, 404, g.W - 20, 439}, p.muted, g.font, DT_SINGLELINE)
		drawText(hdc, "엔진은 OpenAI 공식 GitHub 릴리스에서 내려받고 SHA-256을 검증합니다.", RECT{254, 447, g.W - 20, 482}, p.muted, g.font, DT_SINGLELINE)
		drawText(hdc, "업데이트는 검증 후 사용자가 설치를 승인해야 적용됩니다.", RECT{254, 488, g.W - 20, 520}, p.muted, g.font, DT_SINGLELINE)
	}
}
func (g *GUI) drawButton(dis *DRAWITEMSTRUCT) {
	p := g.pal
	hdc := dis.Hdc
	id := int(dis.CtlID)
	selected := (id == ID_SESSIONS && g.page == 0) || (id == ID_JOBS && g.page == 1) || (id == ID_SETTINGS && g.page == 2)
	bg := p.card
	fg := p.text
	if id == ID_SCHEDULE || id == ID_SENDNOW || id == ID_ENGINE || selected {
		bg = p.accent
		fg = p.accentText
	}
	if id == ID_EXIT {
		bg = p.side
		fg = p.muted
	}
	if dis.ItemState&0x1 != 0 {
		bg = p.border
	}
	roundBox(hdc, RECT{2, 2, dis.Rect.Right - dis.Rect.Left - 2, dis.Rect.Bottom - dis.Rect.Top - 2}, bg)
	labels := map[int]string{ID_SESSIONS: "세션 탐색기", ID_JOBS: "예약 관리", ID_SETTINGS: "환경 설정", ID_THEME: "◐   테마 전환", ID_REFRESH: "새로고침", ID_SENDNOW: "지금 전송", ID_SCHEDULE: "예약 등록", ID_CANCEL: "예약 취소", ID_ENGINE: "Codex 엔진 설치", ID_CHECK: "업데이트 확인", ID_UPDATE: "업데이트 설치", ID_SAVE: "경로 적용", ID_EXIT: "완전히 종료"}
	text := labels[id]
	r := RECT{7, 0, dis.Rect.Right - dis.Rect.Left - 7, dis.Rect.Bottom - dis.Rect.Top}
	drawText(hdc, text, r, fg, g.font, DT_CENTER|DT_SINGLELINE|DT_VCENTER)
}
func (g *GUI) notify(l uintptr) {
	hdr := (*NMHDR)(unsafe.Pointer(l))
	if hdr.Code == LVN_GETDISPINFOW {
		di := (*NMLVDISPINFOW)(unsafe.Pointer(l))
		idx := int(di.Item.IItem)
		sub := int(di.Item.ISubItem)
		var text string
		if int(hdr.IDFrom) == ID_SESSLIST {
			if idx < 0 || idx >= len(g.visible) {
				return
			}
			s := g.visible[idx]
			switch sub {
			case 0:
				text = s.Title
			case 1:
				text = s.Project
			case 2:
				text = s.UpdatedAt.Local().Format("01-02 15:04")
			}
		}
		if int(hdr.IDFrom) == ID_JOBLIST {
			jobs := g.core.Pending()
			if idx < 0 || idx >= len(jobs) {
				return
			}
			j := jobs[idx]
			switch sub {
			case 0:
				text = j.State
			case 1:
				text = j.SessionTitle
			case 2:
				text = j.When.Local().Format("01-02 15:04")
			case 3:
				text = j.Preview
			}
		}
		if di.Item.Mask&1 != 0 && di.Item.Text != nil && di.Item.TextMax > 0 {
			dest := unsafe.Slice(di.Item.Text, int(di.Item.TextMax))
			src := syscall.StringToUTF16(text)
			copy(dest, src)
			dest[len(dest)-1] = 0
		}
		return
	}
	if hdr.Code == LVN_ITEMCHANGED && int(hdr.IDFrom) == ID_SESSLIST {
		idx := int(send(g.ctl(ID_SESSLIST), LVM_GETNEXTITEM, ^uintptr(0), 2))
		if idx != g.selected && idx >= 0 {
			g.selectSession(idx)
		}
	}
}
func mainWndProc(hwnd, msg, w, l uintptr) uintptr {
	if gui == nil {
		return call(u32, "DefWindowProcW", hwnd, msg, w, l)
	}
	g := gui
	switch uint32(msg) {
	case WM_CREATE:
		g.hwnd = hwnd
		g.initControls()
		g.asyncRefresh()
		return 0
	case WM_GETMINMAXINFO:
		m := (*MINMAXINFO)(unsafe.Pointer(l))
		m.MinTrackSize = POINT{X: 1180, Y: 690}
		return 0
	case WM_SIZE:
		g.W = int32(l & 0xFFFF)
		g.H = int32((l >> 16) & 0xFFFF)
		g.layout()
		return 0
	case WM_ERASEBKGND:
		return 1
	case WM_PAINT:
		var ps PAINTSTRUCT
		hdc := call(u32, "BeginPaint", hwnd, uintptr(unsafe.Pointer(&ps)))
		g.draw(hdc)
		call(u32, "EndPaint", hwnd, uintptr(unsafe.Pointer(&ps)))
		return 0
	case WM_DRAWITEM:
		g.drawButton((*DRAWITEMSTRUCT)(unsafe.Pointer(l)))
		return 1
	case WM_CTLCOLOREDIT, WM_CTLCOLORSTATIC:
		call(g32, "SetTextColor", w, uintptr(g.pal.text))
		call(g32, "SetBkColor", w, uintptr(g.pal.field))
		if uint32(msg) == WM_CTLCOLORSTATIC {
			return g.brushCard
		}
		return g.brushField
	case WM_COMMAND:
		id := int(w & 0xFFFF)
		if id == ID_SEARCH && (w>>16)&0xFFFF == EN_CHANGE {
			g.refreshSessions()
		} else if (w>>16)&0xFFFF == BN_CLICKED {
			g.handleButton(id)
		}
		return 0
	case WM_NOTIFY:
		g.notify(l)
		return 0
	case WM_TIMER:
		if w == 1 {
			if g.page == 1 {
				g.refreshJobs()
			}
			call(u32, "InvalidateRect", g.hwnd, 0, 1)
		}
		return 0
	case WM_APP + 1:
		g.refreshSessions()
		g.refreshJobs()
		call(u32, "InvalidateRect", hwnd, 0, 1)
		return 0
	case WM_APP + 2:
		g.core.mu.Lock()
		t := g.history
		g.core.mu.Unlock()
		settext(g.ctl(ID_HISTORY), t)
		return 0
	case WM_APP + 3:
		g.core.mu.Lock()
		target := g.updateTarget
		g.core.mu.Unlock()
		if target != "" {
			g.applyUpdate(target)
		} else {
			g.alert(g.core.Status())
		}
		return 0
	case WM_TRAY:
		if uint32(l) == 0x0202 || uint32(l) == 0x0203 {
			call(u32, "ShowWindow", hwnd, SW_RESTORE)
			call(u32, "SetForegroundWindow", hwnd)
		}
		return 0
	case WM_CLOSE:
		if !g.quitting {
			g.installTray()
			show(hwnd, false)
			return 0
		}
		fallthrough
	case WM_DESTROY:
		g.removeTray()
		g.core.Stop()
		call(u32, "PostQuitMessage", 0)
		return 0
	}
	return call(u32, "DefWindowProcW", hwnd, msg, w, l)
}
func (g *GUI) installTray() {
	if g.tray {
		return
	}
	var d NOTIFYICONDATAW
	d.Size = uint32(unsafe.Sizeof(d))
	d.Hwnd = g.hwnd
	d.UID = 1
	d.Flags = 0x1 | 0x2 | 0x4
	d.CallbackMessage = WM_TRAY
	d.Icon = g.icon
	copy(d.Tip[:], syscall.StringToUTF16("Codex Queue Studio · 예약 실행 중"))
	if call(shell, "Shell_NotifyIconW", 0, uintptr(unsafe.Pointer(&d))) != 0 {
		g.tray = true
	}
}
func (g *GUI) removeTray() {
	if !g.tray {
		return
	}
	d := NOTIFYICONDATAW{Size: uint32(unsafe.Sizeof(NOTIFYICONDATAW{})), Hwnd: g.hwnd, UID: 1}
	call(shell, "Shell_NotifyIconW", 2, uintptr(unsafe.Pointer(&d)))
	g.tray = false
}
func (g *GUI) applyUpdate(target string) {
	exe, err := os.Executable()
	if err != nil {
		g.alert(err.Error())
		return
	}
	helper := filepath.Join(filepath.Dir(exe), "CodexQueueUpdater.exe")
	if _, err = os.Stat(helper); err != nil {
		g.alert("업데이터가 없어 자동 업데이트를 진행할 수 없습니다. ZIP 릴리스를 다시 설치하세요.")
		return
	}
	cmd := exec.Command(helper, "--wait-pid", strconv.Itoa(os.Getpid()), "--target", exe, "--source", target)
	if err = cmd.Start(); err != nil {
		g.alert(err.Error())
		return
	}
	_ = cmd.Process.Release()
	g.quitting = true
	call(u32, "DestroyWindow", g.hwnd)
}
func loadIcon(root string) uintptr {
	_ = os.MkdirAll(root, 0700)
	p := filepath.Join(root, "studio.ico")
	_ = os.WriteFile(p, iconBytes, 0600)
	return call(u32, "LoadImageW", 0, strptr(p), IMAGE_ICON, 0, 0, LR_LOADFROMFILE|LR_DEFAULTSIZE)
}
func showFatal(msg string) {
	call(u32, "MessageBoxW", 0, strptr(msg), strptr("Codex Queue Studio"), 0x10)
}
func systemDark() bool {
	key, err := syscall.UTF16PtrFromString(`Software\Microsoft\Windows\CurrentVersion\Themes\Personalize`)
	if err != nil {
		return false
	}
	var h syscall.Handle
	if syscall.RegOpenKeyEx(syscall.HKEY_CURRENT_USER, key, 0, syscall.KEY_READ, &h) != nil {
		return false
	}
	defer syscall.RegCloseKey(h)
	name, _ := syscall.UTF16PtrFromString("AppsUseLightTheme")
	var typ, size uint32
	size = 4
	data := make([]byte, 4)
	if syscall.RegQueryValueEx(h, name, nil, &typ, &data[0], &size) != nil || size < 4 {
		return false
	}
	return data[0] == 0 && data[1] == 0 && data[2] == 0 && data[3] == 0
}
func main() {
	mutex := call(k32, "CreateMutexW", 0, 0, strptr("Local\\CodexQueueStudioNativeV3"))
	if mutex == 0 {
		showFatal("단일 실행 잠금 생성에 실패했습니다.")
		return
	}
	defer call(k32, "CloseHandle", mutex)
	if e := call(k32, "GetLastError"); e == 183 {
		other := call(u32, "FindWindowW", strptr("CodexQueueStudioNativeV3"), 0)
		if other != 0 {
			call(u32, "ShowWindow", other, SW_RESTORE)
			call(u32, "SetForegroundWindow", other)
		}
		return
	}
	root := appData()
	core, err := NewCore(root)
	if err != nil {
		showFatal(err.Error())
		return
	}
	if len(os.Args) > 1 && os.Args[1] == "--run-due" {
		_ = core.Refresh(context.Background())
		core.SendDue()
		return
	}
	g := &GUI{core: core, page: 0, W: 1350, H: 870, selected: -1}
	theme := core.store.Snapshot().Settings.Theme
	g.dark = theme == "dark" || (theme == "system" && systemDark())
	g.pal = palette(false)
	gui = g
	icc := INITCOMMONCONTROLSEX{Size: uint32(unsafe.Sizeof(INITCOMMONCONTROLSEX{})), ICC: 0x101}
	call(c32, "InitCommonControlsEx", uintptr(unsafe.Pointer(&icc)))
	g.font = call(g32, "CreateFontW", signed(-18), 0, 0, 0, 400, 0, 0, 0, 1, 0, 0, 5, 0, strptr("Segoe UI"))
	g.small = call(g32, "CreateFontW", signed(-16), 0, 0, 0, 400, 0, 0, 0, 1, 0, 0, 5, 0, strptr("Segoe UI"))
	g.bold = call(g32, "CreateFontW", signed(-24), 0, 0, 0, 650, 0, 0, 0, 1, 0, 0, 5, 0, strptr("Segoe UI Variable Display"))
	g.icon = loadIcon(root)
	cls := utf("CodexQueueStudioNativeV3")
	wc := WNDCLASSEX{Size: uint32(unsafe.Sizeof(WNDCLASSEX{})), WndProc: syscall.NewCallback(mainWndProc), Instance: call(k32, "GetModuleHandleW", 0), Cursor: call(u32, "LoadCursorW", 0, IDC_ARROW), ClassName: cls, Icon: g.icon, SmallIcon: g.icon}
	if call(u32, "RegisterClassExW", uintptr(unsafe.Pointer(&wc))) == 0 {
		showFatal("Windows 창 등록에 실패했습니다.")
		return
	}
	hwnd := createWindow("CodexQueueStudioNativeV3", "Codex Queue Studio", WS_OVERLAPPEDWINDOW|WS_VISIBLE, 0, 0, 0, 0, 0, 1350, 870)
	if hwnd == 0 {
		showFatal("네이티브 Windows 창 실행에 실패했습니다.")
		return
	}
	g.hwnd = hwnd
	send(hwnd, WM_SETICON, 1, g.icon)
	send(hwnd, WM_SETICON, 0, g.icon)
	core.Run()
	call(u32, "SetTimer", hwnd, 1, 4000, 0)
	go func() {
		for {
			ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
			defer cancel()
			upd, _ := core.CheckUpdate(ctx)
			if upd != nil {
				core.setStatus("새 버전이 준비되었습니다: " + upd.Tag)
				call(u32, "PostMessageW", hwnd, WM_APP+1, 0, 0)
			}
			select {
			case <-core.stop:
				return
			case <-time.After(6 * time.Hour):
			}
		}
	}()
	var msg MSG
	for call(u32, "GetMessageW", uintptr(unsafe.Pointer(&msg)), 0, 0, 0) != 0 {
		call(u32, "TranslateMessage", uintptr(unsafe.Pointer(&msg)))
		call(u32, "DispatchMessageW", uintptr(unsafe.Pointer(&msg)))
	}
}
