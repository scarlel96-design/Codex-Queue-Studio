//go:build windows

package main

import (
	"context"
	"fmt"
	"reflect"
	"syscall"
	"strings"
	"time"
	"unsafe"

	"codexqueuestudiov3/internal"
)

const (
	explorerAnimationTimer=9910
	explorerAnimationDuration=210*time.Millisecond
)

func (g *GUI) refreshChatSessions() {
	all:=g.core.ChatSessions()
	query:=strings.ToLower(strings.TrimSpace(gettext(g.ctl(ID_SEARCH))))
	next:=make([]internal.ChatSession,0,len(all))
	for _,v:=range all {
		if query==""||strings.Contains(strings.ToLower(v.Title+" "+v.ID+" "+v.ProviderID),query){
			next=append(next,v)
		}
	}
	prevID:=""
	if g.selectedChat>=0&&g.selectedChat<len(g.visibleChats){prevID=g.visibleChats[g.selectedChat].ID}
	old:=g.visibleChats
	g.visibleChats=next
	g.chats=all
	if reflect.DeepEqual(old,next){return}
	h:=g.ctl(ID_GPT_LIST)
	if len(old)!=len(next){
		send(h,LVM_SETITEMCOUNT,uintptr(len(next)),3)
		call(u32,"InvalidateRect",h,0,0)
	}else{
		for i:=range next {
			if old[i]!=next[i] {send(h,LVM_FIRST+21,uintptr(i),uintptr(i))}
		}
	}
	nextSelected:=-1
	for i,v:=range next {if v.ID==prevID {nextSelected=i;break}}
	if g.selectedChat!=nextSelected{
		g.selectedChat=nextSelected
		item:=LVITEMW{StateMask:3}
		send(h,LVM_FIRST+43,^uintptr(0),uintptr(unsafe.Pointer(&item)))
		if nextSelected>=0{
			item.State=3
			send(h,LVM_FIRST+43,uintptr(nextSelected),uintptr(unsafe.Pointer(&item)))
		}
	}
	g.invalidateSummary()
}

func (g *GUI) selectGPTChat(index int) {
	if index<0||index>=len(g.visibleChats) {return}
	g.selectedChat=index
	id:=g.visibleChats[index].ID
	title:=g.visibleChats[index].Title
	seq:=g.chatSeq.Add(1)
	settext(g.ctl(ID_GPT_HISTORY),"API 대화 기록을 불러오는 중…")
	go func(){
		ctx,cancel:=context.WithTimeout(context.Background(),30*time.Second)
		defer cancel()
		messages,err:=g.core.ChatHistory(ctx,id)
		var out strings.Builder
		out.WriteString(title+"\r\n"+id+"\r\n\r\n")
		if err!=nil {
			out.WriteString("대화 기록을 가져오지 못했습니다: "+err.Error())
		} else if len(messages)==0 {
			out.WriteString("이 API 대화에는 저장된 텍스트 메시지가 없습니다.")
		} else {
			for _,m:=range messages{
				role:="GPT"
				if m.Role=="user"{role="사용자"}
				out.WriteString("● "+role+"\r\n"+m.Text+"\r\n\r\n")
			}
		}
		if g.chatSeq.Load()!=seq {return}
		g.chatMu.Lock()
		g.chatHistory=out.String()
		g.chatMu.Unlock()
		call(u32,"PostMessageW",g.hwnd,WM_APP+12,0,0)
	}()
}

func (g *GUI) createGPTChat() {
	if !g.ask("새 OpenAI API 대화를 만들고 로컬 GPT 탭에 등록할까요? 기존 ChatGPT 앱 채팅방은 아닙니다."){return}
	g.core.setStatus("GPT API 대화 생성 중")
	go func(){
		ctx,cancel:=context.WithTimeout(context.Background(),30*time.Second)
		defer cancel()
		chat,err:=g.core.CreateChat(ctx,"openai")
		if err!=nil {g.core.setStatus("GPT 대화 생성 실패: "+err.Error())} else {
			g.core.setStatus("GPT API 대화 생성 완료: "+chat.Title)
		}
		call(u32,"PostMessageW",g.hwnd,WM_APP+11,0,0)
	}()
}

func (g *GUI) setExplorerTab(tab int) {
	if tab==g.explorerTab{return}
	g.chatSeq.Add(1)
	from:=g.explorerTab
	g.explorerTab=tab
	settext(g.ctl(ID_SEARCH),"")
	placeholder:="Codex 작업 세션 검색"
	if tab==1{placeholder="등록된 GPT API 채팅 검색"}
	send(g.ctl(ID_SEARCH),EM_SETCUEBANNER,0,strptr(placeholder))
	g.layout()
	if tab==1 {g.refreshChatSessions()} else {g.refreshSessions()}
	g.startTabAnimation(from,tab)
}

func (g *GUI) startTabAnimation(from,to int) {
	// Respect the Windows "Animation effects" accessibility setting.
	var enabled int32
	ok:=call(u32,"SystemParametersInfoW",0x1042,0,uintptr(unsafe.Pointer(&enabled)),0)!=0
	if ok&&enabled==0 {
		g.tabMotionActive=false
		g.tabMotionPosition=float64(to)
		g.invalidateTabStrip()
		return
	}
	g.tabMotionStarted=time.Now()
	g.tabMotionFrom=float64(from)
	g.tabMotionTo=float64(to)
	g.tabMotionActive=true
	call(u32,"SetTimer",g.hwnd,explorerAnimationTimer,16,0)
	g.invalidateTabStrip()
}

