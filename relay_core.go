package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"codexqueuestudiov3/internal"
)

// RelayRules returns a detached snapshot for the native UI.
func (c *Core) RelayRules() []internal.RelayRule {
	return c.store.Snapshot().Relays
}

// AddRelay starts at the current completed turn. It never replays old work.
func (c *Core) AddRelay(sessionID, conversationID, model, instructions string, sendBack, requireApproval bool) (string,error) {
	rule:=internal.RelayRule{
		ID:internal.NewID(),WorkSessionID:sessionID,ConversationID:strings.TrimSpace(conversationID),Model:strings.TrimSpace(model),
		Instructions:strings.TrimSpace(instructions),Enabled:true,SendBack:sendBack,
		RequireApproval:requireApproval,MaxCycles:3,State:"idle",UpdatedAt:time.Now(),
	}
	if err:=internal.ValidateRelayRule(rule);err!=nil{return "",err}
	var file string
	for _,s:=range c.Snapshot() {
		if s.ID==sessionID && !s.Archived {file=s.File;break}
	}
	if file=="" {return "",errors.New("세션 탐색기에서 로컬 Codex 작업 세션을 선택하세요")}
	if completion,err:=internal.LatestCompletion(file);err==nil {
		rule.LastTurnID=completion.TurnID
	} else {return "",fmt.Errorf("세션 완료 기록 검사 실패: %w",err)}
	if err:=c.store.Update(func(s *internal.State) error {
		for _,v:=range s.Relays {if v.WorkSessionID==sessionID && v.Enabled {return errors.New("이미 해당 세션에서 동작 중인 자동화 규칙이 있습니다")}}
		s.Relays=append(s.Relays,rule)
		internal.AddEvent(s,"relay-created",rule.ID,"Codex 작업 완료 감시 활성화")
		return nil
	});err!=nil{return "",err}
	return rule.ID,nil
}

func (c *Core) SetRelayEnabled(id string,enabled bool) error {
	return c.store.Update(func(s *internal.State) error {
		for i:=range s.Relays {
			r:=&s.Relays[i]
			if r.ID!=id {continue}
			if r.State=="reviewing"||r.State=="reply-sending" {return errors.New("실행 중에는 규칙을 변경할 수 없습니다")}
			if enabled && (r.State=="uncertain"||r.State=="approval-required") {return errors.New("대기 중인 답변 또는 불명확한 실행 상태를 먼저 처리하세요")}
			if enabled && r.CompletedCycles>=r.MaxCycles {return errors.New("최대 반복 횟수에 도달했습니다. 새 규칙으로 다시 시작하세요")}
			r.Enabled=enabled
			if !enabled {r.State="paused"} else {r.State="idle"}
			r.UpdatedAt=time.Now()
			return nil
		}
		return errors.New("자동화 규칙이 없습니다")
	})
}

