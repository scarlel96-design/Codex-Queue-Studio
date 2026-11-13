//go:build windows

package main

import (
	"context"
	_ "embed"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"reflect"
	"strconv"
	"strings"
	"sync"
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
	LVS_EX_LABELTIP              = 0x4000
	LVM_SETCOLUMNWIDTH           = LVM_FIRST + 30
	EM_SETCUEBANNER              = 0x1501
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
	ID_RELEASE_NOTES             = 124
	ID_RELAY_PAGE                = 125
	ID_TAB_CODEX                 = 126
	ID_TAB_GPT                   = 127
	ID_GPT_LIST                  = 128
	ID_GPT_NEW                   = 129
	ID_GPT_HISTORY               = 130
	EM_SETREADONLY               = 0x00CF
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
		return Palette{rgb(14, 17, 24), rgb(22, 26, 35), rgb(32, 37, 47), rgb(41, 47, 59), rgb(244, 246, 250), rgb(167, 177, 194), rgb(91, 157, 248), rgb(255, 255, 255), rgb(61, 69, 86)}
	}
	return Palette{rgb(246, 247, 250), rgb(237, 240, 246), rgb(255, 255, 255), rgb(247, 249, 252), rgb(34, 43, 59), rgb(110, 122, 144), rgb(33, 103, 213), rgb(255, 255, 255), rgb(222, 228, 238)}
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
	call(g32, "RoundRect", hdc, uintptr(r.Left), uintptr(r.Top), uintptr(r.Right), uintptr(r.Bottom), 22, 22)
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
	heading, micro                 uintptr
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
	updateMu                       sync.Mutex
	updateView                     updateDisplay
	healthyTicks                   int
	refreshing                    atomic.Bool
	lastStatus                    string
	explorerTab                   int
	chats                         []internal.ChatSession
	visibleChats                  []internal.ChatSession
	selectedChat                  int
	chatSeq                       atomic.Uint64
	chatMu                        sync.Mutex
	chatHistory                   string
	tabMotionFrom,tabMotionTo      float64
	tabMotionPosition             float64
	tabMotionStarted              time.Time
	tabMotionActive               bool
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
	send(h, LVM_SETEXTENDEDLISTVIEWSTYLE, 0, LVS_EX_FULLROWSELECT|LVS_EX_DOUBLEBUFFER|LVS_EX_LABELTIP)
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
	}{{ID_SESSIONS, "세션 탐색기"}, {ID_JOBS, "예약 관리"}, {ID_RELAY_PAGE, "스마트 릴레이"}, {ID_SETTINGS, "환경 설정"}, {ID_THEME, "테마 전환"}, {ID_REFRESH, "새로고침"}, {ID_SENDNOW, "지금 전송"}, {ID_SCHEDULE, "예약 등록"}, {ID_CANCEL, "예약 취소"}, {ID_REACTIVATE, "선택 예약 재개"}, {ID_ENGINE, "Codex 엔진 설치"}, {ID_CHECK, "업데이트 확인"}, {ID_UPDATE, "업데이트 설치"}, {ID_SAVE, "경로 적용"}, {ID_EXIT, "완전히 종료"}, {ID_TAB_CODEX, "Codex 작업"}, {ID_TAB_GPT, "GPT 채팅"}, {ID_GPT_NEW, "새 GPT API 대화"}} {
		g.button(b.id, b.title)
	}
	g.edit(ID_SEARCH, false)
	send(g.ctl(ID_SEARCH), EM_SETCUEBANNER, 0, strptr("Codex 작업 세션 검색"))
	g.edit(ID_HISTORY, true)
	g.edit(ID_GPT_HISTORY, true)
	send(g.ctl(ID_GPT_HISTORY),EM_SETREADONLY,1,0)
	settext(g.ctl(ID_GPT_HISTORY),"GPT API 대화를 선택하면 기록이 표시됩니다.")
	g.edit(ID_MESSAGE, true)
	g.add(ID_DATE, "SysDateTimePick32", "", WS_TABSTOP)
	g.add(ID_TIME, "SysDateTimePick32", "", WS_TABSTOP|DTS_TIMEFORMAT)
	send(g.ctl(ID_DATE), DTM_SETFORMATW, 0, strptr("yyyy-MM-dd"))
	send(g.ctl(ID_TIME), DTM_SETFORMATW, 0, strptr("HH:mm"))
	g.edit(ID_HOMEPATH, false)
	g.edit(ID_RELEASE_NOTES, true)
	send(g.ctl(ID_RELEASE_NOTES), EM_SETREADONLY, 1, 0)
	settext(g.ctl(ID_RELEASE_NOTES), "업데이트를 확인하면 이 영역에 공식 릴리스 변경 내역이 표시됩니다.")
	setFont(g.ctl(ID_RELEASE_NOTES), g.font)
	g.list(ID_SESSLIST, []struct {
		title string
		width int32
	}{{"세션 제목", 250}, {"프로젝트", 112}, {"출처", 90}, {"수정일", 95}})
	g.list(ID_GPT_LIST, []struct {
		title string
		width int32
	}{{"대화 이름",300},{"제공자",75},{"수정일",110}})
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
	corners := uint32(2) // DWMWCP_ROUND; Windows 11 preserves native caption buttons.
	call(dwm, "DwmSetWindowAttribute", g.hwnd, 33, uintptr(unsafe.Pointer(&corners)), unsafe.Sizeof(corners))
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
	for i, id := range []int{ID_SESSIONS, ID_JOBS, ID_RELAY_PAGE, ID_SETTINGS} {
		move(g.ctl(id), 14, 112+int32(i)*51, 180, 40)
	}
	move(g.ctl(ID_THEME), 14, h-130, 180, 38)
	move(g.ctl(ID_EXIT), 14, h-78, 180, 38)
	all := []int{ID_SEARCH, ID_REFRESH, ID_SESSLIST, ID_HISTORY, ID_MESSAGE, ID_DATE, ID_TIME, ID_SENDNOW, ID_SCHEDULE, ID_JOBLIST, ID_CANCEL, ID_REACTIVATE, ID_ENGINE, ID_CHECK, ID_UPDATE, ID_HOMEPATH, ID_SAVE, ID_RELEASE_NOTES, ID_TAB_CODEX, ID_TAB_GPT, ID_GPT_LIST, ID_GPT_NEW, ID_GPT_HISTORY}
	for _, id := range all {
		show(g.ctl(id), false)
	}
	switch g.page {
	case 0:
		left := area * 51 / 100
		if left < 330 {
			left = 330
		}
		right := area - left - 18
		move(g.ctl(ID_TAB_CODEX), x+4, 120, 145, 34)
		move(g.ctl(ID_TAB_GPT), x+154, 120, 145, 34)
		move(g.ctl(ID_SEARCH), x, 169, left-126, 35)
		move(g.ctl(ID_REFRESH), x+left-116, 169, 116, 35)
		move(g.ctl(ID_SESSLIST), x, 218, left, h-256)
		move(g.ctl(ID_GPT_LIST), x, 218, left, h-256)
		for i,ratio:=range []int32{64,16,20}{send(g.ctl(ID_GPT_LIST),LVM_SETCOLUMNWIDTH,uintptr(i),uintptr(left*ratio/100))}
		for i, ratio := range []int32{45, 21, 16, 18} {
			send(g.ctl(ID_SESSLIST), LVM_SETCOLUMNWIDTH, uintptr(i), uintptr(left*ratio/100))
		}
		xr := x + left + 18
		move(g.ctl(ID_HISTORY), xr, 267, right, h-569)
		move(g.ctl(ID_GPT_HISTORY),xr,295,right,h-439)
		move(g.ctl(ID_GPT_NEW),xr,h-103,right,39)
		move(g.ctl(ID_MESSAGE), xr, h-251, right, 117)
		move(g.ctl(ID_DATE), xr, h-117, right-365, 36)
		move(g.ctl(ID_TIME), xr+right-356, h-117, 95, 36)
		move(g.ctl(ID_SENDNOW), xr+right-251, h-117, 119, 36)
		move(g.ctl(ID_SCHEDULE), xr+right-124, h-117, 124, 36)
		for _,id:=range []int{ID_SEARCH,ID_REFRESH,ID_TAB_CODEX,ID_TAB_GPT}{show(g.ctl(id),true)}
		if g.explorerTab==0 {
			for _,id:=range []int{ID_SESSLIST,ID_HISTORY,ID_MESSAGE,ID_DATE,ID_TIME,ID_SENDNOW,ID_SCHEDULE}{show(g.ctl(id),true)}
		} else {
			for _,id:=range []int{ID_GPT_LIST,ID_GPT_HISTORY,ID_GPT_NEW}{show(g.ctl(id),true)}
		}
	case 1:
		move(g.ctl(ID_JOBLIST), x, 138, area, h-247)
		move(g.ctl(ID_CANCEL), x+area-155, h-89, 155, 40)
		move(g.ctl(ID_REACTIVATE), x+area-326, h-89, 160, 40)
		show(g.ctl(ID_JOBLIST), true)
		show(g.ctl(ID_CANCEL), true)
		show(g.ctl(ID_REACTIVATE), true)
	case 2:
		// Separate the Codex data path from the software-update center.
		margin := int32(22)
		move(g.ctl(ID_HOMEPATH), x+margin, 178, area-238, 40)
		move(g.ctl(ID_SAVE), x+area-193, 178, 171, 40)
		move(g.ctl(ID_ENGINE), x+margin, 273, 213, 39)
		move(g.ctl(ID_CHECK), x+margin, 377, 177, 39)
		move(g.ctl(ID_UPDATE), x+212, 377, 185, 39)
		notesHeight := h - int32(600)
		if notesHeight < 75 {
			notesHeight = 75
		}
		move(g.ctl(ID_RELEASE_NOTES), x+margin, 478, area-2*margin, notesHeight)
		for _, id := range []int{ID_HOMEPATH, ID_SAVE, ID_ENGINE, ID_CHECK, ID_UPDATE, ID_RELEASE_NOTES} {
			show(g.ctl(id), true)
		}
	}
}
func (g *GUI) setPage(p int) {
	if p == 2 && !g.updateSnapshot().Busy {
		if latest := g.core.UpdateAvailable(); latest != nil {
			g.setUpdateView(updateDisplay{Phase: "설치 가능한 버전  ·  " + latest.Tag, Notes: prettyReleaseNotes(latest), Percent: 8})
		}
	}
	g.page = p
	g.layout()
	if p == 1 {
		g.refreshJobs()
	}
	call(u32, "InvalidateRect", g.hwnd, 0, 1)
}
func (g *GUI) refreshSessions() {
	oldID := ""
	if g.selected >= 0 && g.selected < len(g.visible) { oldID = g.visible[g.selected].ID }
	search := strings.ToLower(strings.TrimSpace(gettext(g.ctl(ID_SEARCH))))
	all := g.core.Snapshot()
	oldCount := len(g.sessions)
	next := make([]internal.Session,0,len(all))
	for _,v := range all {
		if search=="" || strings.Contains(strings.ToLower(v.Title+" "+v.Project+" "+v.ID+" "+v.Path),search) {
			next=append(next,v)
			if len(next)>=5000 { break }
		}
	}
	g.sessions=all
	if reflect.DeepEqual(g.visible,next) {
		if oldCount!=len(all) { g.invalidateSummary() }
		return
	}
	old:=g.visible
	g.visible=next
	h:=g.ctl(ID_SESSLIST)
	if len(old)!=len(next) {
		send(h,LVM_SETITEMCOUNT,uintptr(len(next)),3) // LVM_SETITEMCOUNT with NOINVALIDATEALL | NOSCROLL
		call(u32,"InvalidateRect",h,0,0)
	} else {
		start,last:=-1,-1
		for i:=range next {
			if old[i]==next[i] {
				if start>=0 {send(h,LVM_FIRST+21,uintptr(start),uintptr(last));start=-1}
			} else {
				if start<0 {start=i}
				last=i
			}
		}
		if start>=0 {send(h,LVM_FIRST+21,uintptr(start),uintptr(last))}
	}
	prevSelected:=g.selected
	g.selected=-1
	for i,v:=range next { if v.ID==oldID {g.selected=i;break} }
	if prevSelected!=g.selected {
		item:=LVITEMW{StateMask:3}
		send(h,LVM_FIRST+43,^uintptr(0),uintptr(unsafe.Pointer(&item)))
		if g.selected>=0 {item.State=3;send(h,LVM_FIRST+43,uintptr(g.selected),uintptr(unsafe.Pointer(&item)))}
	}
	g.invalidateSummary()
}
func (g *GUI) invalidateSummary() {
	r:=RECT{Left:230,Top:204,Right:g.W-28,Bottom:239}
	call(u32,"InvalidateRect",g.hwnd,uintptr(unsafe.Pointer(&r)),0)
}
func (g *GUI) refreshStatusRail() {
	status:=g.core.Status()
	if status==g.lastStatus { return }
	g.lastStatus=status
	r:=RECT{Left:233,Top:g.H-40,Right:g.W-24,Bottom:g.H}
	call(u32,"InvalidateRect",g.hwnd,uintptr(unsafe.Pointer(&r)),0)
}
func (g *GUI) refreshJobs() {
	next:=g.core.Pending()
	if reflect.DeepEqual(g.jobs,next) {return}
	old:=g.jobs
	g.jobs=next
	h:=g.ctl(ID_JOBLIST)
	if len(old)!=len(next) {
		send(h,LVM_SETITEMCOUNT,uintptr(len(next)),3)
		call(u32,"InvalidateRect",h,0,0)
		return
	}
	for i:=range next {if old[i]!=next[i] {send(h,LVM_FIRST+21,uintptr(i),uintptr(i))}}
}
func (g *GUI) asyncRefresh() {
	if !g.refreshing.CompareAndSwap(false,true) {return}
	g.core.setStatus("세션 검색 중")
	go func() {
		defer g.refreshing.Store(false)
		ctx,cancel:=context.WithTimeout(context.Background(),30*time.Second)
		defer cancel()
		if err:=g.core.RefreshLocal();err==nil {
			call(u32,"PostMessageW",g.hwnd,WM_APP+1,0,0)
			_=g.core.MergeRemote(ctx)
		}
		call(u32,"PostMessageW",g.hwnd,WM_APP+1,0,0)
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
		output.WriteString("제목: " + s.Title + "\r\n제목 정보: " + s.TitleSource + "\r\n출처: " + s.Source + "\r\n세션: " + s.ID + "\r\n프로젝트: " + s.Path + "\r\n\r\n")
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
	case ID_RELAY_PAGE:
		openRelayWindow(g)
	case ID_THEME:
		g.dark = !g.dark
		chosen := "light"
		if g.dark {
			chosen = "dark"
		}
		_ = g.core.store.Update(func(st *internal.State) error { st.Settings.Theme = chosen; return nil })
		g.updateColors()
	case ID_TAB_CODEX:
		g.setExplorerTab(0)
	case ID_TAB_GPT:
		g.setExplorerTab(1)
	case ID_GPT_NEW:
		g.createGPTChat()
	case ID_REFRESH:
		if g.explorerTab==1 {g.refreshChatSessions();if g.selectedChat>=0 {g.selectGPTChat(g.selectedChat)}} else {g.asyncRefresh()}
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
		state := g.updateSnapshot()
		if state.Busy {
			return
		}
		g.setUpdateView(updateDisplay{Busy: true, Phase: "GitHub 릴리스 확인 중", Percent: 3, Notes: "최신 배포 파일과 무결성 정보를 확인하고 있습니다."})
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
			defer cancel()
			upd, err := g.core.CheckUpdate(ctx)
			if err != nil {
				g.updateFailure(err)
				g.core.setStatus("업데이트 조회 오류: " + err.Error())
			} else if upd == nil {
				g.setUpdateView(updateDisplay{Phase: "최신 상태입니다", Notes: "새 릴리스가 없습니다. GitHub에 배포 파일이 게시되면 여기에서 변경 내역을 확인할 수 있습니다.", Percent: 100})
				g.core.setStatus("현재 버전 " + AppVersion + " · 최신 상태")
			} else {
				g.setUpdateView(updateDisplay{Phase: "업데이트 준비 완료  ·  " + upd.Tag, Notes: prettyReleaseNotes(upd), Percent: 8})
				g.core.setStatus("새 버전 " + upd.Tag + "의 공식 변경 내역을 확인하세요")
			}
		}()
	case ID_UPDATE:
		upd := g.core.UpdateAvailable()
		if upd == nil {
			g.alert("먼저 업데이트를 확인하세요.")
			return
		}
		if g.updateSnapshot().Busy {
			return
		}
		if err := g.core.CanInstallUpdate(); err != nil {
			g.alert(err.Error())
			return
		}
		if !g.ask("릴리스 변경 내역을 확인하셨나요?\n" + upd.Tag + " 설치를 시작합니다.") {
			return
		}
		g.setUpdateView(updateDisplay{Busy: true, Phase: "업데이트 다운로드 준비", Percent: 0, Notes: prettyReleaseNotes(upd), Total: upd.Size})
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 6*time.Minute)
			defer cancel()
			path, err := g.core.DownloadUpdateProgress(ctx, *upd, g.updateProgress)
			if err != nil {
				g.updateFailure(err)
				g.core.setStatus("업데이트 실패: " + err.Error())
				return
			}
			g.core.mu.Lock()
			g.updateTarget = path
			g.core.mu.Unlock()
			g.updateProgress("설치 프로그램 시작 및 안전한 재시작 준비", 94, upd.Size, upd.Size)
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
	fill(hdc,RECT{207,0,208,g.H},p.border)
	fill(hdc, RECT{17, 101, 189, 102}, p.border)
	roundBox(hdc, RECT{16, g.H - 166, 191, g.H - 152}, p.border)
	roundBox(hdc, RECT{19, 26, 58, 66}, p.border)
	roundBox(hdc, RECT{20, 24, 56, 59}, p.accent)
	drawText(hdc, "C", RECT{22, 23, 54, 60}, p.accentText, g.bold, DT_CENTER|DT_VCENTER|DT_SINGLELINE)
	drawText(hdc, "Codex Queue", RECT{67, 27, 204, 58}, p.text, g.font, DT_SINGLELINE|DT_VCENTER)
	drawText(hdc, "STUDIO  ·  3.4.0", RECT{21, 70, 189, 94}, p.muted, g.micro, DT_SINGLELINE)
	title := "세션 탐색기"
	sub := "Codex 작업과 등록된 GPT API 대화를 빠르게 관리합니다"
	if g.page == 1 {
		title = "예약 작업"
		sub = "예약 실행 상태와 실패 이력을 관리합니다"
	} else if g.page == 2 {
		title = "환경 설정"
		sub = "독립 Codex 엔진 · 업데이트 · 세션 경로"
	}
	drawText(hdc, title, RECT{234, 25, g.W - 30, 67}, p.text, g.heading, DT_SINGLELINE)
	drawText(hdc, sub, RECT{234, 77, g.W - 30, 109}, p.muted, g.font, DT_SINGLELINE)
	status := g.core.Status()
	// Status rail separates operational feedback from the main content.
	fill(hdc, RECT{233, g.H - 39, g.W - 25, g.H - 38}, p.border)
	roundBox(hdc, RECT{236, g.H - 26, 245, g.H - 17}, rgb(35, 171, 125))
	drawText(hdc, status, RECT{253, g.H - 31, g.W - 24, g.H - 8}, p.muted, g.small, DT_SINGLELINE|DT_END_ELLIPSIS)
	switch g.page {
	case 0:
		x := int32(233)
		area := g.W - x - 24
		left := area * 51 / 100
		if left < 330 {
			left = 330
		}
		xr := x + left + 18
		right := area - left - 18
		roundBox(hdc,RECT{x-5,213,x+left+7,g.H-33},p.border)
		roundBox(hdc,RECT{xr-5,213,xr+right+7,g.H-33},p.border)
		roundBox(hdc,RECT{x-6,210,x+left+6,g.H-36},p.card)
		roundBox(hdc,RECT{xr-6,210,xr+right+6,g.H-36},p.card)
		g.drawExplorerTabs(hdc)
		drawText(hdc,g.describeExplorer(),RECT{x+6,207,x+left,230},p.muted,g.small,DT_SINGLELINE)
		if g.explorerTab==1 {g.drawGPTInspector(hdc,xr,right);break}
		heading := "세션을 선택하세요"
		if s, ok := g.selectedSession(); ok {
			heading = s.Title
		}
		drawText(hdc, heading, RECT{xr + 7, 228, xr + right - 8, 256}, p.text, g.bold, DT_SINGLELINE|DT_END_ELLIPSIS)
		drawText(hdc, "세션 대화 기록  ·  저장된 제목 우선", RECT{xr + 4, 262, xr + right, 285}, p.muted, g.small, DT_SINGLELINE)
		drawText(hdc, "예약할 메시지", RECT{xr + 4, g.H - 276, xr + right, g.H - 250}, p.text, g.font, DT_SINGLELINE)
		drawText(hdc, "예약 날짜와 시간", RECT{xr + 4, g.H - 141, xr + right, g.H - 114}, p.muted, g.small, DT_SINGLELINE)
	case 1:
		roundBox(hdc, RECT{226, 123, g.W - 16, g.H - 115}, p.card)
		drawText(hdc, "취소는 대기 상태에서만 가능합니다. 실행 상태가 불명확하면 자동 재전송하지 않습니다.", RECT{245, g.H - 111, g.W - 190, g.H - 84}, p.muted, g.small, DT_SINGLELINE)
	case 2:
		g.drawUpdateCenter(hdc)
	}
}
func (g *GUI) drawButton(dis *DRAWITEMSTRUCT) {
	p := g.pal
	hdc := dis.Hdc
	id := int(dis.CtlID)
	selected := (id == ID_SESSIONS && g.page == 0) || (id == ID_JOBS && g.page == 1) || (id == ID_SETTINGS && g.page == 2) || (id == ID_RELAY_PAGE && g.page == 3) || (id == ID_TAB_CODEX && g.explorerTab==0) || (id == ID_TAB_GPT && g.explorerTab==1)
	bg := p.card
	fg := p.text
	if id == ID_SCHEDULE || id == ID_SENDNOW || id == ID_UPDATE || id==ID_GPT_NEW {
		bg = p.accent
		fg = p.accentText
	}
	if selected {
		bg = p.field
		fg = p.accent
	}
	if id==ID_TAB_CODEX||id==ID_TAB_GPT {
		bg=p.field
		fg=p.muted
		if selected {bg=p.card;fg=p.accent}
	}
	if id == ID_EXIT {
		bg = p.side
		fg = p.muted
	}
	if dis.ItemState&0x1 != 0 {
		bg = p.border
		fg = p.text
	}
	if dis.ItemState&0x4 != 0 { // ODS_DISABLED
		bg = p.field
		fg = p.muted
	}
	roundBox(hdc, RECT{2, 2, dis.Rect.Right - dis.Rect.Left - 2, dis.Rect.Bottom - dis.Rect.Top - 2}, bg)
	if selected && id!=ID_TAB_CODEX && id!=ID_TAB_GPT {
		roundBox(hdc, RECT{4, 8, 8, dis.Rect.Bottom - dis.Rect.Top - 8}, p.accent)
	}
	labels := map[int]string{ID_SESSIONS: "세션 탐색기", ID_JOBS: "예약 관리", ID_SETTINGS: "환경 설정", ID_RELAY_PAGE: "스마트 릴레이", ID_TAB_CODEX:"Codex 작업",ID_TAB_GPT:"GPT 채팅",ID_GPT_NEW:"새 GPT API 대화", ID_THEME: "◐   테마 전환", ID_REFRESH: "새로고침", ID_SENDNOW: "지금 전송", ID_SCHEDULE: "예약 등록", ID_CANCEL: "예약 취소", ID_REACTIVATE: "선택 예약 재개", ID_ENGINE: "Codex 엔진 설치", ID_CHECK: "업데이트 확인", ID_UPDATE: "업데이트 설치", ID_SAVE: "경로 적용", ID_EXIT: "완전히 종료"}
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
				text = s.Source
			case 3:
				text = s.UpdatedAt.Local().Format("01-02 15:04")
			}
		}
		if int(hdr.IDFrom) == ID_JOBLIST {
			jobs := g.jobs
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
		if !hasFlag("--safe-mode") && !hasFlag("--ui-smoke-test") {
			g.asyncRefresh()
		} else if hasFlag("--safe-mode") {
			g.core.setStatus("안전 모드: 자동 세션 검색을 건너뛰었습니다")
		}
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
	case WM_SETTINGCHANGE:
		if g.core.store.Snapshot().Settings.Theme == "system" {
			dark := systemDark()
			if g.dark != dark {
				g.dark = dark
				g.updateColors()
			}
		}
		return 0
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
		return g.brushField
	case WM_COMMAND:
		id := int(w & 0xFFFF)
		if id == ID_SEARCH && (w>>16)&0xFFFF == EN_CHANGE {
			if g.explorerTab==0 {g.refreshSessions()}else{g.refreshChatSessions()}
		} else if (w>>16)&0xFFFF == BN_CLICKED {
			g.handleButton(id)
		}
		return 0
	case WM_NOTIFY:
		if !g.handleGPTNotify(l){g.notify(l)}
		return 0
	case WM_TIMER:
		if w==explorerAnimationTimer {g.tickTabAnimation();return 0}
		if w == 9902 && !hasFlag("--ui-smoke-test") {
			g.healthyTicks++
			if g.healthyTicks >= 6 {
				call(u32, "KillTimer", hwnd, 9902)
				path, token := updateHealthArgs()
				if path != "" && token != "" {
					// The marker is written by a timer running on the Win32 UI thread.
					// A created window alone is not evidence that messages are processed.
					_ = os.WriteFile(path, []byte(token+"\n"+strconv.Itoa(os.Getpid())), 0600)
				}
			}
			return 0
		}
		if w == 9901 && hasFlag("--ui-smoke-test") {
			// Exercise the native tab controls and relay dialog without real traffic.
			g.setExplorerTab(1)
			if g.explorerTab!=1 || g.ctl(ID_GPT_LIST)==0 {os.Exit(21)}
			g.setExplorerTab(0)
			if g.explorerTab!=0 || g.ctl(ID_SESSLIST)==0 {os.Exit(22)}
			openRelayWindow(g)
			if activeRelayWindow==nil||activeRelayWindow.hwnd==0 {os.Exit(23)}
			if activeRelayWindow.ctl(relayWork)==0||activeRelayWindow.ctl(relayModel)==0 {os.Exit(24)}
			call(u32,"DestroyWindow",activeRelayWindow.hwnd)
			g.quitting=true
			call(u32,"DestroyWindow",hwnd)
			return 0
		}
		if w == 1 {
			if g.page == 1 { g.refreshJobs() }
			g.refreshStatusRail()
		}
		return 0
	case WM_APP + 11:
		g.refreshChatSessions()
		g.refreshStatusRail()
		return 0
	case WM_APP + 12:
		g.refreshGPTHistoryView()
		return 0
	case WM_APP + 1:
		g.refreshSessions()
		g.refreshJobs()
		g.refreshStatusRail()
		return 0
	case WM_APP + 2:
		g.core.mu.Lock()
		t := g.history
		g.core.mu.Unlock()
		settext(g.ctl(ID_HISTORY), t)
		return 0
	case WM_APP + 4:
		state := g.updateSnapshot()
		if state.Notes != "" && gettext(g.ctl(ID_RELEASE_NOTES)) != state.Notes {
			settext(g.ctl(ID_RELEASE_NOTES), state.Notes)
		}
		enabled := uintptr(1)
		if state.Busy {
			enabled = 0
		}
		call(u32, "EnableWindow", g.ctl(ID_CHECK), enabled)
		if g.core.UpdateAvailable() == nil || state.Busy {
			enabled = 0
		}
		call(u32, "EnableWindow", g.ctl(ID_UPDATE), enabled)
		call(u32, "InvalidateRect", hwnd, 0, 1)
		return 0
	case WM_APP + 3:
		g.core.mu.Lock()
		target := g.updateTarget
		g.core.mu.Unlock()
		if target != "" {
			g.applyUpdate(target)
		} else {
			g.setUpdateView(updateDisplay{Phase: g.core.Status(), Percent: 0})
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
		if activeRelayWindow!=nil && activeRelayWindow.hwnd!=0 {call(u32,"DestroyWindow",activeRelayWindow.hwnd)}
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
	helper := filepath.Join(target, "CodexQueueUpdater.exe")
	if _, err = os.Stat(helper); err != nil {
		g.alert("업데이터가 없어 자동 업데이트를 진행할 수 없습니다. ZIP 릴리스를 다시 설치하세요.")
		return
	}
	g.updateProgress("업데이터 실행 · 화면 전환 준비", 96, 0, 0)
	cmd := exec.Command(helper, "--wait-pid", strconv.Itoa(os.Getpid()), "--target", exe, "--source", target)
	if err = cmd.Start(); err != nil {
		g.updateFailure(err)
		return
	}
	_ = cmd.Process.Release()
	g.quitting = true
	call(u32, "DestroyWindow", g.hwnd)
}
func loadIcon(root string) uintptr {
	// Use the executable resource first. It also supplies Explorer/taskbar artwork.
	h := call(u32, "LoadImageW", call(k32, "GetModuleHandleW", 0), 1, IMAGE_ICON, 0, 0, LR_DEFAULTSIZE)
	if h != 0 {
		return h
	}
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
func updateHealthArgs() (path, token string) {
	for i := 1; i+1 < len(os.Args); i++ {
		switch os.Args[i] {
		case "--post-update-health-file":
			path = os.Args[i+1]
		case "--post-update-health-nonce":
			token = os.Args[i+1]
		}
	}
	return
}
func hasFlag(flag string) bool {
	for _, arg := range os.Args[1:] {
		if arg == flag {
			return true
		}
	}
	return false
}

func main() {
	// Win32 windows and their message loop must run on the same OS thread.
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
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
	g := &GUI{core: core, page: 0, W: 1350, H: 870, selected: -1, selectedChat:-1}
	theme := core.store.Snapshot().Settings.Theme
	g.dark = theme == "dark" || (theme == "system" && systemDark())
	g.pal = palette(g.dark)
	gui = g
	icc := INITCOMMONCONTROLSEX{Size: uint32(unsafe.Sizeof(INITCOMMONCONTROLSEX{})), ICC: 0x101}
	call(c32, "InitCommonControlsEx", uintptr(unsafe.Pointer(&icc)))
	g.font = call(g32, "CreateFontW", signed(-17), 0, 0, 0, 400, 0, 0, 0, 1, 0, 0, 5, 0, strptr("Segoe UI Variable Text"))
	g.small = call(g32, "CreateFontW", signed(-15), 0, 0, 0, 400, 0, 0, 0, 1, 0, 0, 5, 0, strptr("Segoe UI Variable Text"))
	g.bold = call(g32, "CreateFontW", signed(-22), 0, 0, 0, 650, 0, 0, 0, 1, 0, 0, 5, 0, strptr("Segoe UI Variable Display"))
	g.heading = call(g32, "CreateFontW", signed(-30), 0, 0, 0, 640, 0, 0, 0, 1, 0, 0, 5, 0, strptr("Segoe UI Variable Display"))
	g.micro = call(g32, "CreateFontW", signed(-13), 0, 0, 0, 570, 0, 0, 0, 1, 0, 0, 5, 0, strptr("Segoe UI Variable Text"))
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
	if hasFlag("--ui-smoke-test") {
		// The smoke test checks a live Win32 window without scanning sessions or sending messages.
		call(u32, "SetTimer", hwnd, 9901, 350, 0)
	} else {
		core.Run()
		call(u32, "SetTimer", hwnd, 9902, 600, 0)
		call(u32, "SetTimer", hwnd, 1, 4000, 0)
	}
	if !hasFlag("--ui-smoke-test") {
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
	}
	var msg MSG
	for call(u32, "GetMessageW", uintptr(unsafe.Pointer(&msg)), 0, 0, 0) != 0 {
		call(u32, "TranslateMessage", uintptr(unsafe.Pointer(&msg)))
		call(u32, "DispatchMessageW", uintptr(unsafe.Pointer(&msg)))
	}
}
