package main

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"codexqueuestudiov3/internal"
)

func (c *Core) chatProviders() *internal.ProviderRegistry {
	// Providers are stateless adapters. Adding a vendor requires one adapter
	// registration and does not change session storage or the relay UI contract.
	return internal.NewProviderRegistry(internal.NewOpenAIChatProvider())
}

func (c *Core) ProviderIDs() []string {
	return c.chatProviders().IDs()
}

func (c *Core) ReviewModels(ctx context.Context,providerID string) ([]internal.ModelOption,error) {
	p,ok:=c.chatProviders().Get(providerID)
	if !ok {return nil,fmt.Errorf("등록되지 않은 AI 제공자: %s",providerID)}
	return p.Models(ctx)
}

func (c *Core) ChatSessions() []internal.ChatSession {
	snap:=c.store.Snapshot()
	existing:=map[string]internal.ChatSession{}
	for _,chat:=range snap.ChatSessions{existing[chat.ID]=chat}
	for _,r:=range snap.Relays{
		if r.ConversationID=="" {continue}
		if _,exists:=existing[r.ConversationID];exists {continue}
		existing[r.ConversationID]=internal.ChatSession{
			ID:r.ConversationID,ProviderID:"openai",
			Title:"Codex 검토 · "+r.WorkSessionID[:min(8,len(r.WorkSessionID))],
			CreatedAt:r.UpdatedAt,UpdatedAt:r.UpdatedAt,
		}
	}
	list:=make([]internal.ChatSession,0,len(existing))
	for _,v:=range existing{list=append(list,v)}
	sort.Slice(list,func(i,j int)bool{
		if list[i].UpdatedAt.Equal(list[j].UpdatedAt){return list[i].ID<list[j].ID}
		return list[i].UpdatedAt.After(list[j].UpdatedAt)
	})
	return list
}

func (c *Core) RegisterChat(providerID,id,title string) error {
	p,ok:=c.chatProviders().Get(providerID)
	if !ok{return errors.New("지원되지 않는 AI 제공자")}
	if p.ID()=="openai" && !strings.HasPrefix(id,"conv_"){return errors.New("OpenAI API 대화 ID를 확인하세요")}
	if strings.TrimSpace(title)==""{title=p.Label()+" · "+id}
	now:=time.Now()
	return c.store.Update(func(s *internal.State)error{
		for i:=range s.ChatSessions{
			if s.ChatSessions[i].ID==id {
				if s.ChatSessions[i].ProviderID!=p.ID(){return errors.New("동일한 ID가 다른 제공자에 등록되었습니다")}
				s.ChatSessions[i].Title=title
				s.ChatSessions[i].UpdatedAt=now
				return nil
			}
		}
		s.ChatSessions=append(s.ChatSessions,internal.ChatSession{
			ID:id,ProviderID:p.ID(),Title:title,CreatedAt:now,UpdatedAt:now,
		})
		internal.AddEvent(s,"api-chat",id,"API 채팅 세션이 로컬 탐색기에 등록되었습니다")
		return nil
	})
}

func (c *Core) CreateChat(ctx context.Context,providerID string)(internal.ChatSession,error) {
	p,ok:=c.chatProviders().Get(providerID)
	if !ok{return internal.ChatSession{},errors.New("지원되지 않는 AI 제공자")}
	id,err:=p.CreateConversation(ctx)
	if err!=nil{return internal.ChatSession{},err}
	now:=time.Now()
	title:=p.Label()+" · "+now.Local().Format("2006-01-02 15:04")
	if err=c.RegisterChat(providerID,id,title);err!=nil{return internal.ChatSession{},err}
	for _,v:=range c.ChatSessions(){if v.ID==id{return v,nil}}
	return internal.ChatSession{},errors.New("등록한 API 채팅을 확인할 수 없습니다")
}

func (c *Core) ChatHistory(ctx context.Context,id string)([]internal.ChatMessage,error) {
	for _,chat:=range c.ChatSessions(){
		if chat.ID!=id{continue}
		p,ok:=c.chatProviders().Get(chat.ProviderID)
		if !ok{return nil,errors.New("이 대화의 AI 제공자가 설치되지 않았습니다")}
		return p.History(ctx,id,80)
	}
	return nil,errors.New("등록된 GPT API 대화가 아닙니다")
}
