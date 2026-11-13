//go:build windows

package main

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"syscall"
	"time"
	"unsafe"

	"codexqueuestudiov3/internal"
)

const (
	relayWork = 301
	relayConversation = 302
	relayModel = 303
	relayInstructions = 304
	relaySendBack = 305
	relayApproval = 306
	relayCreate = 307
	relayRules = 308
	relayToggle = 309
	relayApprove = 310
	relayPreview = 311
	relayWMUpdated = WM_APP + 52
)

type RelayWindow struct {
	hwnd uintptr
	parent *GUI
	W,H int32
	controls map[int]uintptr
	rules []internal.RelayRule
	selectedID string
}

var activeRelayWindow *RelayWindow

func (r *RelayWindow) ctl(id int) uintptr { return r.controls[id] }

func (r *RelayWindow) control(id int,class,title string,style uintptr) {
	h:=createWindow(class,title,WS_CHILD|WS_VISIBLE|style,0,r.hwnd,id,0,0,100,30)
	r.controls[id]=h
	setFont(h,r.parent.font)
}

func (r *RelayWindow) init() {
	r.controls=map[int]uintptr{}
	for _,id:=range []int{relayWork,relayConversation,relayModel,relayInstructions,relayPreview} {
		style:=uintptr(WS_TABSTOP|0x0080)
		if id==relayInstructions || id==relayPreview {style=WS_TABSTOP|ES_MULTILINE|ES_AUTOVSCROLL|WS_VSCROLL}
		r.control(id,"EDIT","",style)
	}
	send(r.ctl(relayWork),EM_SETREADONLY,1,0)
	send(r.ctl(relayPreview),EM_SETREADONLY,1,0)
	r.control(relaySendBack,"BUTTON","AI 답변을 원래 Codex 세션으로 다시 전송",WS_TABSTOP|0x3)
	r.control(relayApproval,"BUTTON","재전송 전에 사용자 승인 받기",WS_TABSTOP|0x3)
	send(r.ctl(relayApproval),0x00F1,1,0) // BM_SETCHECK
	r.control(relayCreate,"BUTTON","새 자동화 규칙 생성",WS_TABSTOP|0x1)
	r.control(relayToggle,"BUTTON","규칙 활성화 / 일시 정지",WS_TABSTOP)
	r.control(relayApprove,"BUTTON","검토 답변 승인 및 전달",WS_TABSTOP)
	r.control(relayRules,"SysListView32","",WS_TABSTOP|LVS_REPORT|LVS_SINGLESEL|LVS_SHOWSELALWAYS|LVS_OWNERDATA)
	h:=r.ctl(relayRules)
	send(h,LVM_SETEXTENDEDLISTVIEWSTYLE,0,LVS_EX_FULLROWSELECT|LVS_EX_DOUBLEBUFFER|LVS_EX_LABELTIP)
	for i,col:=range []struct{title string;width int32}{{"작업 세션",190},{"API 대화",155},{"상태",120},{"반복",75}} {
		c:=LVCOLUMNW{Mask:0x7,CX:col.width,Text:utf(col.title),SubItem:int32(i)}
		send(h,LVM_INSERTCOLUMNW,uintptr(i),uintptr(unsafe.Pointer(&c)))
	}
	if s,ok:=r.parent.selectedSession();ok {settext(r.ctl(relayWork),s.ID)}
	settext(r.ctl(relayModel),"gpt-5.4")
	settext(r.ctl(relayPreview),"규칙을 선택하면 실행 상태와 AI 답변을 확인할 수 있습니다.")
	r.resize()
	r.refresh()
	call(u32,"SetTimer",r.hwnd,1,3000,0)
}

