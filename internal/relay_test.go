package internal

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLatestCompletionUsesTerminalEventAndDedupIdentity(t *testing.T) {
	path:=filepath.Join(t.TempDir(),"rollout.jsonl")
	data:=strings.Join([]string{
		`{"timestamp":"2026-11-13T00:00:00Z","type":"event_msg","payload":{"type":"task_started"}}`,
		`{"timestamp":"2026-11-13T00:00:01Z","type":"event_msg","payload":{"type":"task_complete","turn_id":"turn-old","last_agent_message":"previous work"}}`,
		`{"timestamp":"2026-11-13T00:00:02Z","type":"event_msg","payload":{"type":"task_complete","turn_id":"turn-empty","last_agent_message":null}}`,
		`{"timestamp":"2026-11-13T00:00:03Z","type":"event_msg","payload":{"type":"task_complete","turn_id":"turn-latest","last_agent_message":"verified result"}}`,
	} ,"\n")+"\n"
	if err:=os.WriteFile(path,[]byte(data),0600);err!=nil{t.Fatal(err)}
	got,err:=LatestCompletion(path);if err!=nil{t.Fatal(err)}
	if got.TurnID!="turn-latest"||got.Message!="verified result"{t.Fatalf("%+v",got)}
}

func TestResponsesClientUsesSeparateConversationAndParsesReview(t *testing.T) {
	var requests []string
	srv:=httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter,r *http.Request) {
		if r.Header.Get("Authorization")!="Bearer test-token"{t.Error("authorization")}
		requests=append(requests,r.URL.Path)
		w.Header().Set("Content-Type","application/json")
		switch r.URL.Path {
		case "/v1/conversations":
			_,_=w.Write([]byte(`{"id":"conv_test123"}`))
		case "/v1/responses":
			var req struct{ Conversation string `json:"conversation"`; Model string `json:"model"`; Store bool `json:"store"` }
			if err:=json.NewDecoder(r.Body).Decode(&req);err!=nil{t.Error(err)}
			if req.Conversation!="conv_test123"||req.Model!="test-model"||!req.Store{t.Errorf("incorrect review request %+v",req)}
			_,_=w.Write([]byte(`{"status":"completed","output":[{"type":"message","content":[{"type":"output_text","text":"문제없음"}]}]}`))
		default: w.WriteHeader(404)
		}
	}))
	defer srv.Close()
	client:=ResponsesClient{Key:"test-token",BaseURL:srv.URL+"/v1",HTTP:srv.Client()}
	conv,err:=client.CreateConversation(context.Background());if err!=nil{t.Fatal(err)}
	reply,err:=client.Review(context.Background(),conv,"test-model","Codex report","")
	if err!=nil{t.Fatal(err)}
	if reply!="문제없음"{t.Fatal(reply)}
	if len(requests)!=2||requests[0]!="/v1/conversations"||requests[1]!="/v1/responses"{t.Fatal(requests)}
}

func TestRelayRequiresExplicitCodexSessionAndBoundedCycles(t *testing.T) {
	r:=RelayRule{WorkSessionID:"invalid",Model:"test-model",MaxCycles:3}
	if ValidateRelayRule(r)==nil {t.Fatal("invalid Codex session accepted")}
	r.WorkSessionID="123e4567-e89b-12d3-a456-426614174000"
	if err:=ValidateRelayRule(r);err!=nil{t.Fatal(err)}
	r.ConversationID="chatgpt-browser-chat-id"
	if ValidateRelayRule(r)==nil {t.Fatal("ChatGPT UI chat ID accepted")}
	r.ConversationID=""
	r.MaxCycles=0
	if ValidateRelayRule(r)==nil {t.Fatal("unbounded relay accepted")}
}

func TestRelayStoreRestartsUncertainWithoutResending(t *testing.T) {
	path:=filepath.Join(t.TempDir(),"state.json")
	store,err:=NewStore(path,Settings{});if err!=nil{t.Fatal(err)}
	err=store.Update(func(s *State)error{
		s.Relays=[]RelayRule{{ID:"id-1",State:"reply-sending",Enabled:true,LastTurnID:"t-1"}}
		return nil
	})
	if err!=nil{t.Fatal(err)}
	again,err:=NewStore(path,Settings{});if err!=nil{t.Fatal(err)}
	r:=again.Snapshot().Relays[0]
	if r.State!="uncertain"||r.LastTurnID!="t-1"{t.Fatalf("unexpected recovery %+v",r)}
}
