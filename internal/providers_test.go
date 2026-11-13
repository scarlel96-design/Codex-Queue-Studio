package internal

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
)

type fixtureProvider struct {id string}
func (f fixtureProvider) ID()string{return f.id}
func (f fixtureProvider) Label()string{return "fixture"}
func (f fixtureProvider) Models(context.Context)([]ModelOption,error){return nil,nil}
func (f fixtureProvider) CreateConversation(context.Context)(string,error){return "fixture_1",nil}
func (f fixtureProvider) History(context.Context,string,int)([]ChatMessage,error){return nil,nil}
func (f fixtureProvider) Review(context.Context,string,string,string,string)(string,error){return "",nil}

func TestProviderRegistryIsExtensibleAndRejectsDuplicates(t *testing.T){
	r:=NewProviderRegistry(fixtureProvider{id:"openai"})
	if err:=r.Register(fixtureProvider{id:"gemini"});err!=nil{t.Fatal(err)}
	if err:=r.Register(fixtureProvider{id:"gemini"});err==nil{t.Fatal("duplicate registered")}
	if _,ok:=r.Get("GEMINI");!ok{t.Fatal("provider lookup must be case-insensitive")}
	if ids:=r.IDs();len(ids)!=2||ids[0]!="gemini"||ids[1]!="openai"{t.Fatal(ids)}
}

func TestOpenAIProviderListsAccountModels(t *testing.T){
	server:=httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter,r *http.Request){
		if r.URL.Path!="/v1/models"{t.Fatalf("unexpected %q",r.URL.Path)}
		if r.Header.Get("Authorization")!="Bearer test-key"{t.Error("missing bearer token")}
		w.Header().Set("Content-Type","application/json")
		_=json.NewEncoder(w).Encode(map[string]any{"data":[]map[string]any{
			{"id":"gpt-6","owned_by":"openai","shutdown_date":nil},
			{"id":"o4-mini","owned_by":"openai","shutdown_date":nil},
			{"id":"text-embedding-3-large","shutdown_date":nil},
			{"id":"gpt-image-1","shutdown_date":nil},
			{"id":"gpt-1-obsolete","shutdown_date":"2026-10-23"},
		}})
	}))
	defer server.Close()
	provider:=OpenAIChatProvider{Client:ResponsesClient{Key:"test-key",BaseURL:server.URL+"/v1",HTTP:server.Client()}}
	models,err:=provider.Models(context.Background())
	if err!=nil{t.Fatal(err)}
	if len(models)!=2||models[0].ID!="gpt-6"||models[1].ID!="o4-mini"{t.Fatalf("%+v",models)}
}

func TestOpenAIChatHistoryUsesExplicitConversationID(t *testing.T){
	server:=httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter,r *http.Request){
		if r.URL.Path!="/v1/conversations/conv_alpha/items"{t.Fatalf("unexpected %q",r.URL.Path)}
		if r.URL.Query().Get("limit")!="80"||r.URL.Query().Get("order")!="desc"{t.Fatal(r.URL.RawQuery)}
		_,_=w.Write([]byte(`{"data":[{"type":"message","role":"assistant","content":[{"type":"output_text","text":"검토 완료"}]},{"type":"message","role":"user","content":[{"type":"input_text","text":"결과 검토"}]},{"type":"file_search_call","role":"","content":[]}]}`))
	}))
	defer server.Close()
	provider:=OpenAIChatProvider{Client:ResponsesClient{Key:"test-key",BaseURL:server.URL+"/v1",HTTP:server.Client()}}
	h,err:=provider.History(context.Background(),"conv_alpha",80)
	if err!=nil{t.Fatal(err)}
	if len(h)!=2||h[0].Role!="user"||h[0].Text!="결과 검토"||h[1].Role!="assistant"{t.Fatalf("%+v",h)}
	if _,err:=provider.History(context.Background(),"non-api-id",80);err==nil{t.Fatal("invalid conversation ID accepted")}
}

func TestStatePersistsRegisteredChats(t *testing.T){
	path:=filepath.Join(t.TempDir(),"state.json")
	st,err:=NewStore(path,Settings{});if err!=nil{t.Fatal(err)}
	err=st.Update(func(s *State)error{s.ChatSessions=append(s.ChatSessions,ChatSession{ID:"conv_abc",ProviderID:"openai",Title:"검토"}) ;return nil})
	if err!=nil{t.Fatal(err)}
	loaded,err:=NewStore(path,Settings{});if err!=nil{t.Fatal(err)}
	if a:=loaded.Snapshot().ChatSessions;len(a)!=1||a[0].ID!="conv_abc"{t.Fatalf("%+v",a)}
}