func (r *RelayWindow) resize() {
	w,h:=r.W,r.H
	if w<900{w=900}
	if h<660{h=660}
	r.W=w;r.H=h
	left:=w*45/100
	x:=int32(42)
	formW:=left-64
	rightX:=left+26
	rightW:=w-rightX-36
	move(r.ctl(relayWork),x,176,formW,35)
	move(r.ctl(relayConversation),x,249,formW,35)
	move(r.ctl(relayModel),x,322,formW,35)
	move(r.ctl(relayInstructions),x,394,formW,124)
	move(r.ctl(relaySendBack),x,545,formW,28)
	move(r.ctl(relayApproval),x,582,formW,28)
	move(r.ctl(relayCreate),x,630,formW,42)
	move(r.ctl(relayRules),rightX,174,rightW,h-482)
	move(r.ctl(relayPreview),rightX,h-291,rightW,172)
	move(r.ctl(relayToggle),rightX,h-91,rightW*45/100,42)
	move(r.ctl(relayApprove),rightX+rightW*45/100+11,h-91,rightW-rightW*45/100-11,42)
	call(u32,"InvalidateRect",r.hwnd,0,0)
}

func (r *RelayWindow) refresh() {
	next:=r.parent.core.RelayRules()
	if reflect.DeepEqual(r.rules,next) {return}
	old:=r.rules
	r.rules=next
	h:=r.ctl(relayRules)
	if len(old)!=len(next) {
		send(h,LVM_SETITEMCOUNT,uintptr(len(next)),3)
		call(u32,"InvalidateRect",h,0,0)
	} else {
		for i:=range next {if old[i]!=next[i] {send(h,LVM_FIRST+21,uintptr(i),uintptr(i))}}
	}
	idx:=-1
	for i,v:=range next {if v.ID==r.selectedID{idx=i;break}}
	if idx>=0{
		item:=LVITEMW{StateMask:3,State:3}
		send(h,LVM_FIRST+43,uintptr(idx),uintptr(unsafe.Pointer(&item)))
	}
	r.preview()
}

func (r *RelayWindow) selected() (internal.RelayRule,bool) {
	idx:=int(send(r.ctl(relayRules),LVM_GETNEXTITEM,^uintptr(0),2))
	if idx<0||idx>=len(r.rules){return internal.RelayRule{},false}
	return r.rules[idx],true
}

func (r *RelayWindow) preview() {
	rule,ok:=r.selected()
	if !ok{return}
	r.selectedID=rule.ID
	info:=fmt.Sprintf("상태: %s\r\n작업: %s\r\nAPI 대화: %s\r\n반복: %d/%d",relayStatus(rule.State),rule.WorkSessionID,rule.ConversationID,rule.CompletedCycles,rule.MaxCycles)
	if rule.LastError!=""{info+="\r\n원인: "+rule.LastError}
	if rule.ReplyCipher!=""{
		if reply,err:=internal.OpenMessage(rule.ReplyCipher);err==nil{info+="\r\n\r\n검토 답변:\r\n"+reply}
	}
	settext(r.ctl(relayPreview),info)
}

func relayStatus(status string) string {
	switch status {
	case "idle":return "다음 작업 완료 감시 중"
	case "reviewing":return "AI 검토 중"
	case "approval-required":return "사용자 승인 대기"
	case "reply-sending":return "Codex 재전송 중"
	case "uncertain":return "결과 불명 · 자동 중지"
	case "paused":return "일시 정지"
	default:return status
	}
}

func (r *RelayWindow) alert(message string) {
	call(u32,"MessageBoxW",r.hwnd,strptr(message),strptr("Codex Queue Studio · 스마트 릴레이"),0x40)
}
func (r *RelayWindow) ask(message string) bool {
	return call(u32,"MessageBoxW",r.hwnd,strptr(message),strptr("Codex Queue Studio · 스마트 릴레이"),0x24)==6
}