func (g *GUI) tickTabAnimation() {
	if !g.tabMotionActive{return}
	fraction:=float64(time.Since(g.tabMotionStarted))/float64(explorerAnimationDuration)
	if fraction>=1{
		g.tabMotionPosition=g.tabMotionTo
		g.tabMotionActive=false
		call(u32,"KillTimer",g.hwnd,explorerAnimationTimer)
	}else{
		if fraction<0{fraction=0}
		// Cubic ease-out; no full-window redraw.
		t:=1-(1-fraction)*(1-fraction)*(1-fraction)
		g.tabMotionPosition=g.tabMotionFrom+(g.tabMotionTo-g.tabMotionFrom)*t
	}
	g.invalidateTabStrip()
}
func (g *GUI) invalidateTabStrip(){
	r:=RECT{Left:226,Top:110,Right:560,Bottom:167}
	call(u32,"InvalidateRect",g.hwnd,uintptr(unsafe.Pointer(&r)),0)
}

func (g *GUI) drawExplorerTabs(hdc uintptr) {
	if g.page!=0{return}
	p:=g.pal
	x:=int32(233)
	// A compact two-option segmented control. Native Windows caption remains.
	roundBox(hdc,RECT{x-4,115,x+306,159},p.field)
	if g.tabMotionPosition<0 {g.tabMotionPosition=0}
	if g.tabMotionPosition>1 {g.tabMotionPosition=1}
	offset:=int32(g.tabMotionPosition*150)
	roundBox(hdc,RECT{x+offset,119,x+149+offset,154},p.card)
	drawText(hdc,"Codex 작업",RECT{x+8,121,x+145,151},func()uint32{if g.explorerTab==0{return p.accent};return p.muted}(),g.font,DT_CENTER|DT_VCENTER|DT_SINGLELINE)
	drawText(hdc,"GPT 채팅",RECT{x+158,121,x+293,151},func()uint32{if g.explorerTab==1{return p.accent};return p.muted}(),g.font,DT_CENTER|DT_VCENTER|DT_SINGLELINE)
}

func (g *GUI) drawGPTInspector(hdc uintptr,xr,right int32) {
	p:=g.pal
	title:="GPT API 대화를 선택하세요"
	if g.selectedChat>=0&&g.selectedChat<len(g.visibleChats){title=g.visibleChats[g.selectedChat].Title}
	drawText(hdc,title,RECT{xr+8,221,xr+right-8,255},p.text,g.bold,DT_SINGLELINE|DT_END_ELLIPSIS)
	drawText(hdc,"OpenAI API · 실제 대화 내용 · 최근 80개 메시지",RECT{xr+7,258,xr+right-8,287},p.muted,g.small,DT_SINGLELINE)
	drawText(hdc,"ChatGPT 앱의 개인 채팅은 API 목록에 포함되지 않습니다.",RECT{xr+7,g.H-139,xr+right-8,g.H-107},p.muted,g.small,DT_SINGLELINE)
}

func (g *GUI) handleGPTNotify(l uintptr) bool {
	hdr:=(*NMHDR)(unsafe.Pointer(l))
	if int(hdr.IDFrom)!=ID_GPT_LIST {return false}
	if hdr.Code==LVN_GETDISPINFOW {
		di:=(*NMLVDISPINFOW)(unsafe.Pointer(l))
		idx:=int(di.Item.IItem)
		if idx<0||idx>=len(g.visibleChats){return true}
		chat:=g.visibleChats[idx]
		var out string
		switch di.Item.ISubItem {
		case 0:out=chat.Title
		case 1:out=chat.ProviderID
		case 2:out=chat.UpdatedAt.Local().Format("01-02 15:04")
		}
		if di.Item.Mask&1!=0&&di.Item.Text!=nil&&di.Item.TextMax>0{
			dest:=unsafe.Slice(di.Item.Text,int(di.Item.TextMax))
			copy(dest,syscallUTF16(out))
			dest[len(dest)-1]=0
		}
		return true
	}
	if hdr.Code==LVN_ITEMCHANGED {
		idx:=int(send(g.ctl(ID_GPT_LIST),LVM_GETNEXTITEM,^uintptr(0),2))
		if idx>=0&&idx!=g.selectedChat {g.selectGPTChat(idx)}
		return true
	}
	return false
}

func syscallUTF16(text string) []uint16 {return syscall.StringToUTF16(text)}

func (g *GUI) refreshGPTHistoryView() {
	g.chatMu.Lock()
	value:=g.chatHistory
	g.chatMu.Unlock()
	if g.explorerTab==1 && g.page==0 {
		settext(g.ctl(ID_GPT_HISTORY),value)
	}
}
func (g *GUI) describeExplorer() string {
	if g.explorerTab==1 {return fmt.Sprintf("GPT API 대화 %d개",len(g.visibleChats))}
	return fmt.Sprintf("Codex 작업 %d개 · 검색 %d개",len(g.sessions),len(g.visible))
}
