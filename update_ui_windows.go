//go:build windows

package main

import (
	"fmt"
	"strings"
	"time"
)

// updateDisplay contains UI-only state. It never stores the message payload.
type updateDisplay struct {
	Phase    string
	Notes    string
	Error    string
	Percent  int
	Received int64
	Total    int64
	Busy     bool
}

func (g *GUI) setUpdateView(next updateDisplay) {
	g.updateMu.Lock()
	g.updateView = next
	g.updateMu.Unlock()
	if g.hwnd != 0 {
		call(u32, "PostMessageW", g.hwnd, WM_APP+4, 0, 0)
	}
}
func (g *GUI) updateProgress(stage string, pct int, got, total int64) {
	g.updateMu.Lock()
	state := g.updateView
	state.Phase = stage
	if pct < 0 {
		pct = 0
	}
	if pct > 100 {
		pct = 100
	}
	state.Percent = pct
	state.Received = got
	state.Total = total
	g.updateView = state
	g.updateMu.Unlock()
	if g.hwnd != 0 {
		call(u32, "PostMessageW", g.hwnd, WM_APP+4, 0, 0)
	}
}
func (g *GUI) updateFailure(err error) {
	if err == nil {
		return
	}
	g.updateMu.Lock()
	state := g.updateView
	state.Phase = "작업을 완료하지 못했습니다"
	state.Error = err.Error()
	state.Busy = false
	g.updateView = state
	g.updateMu.Unlock()
	if g.hwnd != 0 {
		call(u32, "PostMessageW", g.hwnd, WM_APP+4, 0, 0)
	}
}
func (g *GUI) updateSnapshot() updateDisplay {
	g.updateMu.Lock()
	defer g.updateMu.Unlock()
	return g.updateView
}
func pill(hdc uintptr, r RECT, bg, fg uint32, text string, font uintptr) {
	roundBox(hdc, r, bg)
	drawText(hdc, text, r, fg, font, DT_SINGLELINE|DT_VCENTER|DT_CENTER)
}
func (g *GUI) drawUpdateCenter(hdc uintptr) {
	p := g.pal
	x := int32(233)
	width := g.W - x - 24
	if width < 660 {
		width = 660
	}
	// Connection preferences.
	roundBox(hdc, RECT{x - 5, 123, x + width + 5, 244}, p.card)
	drawText(hdc, "01  데이터 연결", RECT{x + 22, 139, x + width - 20, 165}, p.text, g.bold, DT_SINGLELINE)
	drawText(hdc, "Codex 세션을 읽을 로컬 경로", RECT{x + 23, 166, x + width - 22, 184}, p.muted, g.micro, DT_SINGLELINE)
	// Engine availability.
	roundBox(hdc, RECT{x - 5, 253, x + width + 5, 328}, p.card)
	drawText(hdc, "02  실행 엔진", RECT{x + 246, 266, x + width - 20, 289}, p.text, g.bold, DT_SINGLELINE)
	engine := "엔진 미설치  ·  Codex 엔진 설치를 선택하세요"
	if g.core.EnginePresent() {
		engine = "Codex 실행 엔진 연결됨"
	}
	drawText(hdc, engine, RECT{x + 246, 296, x + width - 20, 318}, p.muted, g.small, DT_SINGLELINE|DT_END_ELLIPSIS)
	// Native update center.
	roundBox(hdc, RECT{x - 5, 336, x + width + 5, g.H - 53}, p.card)
	drawText(hdc, "03  소프트웨어 업데이트", RECT{x + 22, 348, x + width - 22, 375}, p.text, g.bold, DT_SINGLELINE)
	u := g.core.UpdateAvailable()
	version := "현재  " + AppVersion
	if u != nil {
		version += "   →   " + u.Tag
	}
	drawText(hdc, version, RECT{x + 414, 349, x + width - 24, 373}, p.muted, g.small, DT_SINGLELINE|DT_END_ELLIPSIS)
	if u != nil {
		title := u.Name
		if strings.TrimSpace(title) == "" {
			title = "Codex Queue Studio " + u.Tag
		}
		drawText(hdc, title, RECT{x + 23, 435, x + width - 22, 458}, p.text, g.font, DT_SINGLELINE|DT_END_ELLIPSIS)
	} else {
		drawText(hdc, "릴리스의 공식 변경 내역", RECT{x + 23, 435, x + width - 20, 458}, p.text, g.font, DT_SINGLELINE)
	}
	state := g.updateSnapshot()
	phase := state.Phase
	if phase == "" {
		phase = "업데이트 확인을 눌러 새 버전과 변경 내역을 확인하세요"
	}
	if state.Error != "" {
		phase += "  ·  " + state.Error
	}
	drawText(hdc, phase, RECT{x + 23, g.H - 116, x + width - 23, g.H - 89}, p.muted, g.small, DT_SINGLELINE|DT_END_ELLIPSIS)
	bar := RECT{x + 23, g.H - 82, x + width - 23, g.H - 74}
	roundBox(hdc, bar, p.border)
	pct := state.Percent
	if pct > 0 {
		bar.Right = bar.Left + (bar.Right-bar.Left)*int32(pct)/100
		if bar.Right < bar.Left+8 {
			bar.Right = bar.Left + 8
		}
		roundBox(hdc, bar, p.accent)
	}
	drawText(hdc, fmt.Sprintf("%d%%", pct), RECT{x + width - 91, g.H - 118, x + width - 21, g.H - 91}, p.muted, g.micro, DT_SINGLELINE|DT_CENTER|DT_VCENTER)
	if state.Busy && state.Total > 0 {
		mb := float64(state.Received) / 1048576
		total := float64(state.Total) / 1048576
		status := fmt.Sprintf("%.1f / %.1f MB", mb, total)
		drawText(hdc, status, RECT{x + width - 252, g.H - 118, x + width - 96, g.H - 92}, p.muted, g.micro, DT_SINGLELINE|DT_CENTER)
	}
	// Show download steps as small separated labels.
	labels := []string{"확인", "다운로드", "검증", "설치", "실행 확인"}
	spacing := width - int32(42)
	for i, label := range labels {
		xx := x + 23 + int32(i)*spacing/5
		if i > 0 {
			fill(hdc, RECT{xx - 14, g.H - 49, xx - 12, g.H - 37}, p.border)
		}
		drawText(hdc, label, RECT{xx, g.H - 50, xx + spacing/5 - 16, g.H - 29}, p.muted, g.micro, DT_SINGLELINE)
	}
}

// Format an official release's notes for a native multiline text control.
func prettyReleaseNotes(upd *UpdateInfo) string {
	if upd == nil {
		return "업데이트 확인 후 변경 내역이 여기에 표시됩니다."
	}
	title := strings.TrimSpace(upd.Name)
	if title == "" {
		title = upd.Tag
	}
	txt := "새로운 버전  ·  " + title + "\r\n"
	if upd.Prerelease {
		txt += "시험용 릴리스  ·  자동 업데이트 검증 대상\r\n"
	}
	if upd.PublishedAt != "" {
		if ts, e := time.Parse(time.RFC3339, upd.PublishedAt); e == nil {
			txt += "게시일  " + ts.Local().Format("2006.01.02 15:04") + "\r\n"
		}
	}
	txt += "\r\n변경 내역\r\n────────────────────────────\r\n"
	notes := upd.Notes
	if strings.TrimSpace(notes) == "" {
		notes = "릴리스에 변경 내역이 등록되지 않았습니다."
	}
	txt += strings.ReplaceAll(strings.ReplaceAll(notes, "\r\n", "\n"), "\n", "\r\n")
	return txt
}