func (r *RelayWindow) click(id int) {
	switch id {
	case relayCreate:
		work:=strings.TrimSpace(gettext(r.ctl(relayWork)))
		conv:=strings.TrimSpace(gettext(r.ctl(relayConversation)))
		model:=strings.TrimSpace(gettext(r.ctl(relayModel)))
		instructions:=strings.TrimSpace(gettext(r.ctl(relayInstructions)))
		back:=send(r.ctl(relaySendBack),0x00F0,0,0)!=0
		approval:=send(r.ctl(relayApproval),0x00F0,0,0)!=0
		if !r.ask("작업 완료 보고가 OpenAI API로 전송됩니다. ChatGPT 구독과 별도의 API 사용 요금이 발생할 수 있습니다. 규칙을 생성할까요?"){return}
		if back && !approval && !r.ask("사용자 승인 없이 AI 답변이 Codex 작업 세션으로 전달됩니다. 최대 3회 반복합니다. 활성화할까요?"){return}
		id,err:=r.parent.core.AddRelay(work,conv,model,instructions,back,approval)
		if err!=nil{r.alert(err.Error());return}
		r.selectedID=id
		r.refresh()
	case relayToggle:
		rule,ok:=r.selected();if !ok{r.alert("규칙을 선택하세요");return}
		if rule.State=="uncertain" {
			if !r.ask("이전 요청의 결과를 확인하지 못했습니다. 중복 전송을 방지하기 위해 해당 턴을 건너뛰고 다음 작업 완료부터 다시 감시할까요?"){return}
			if err:=r.parent.core.AcknowledgeRelayUncertain(rule.ID);err!=nil{r.alert(err.Error())}
		} else {
			if err:=r.parent.core.SetRelayEnabled(rule.ID,!rule.Enabled);err!=nil{r.alert(err.Error())}
		}
		r.refresh()
	case relayApprove:
		rule,ok:=r.selected();if !ok{r.alert("규칙을 선택하세요");return}
		if rule.State!="approval-required"{r.alert("승인을 기다리는 답변이 없습니다");return}
		r.preview()
		if !r.ask("아래 상세 보기에 표시된 AI 답변을 확인했습니까? 이 답변을 Codex 작업 세션으로 전송할까요?"){return}
		go func(){
			ctx,cancel:=context.WithTimeout(context.Background(),30*time.Minute)
			defer cancel()
			if err:=r.parent.core.ApproveRelayReply(ctx,rule.ID);err!=nil{r.parent.core.setStatus(err.Error())}
			call(u32,"PostMessageW",r.hwnd,relayWMUpdated,0,0)
		}()
	}
}

func (r *RelayWindow) notify(l uintptr) {
	hdr:=(*NMHDR)(unsafe.Pointer(l))
	if int(hdr.IDFrom)!=relayRules{return}
	if hdr.Code==LVN_ITEMCHANGED {
		r.preview()
		return
	}
	if hdr.Code!=LVN_GETDISPINFOW{return}
	di:=(*NMLVDISPINFOW)(unsafe.Pointer(l))
	idx:=int(di.Item.IItem)
	if idx<0||idx>=len(r.rules){return}
	v:=r.rules[idx]
	txt:=""
	switch int(di.Item.ISubItem) {
	case 0:txt=v.WorkSessionID
	case 1:txt=v.ConversationID
	case 2:txt=relayStatus(v.State)
	case 3:txt=fmt.Sprintf("%d/%d",v.CompletedCycles,v.MaxCycles)
	}
	if di.Item.Mask&1!=0 && di.Item.Text!=nil && di.Item.TextMax>0 {
		dest:=unsafe.Slice(di.Item.Text,int(di.Item.TextMax))
		copy(dest,syscall.StringToUTF16(txt))
		dest[len(dest)-1]=0
	}
}

func (r *RelayWindow) draw(hdc uintptr) {
	p:=r.parent.pal
	fill(hdc,RECT{0,0,r.W,r.H},p.bg)
	left:=r.W*45/100
	rightX:=left+26
	roundBox(hdc,RECT{25,112,left-6,r.H-23},p.card)
	roundBox(hdc,RECT{rightX-15,112,r.W-24,r.H-23},p.card)
	drawText(hdc,"스마트 릴레이",RECT{32,24,r.W-30,68},p.text,r.parent.heading,DT_SINGLELINE)
	drawText(hdc,"Codex 완료 → OpenAI API 대화 검토 → 선택한 경우 원래 Codex로 답변 전달",RECT{34,73,r.W-28,104},p.muted,r.parent.font,DT_SINGLELINE)
	for _,item:=range []struct{label string;y int32}{
		{"Codex 작업 세션 UUID",149},
		{"OpenAI API Conversation ID · 비우면 새로 생성",222},
		{"검토 모델",295},
		{"검토 지침 · 선택 사항",367},
	} {
		drawText(hdc,item.label,RECT{42,item.y,left-20,item.y+26},p.muted,r.parent.small,DT_SINGLELINE)
	}
	drawText(hdc,"연동 규칙 및 상태",RECT{rightX,143,r.W-26,170},p.text,r.parent.bold,DT_SINGLELINE)
	drawText(hdc,"선택한 규칙의 AI 검토 결과",RECT{rightX,r.H-323,r.W-26,r.H-294},p.muted,r.parent.small,DT_SINGLELINE)
	drawText(hdc,"기존 ChatGPT 앱 채팅 ID는 API Conversation ID와 다릅니다.",RECT{42,r.H-102,left-20,r.H-61},p.muted,r.parent.small,DT_SINGLELINE)
	drawText(hdc,"API 키는 OPENAI_API_KEY 환경 변수로 설정합니다.",RECT{42,r.H-68,left-20,r.H-32},p.muted,r.parent.small,DT_SINGLELINE)
}