// runRelayOnce never retries uncertain delivery automatically.
func (c *Core) runRelayOnce(ctx context.Context) {
	if !c.relayMu.TryLock(){return}
	defer c.relayMu.Unlock()
	for _,rule:=range c.store.Snapshot().Relays {
		if !rule.Enabled||rule.State!="idle"||rule.CompletedCycles>=rule.MaxCycles {continue}
		var file string
		for _,s:=range c.Snapshot(){if s.ID==rule.WorkSessionID && !s.Archived {file=s.File;break}}
		if file=="" {continue}
		completion,err:=internal.LatestCompletion(file)
		if err!=nil || completion.TurnID=="" || completion.TurnID==rule.LastTurnID {continue}
		if strings.TrimSpace(os.Getenv("OPENAI_API_KEY"))=="" {
			c.setStatus("스마트 릴레이 대기: OPENAI_API_KEY 환경 변수가 필요합니다")
			continue
		}
		// Reserve this turn before any network call; retries require user action.
		if err:=c.store.Update(func(st *internal.State) error {
			for i:=range st.Relays {
				r:=&st.Relays[i]
				if r.ID==rule.ID && r.State=="idle" && r.LastTurnID!=completion.TurnID {
					r.LastTurnID=completion.TurnID;r.State="reviewing";r.UpdatedAt=time.Now()
					internal.AddEvent(st,"relay-reviewing",r.ID,"Codex 작업 완료 감지, API 채팅 검토 준비")
					return nil
				}
			}
			return errors.New("자동화 상태가 변경되었습니다")
		});err!=nil {continue}
		client:=internal.NewResponsesClientFromEnvironment()
		convID:=rule.ConversationID
		if convID=="" {
			convID,err=client.CreateConversation(ctx)
			if err==nil {
				err=c.store.Update(func(st *internal.State) error {
					for i:=range st.Relays {if st.Relays[i].ID==rule.ID && st.Relays[i].State=="reviewing" {st.Relays[i].ConversationID=convID;return nil}}
					return errors.New("자동화 규칙이 변경되었습니다")
				})
			}
		}
		var reply string
		if err==nil {
			reply,err=client.Review(ctx,convID,rule.Model,completion.Message,rule.Instructions)
		}
		if err!=nil {
			c.relayUncertain(rule.ID,"AI 채팅 검토 결과를 확인할 수 없습니다: "+err.Error())
			continue
		}
		cipher:=""
		if rule.SendBack {
			cipher,err=internal.SealMessage(reply)
			if err!=nil {c.relayUncertain(rule.ID,err.Error());continue}
		}
		state:="idle"
		if rule.CompletedCycles+1>=rule.MaxCycles {state="paused"}
		if rule.SendBack {
			if rule.RequireApproval {state="approval-required"} else {state="reply-sending"}
		}
		err=c.store.Update(func(st *internal.State) error {
			for i:=range st.Relays {
				r:=&st.Relays[i]
				if r.ID==rule.ID && r.State=="reviewing" {
					r.State=state;r.CompletedCycles++;r.ReplyCipher=cipher;r.UpdatedAt=time.Now()
					r.LastError=""
					internal.AddEvent(st,"relay-reviewed",r.ID,"AI 검토 완료: "+state)
					return nil
				}
			}
			return errors.New("자동화 규칙이 변경되었습니다")
		})
		if err!=nil{continue}
		if state=="reply-sending" { c.deliverRelayReply(ctx,rule.ID,rule.WorkSessionID,cipher) }
	}
}

func (c *Core) relayUncertain(id,reason string) {
	_ = c.store.Update(func(st *internal.State) error {
		for i:=range st.Relays {
			r:=&st.Relays[i]
			if r.ID!=id{continue}
			r.State="uncertain";r.Enabled=false;r.LastError=reason;r.UpdatedAt=time.Now()
			internal.AddEvent(st,"relay-uncertain",id,reason)
			break
		}
		return nil
	})
	c.setStatus("자동화 확인 필요 · "+reason)
}

// ApproveRelayReply authorizes one pending AI response. The caller must ask the user first.
func (c *Core) ApproveRelayReply(ctx context.Context,id string) error {
	var rule internal.RelayRule
	err:=c.store.Update(func(st *internal.State) error {
		for i:=range st.Relays {
			r:=&st.Relays[i]
			if r.ID!=id {continue}
			if r.State!="approval-required"||r.ReplyCipher=="" {return errors.New("승인을 기다리는 답변이 없습니다")}
			rule=*r
			r.State="reply-sending";r.UpdatedAt=time.Now()
			internal.AddEvent(st,"relay-approved",r.ID,"사용자가 AI 답변의 Codex 전달을 승인했습니다")
			return nil
		}
		return errors.New("자동화 규칙을 찾을 수 없습니다")
	})
	if err!=nil{return err}
	c.deliverRelayReply(ctx,id,rule.WorkSessionID,rule.ReplyCipher)
	return nil
}

func (c *Core) deliverRelayReply(ctx context.Context,id,workSessionID,cipher string) {
	reply,err:=internal.OpenMessage(cipher)
	if err!=nil {c.relayUncertain(id,"답변 복호화 실패: "+err.Error());return}
	if !c.EnginePresent(){c.relayUncertain(id,"Codex 실행 엔진이 없습니다");return}
	c.sendMu.Lock()
	detail,err:=c.Server().Send(ctx,workSessionID,reply)
	c.sendMu.Unlock()
	if err!=nil {
		c.relayUncertain(id,"Codex 답변 전달 결과 불명: "+err.Error()+" "+detail)
		return
	}
	_ = c.store.Update(func(st *internal.State) error {
		for i:=range st.Relays {
			r:=&st.Relays[i]
			if r.ID==id && r.State=="reply-sending" {
				r.ReplyCipher=""
				if r.CompletedCycles>=r.MaxCycles {r.State="paused";r.Enabled=false} else {r.State="idle"}
				r.UpdatedAt=time.Now()
				internal.AddEvent(st,"relay-delivered",id,"Codex 응답 전달 완료")
			}
		}
		return nil
	})
	c.setStatus("AI 검토 답변을 Codex에 전달했습니다")
}
