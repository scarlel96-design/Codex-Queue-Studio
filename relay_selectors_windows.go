//go:build windows

package main

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"time"

	"codexqueuestudiov3/internal"
)

const (
	cbAddString=0x0143
	cbResetContent=0x014B
	cbGetCurSel=0x0147
	cbSetCurSel=0x014E
	cbErr=^uintptr(0)
	relayModelRefresh=312
	relayWMModels=WM_APP+53
)

// All provider-specific model and conversation discovery belongs here.
// The relay core receives stable IDs, not names copied from UI labels.
func (r *RelayWindow) initializeSelectors() {
	r.control(relayWork,"COMBOBOX","",WS_TABSTOP|WS_VSCROLL|0x0003)
	r.control(relayConversation,"COMBOBOX","",WS_TABSTOP|WS_VSCROLL|0x0003)
	r.control(relayModel,"COMBOBOX","",WS_TABSTOP|WS_VSCROLL|0x0003)
	r.control(relayModelRefresh,"BUTTON","모델 새로고침",WS_TABSTOP)
	r.refreshSelectors()
	r.requestModels()
}

func addComboString(hwnd uintptr,s string){
	send(hwnd,cbAddString,0,strptr(s))
}
func comboSelection(hwnd uintptr) int{
	idx:=send(hwnd,cbGetCurSel,0,0)
	if idx==cbErr{return -1}
	return int(idx)
}
func clippedLabel(value string,n int)string{
	v:=[]rune(strings.Join(strings.Fields(value)," "))
	if len(v)>n{return string(v[:n])+"…"}
	return string(v)
}

func (r *RelayWindow) refreshSelectors() {
	nextWork:=r.parent.core.Snapshot()
	nextChats:=r.parent.core.ChatSessions()
	if !reflect.DeepEqual(r.workSessions,nextWork) {
		selectedID:=r.workChoice()
		if selectedID=="" {
			if s,ok:=r.parent.selectedSession();ok {selectedID=s.ID}
		}
		r.workSessions=nextWork
		h:=r.ctl(relayWork)
		send(h,cbResetContent,0,0)
		choice:=0
		for i,s:=range nextWork{
			addComboString(h,clippedLabel(s.Title,46)+"   ·   "+clippedLabel(s.Project,16))
			if s.ID==selectedID{choice=i}
		}
		if len(nextWork)>0{send(h,cbSetCurSel,uintptr(choice),0)}
	}
	if !reflect.DeepEqual(r.apiChats,nextChats) {
		selectedID:=r.conversationChoice()
		if selectedID==""&&r.parent.explorerTab==1 && r.parent.selectedChat>=0 && r.parent.selectedChat<len(r.parent.visibleChats){
			selectedID=r.parent.visibleChats[r.parent.selectedChat].ID
		}
		r.apiChats=nextChats
		h:=r.ctl(relayConversation)
		send(h,cbResetContent,0,0)
		addComboString(h,"새 GPT API 대화 자동 생성")
		choice:=0
		for i,v:=range nextChats{
			addComboString(h,clippedLabel(v.Title,52))
			if v.ID==selectedID {choice=i+1}
		}
		send(h,cbSetCurSel,uintptr(choice),0)
	}
}

func (r *RelayWindow) workChoice() string {
	i:=comboSelection(r.ctl(relayWork))
	if i<0||i>=len(r.workSessions){return ""}
	return r.workSessions[i].ID
}
func (r *RelayWindow) conversationChoice() string {
	i:=comboSelection(r.ctl(relayConversation))
	if i<=0||i>len(r.apiChats){return ""}
	return r.apiChats[i-1].ID
}
func (r *RelayWindow) modelChoice() string {
	i:=comboSelection(r.ctl(relayModel))
	if i<0||i>=len(r.displayModels){return ""}
	return r.displayModels[i].ID
}

func (r *RelayWindow) requestModels() {
	r.modelMu.Lock()
	if r.modelsLoading{r.modelMu.Unlock();return}
	r.modelsLoading=true
	r.modelError=""
	r.modelMu.Unlock()
	r.displayModels=nil // A loading placeholder is never a valid model selection.
	h:=r.ctl(relayModel)
	send(h,cbResetContent,0,0)
	addComboString(h,"계정 모델 목록 조회 중…")
	send(h,cbSetCurSel,0,0)
	go func() {
		ctx,cancel:=context.WithTimeout(context.Background(),25*time.Second)
		defer cancel()
		models,err:=r.parent.core.ReviewModels(ctx,"openai")
		r.modelMu.Lock()
		if err!=nil {r.modelError=err.Error()} else {r.models=models}
		r.modelsLoading=false
		r.modelMu.Unlock()
		call(u32,"PostMessageW",r.hwnd,relayWMModels,0,0)
	}()
}

func (r *RelayWindow) showModels() {
	r.modelMu.Lock()
	models:=append([]internal.ModelOption(nil),r.models...)
	errText:=r.modelError
	r.modelMu.Unlock()
	r.displayModels=models
	h:=r.ctl(relayModel)
	send(h,cbResetContent,0,0)
	if errText!="" {
		addComboString(h,"API 모델 조회 실패 · 새로고침 필요")
		send(h,cbSetCurSel,0,0)
		r.parent.core.setStatus("검토 모델 조회 실패: "+errText)
		return
	}
	if len(models)==0 {
		addComboString(h,"사용 가능한 모델이 없습니다")
		send(h,cbSetCurSel,0,0)
		return
	}
	for _,model:=range models {addComboString(h,model.ID)}
	send(h,cbSetCurSel,0,0)
}

func (r *RelayWindow) selectedSummary() string {
	return fmt.Sprintf("Codex %d개 · GPT API 대화 %d개 · 검토 모델 %d개",len(r.workSessions),len(r.apiChats),len(r.displayModels))
}