func relayWndProc(hwnd,msg,w,l uintptr) uintptr {
	r:=activeRelayWindow
	if r==nil{return call(u32,"DefWindowProcW",hwnd,msg,w,l)}
	switch uint32(msg) {
	case WM_CREATE:
		r.hwnd=hwnd;r.init();return 0
	case WM_GETMINMAXINFO:
		m:=(*MINMAXINFO)(unsafe.Pointer(l));m.MinTrackSize=POINT{X:1020,Y:730};return 0
	case WM_SIZE:
		r.W=int32(l&0xFFFF);r.H=int32((l>>16)&0xFFFF);r.resize();return 0
	case WM_ERASEBKGND:return 1
	case WM_PAINT:
		var ps PAINTSTRUCT
		hdc:=call(u32,"BeginPaint",hwnd,uintptr(unsafe.Pointer(&ps)))
		r.draw(hdc)
		call(u32,"EndPaint",hwnd,uintptr(unsafe.Pointer(&ps)))
		return 0
	case WM_CTLCOLOREDIT,WM_CTLCOLORSTATIC:
		call(g32,"SetTextColor",w,uintptr(r.parent.pal.text))
		call(g32,"SetBkColor",w,uintptr(r.parent.pal.field))
		return r.parent.brushField
	case WM_TIMER:
		if w==1{r.refresh()}
		return 0
	case relayWMUpdated:
		r.refresh();return 0
	case WM_NOTIFY:
		r.notify(l);return 0
	case WM_COMMAND:
		if (w>>16)&0xFFFF==BN_CLICKED{r.click(int(w&0xFFFF))}
		return 0
	case WM_CLOSE:
		call(u32,"DestroyWindow",hwnd);return 0
	case WM_DESTROY:
		call(u32,"KillTimer",hwnd,1)
		activeRelayWindow=nil
		return 0
	}
	return call(u32,"DefWindowProcW",hwnd,msg,w,l)
}

func openRelayWindow(parent *GUI) {
	if activeRelayWindow!=nil && activeRelayWindow.hwnd!=0 {
		call(u32,"ShowWindow",activeRelayWindow.hwnd,SW_RESTORE)
		call(u32,"SetForegroundWindow",activeRelayWindow.hwnd)
		return
	}
	r:=&RelayWindow{parent:parent,W:1180,H:790}
	activeRelayWindow=r
	cls:=utf("CQSRelayNativeV1")
	wc:=WNDCLASSEX{Size:uint32(unsafe.Sizeof(WNDCLASSEX{})),WndProc:syscall.NewCallback(relayWndProc),
		Instance:call(k32,"GetModuleHandleW",0),Cursor:call(u32,"LoadCursorW",0,IDC_ARROW),ClassName:cls,Icon:parent.icon,SmallIcon:parent.icon}
	call(u32,"RegisterClassExW",uintptr(unsafe.Pointer(&wc)))
	h:=createWindow("CQSRelayNativeV1","Codex Queue Studio · 스마트 릴레이",WS_OVERLAPPEDWINDOW|WS_VISIBLE,0,0,0,100,80,1180,790)
	if h==0{
		activeRelayWindow=nil
		parent.alert("스마트 릴레이 창을 만들지 못했습니다")
		return
	}
	r.hwnd=h
	send(h,WM_SETICON,1,parent.icon)
}
