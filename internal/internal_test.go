package internal

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestMain(m *testing.M) {
	if os.Getenv("CODEX_QUEUE_FAKE_RPC") != "" {
		fakeServer()
		os.Exit(0)
	}
	os.Exit(m.Run())
}
func fakeServer() {
	scanner := bufio.NewScanner(os.Stdin)
	w := bufio.NewWriter(os.Stdout)
	for scanner.Scan() {
		var v struct {
			ID     int             `json:"id"`
			Method string          `json:"method"`
			Params json.RawMessage `json:"params"`
		}
		if json.Unmarshal(scanner.Bytes(), &v) != nil {
			continue
		}
		if v.ID <= 0 {
			continue
		}
		reply := map[string]any{"id": v.ID, "result": map[string]any{}}
		switch v.Method {
		case "thread/list":
			reply["result"] = map[string]any{"data": []map[string]any{{"id": "11111111-1111-1111-1111-111111111111", "name": "Guardian review", "preview": "스타크래프트 캠페인 설계", "cwd": "/projects/game", "source": "vscode", "updatedAt": time.Now().Unix()}}, "nextCursor": nil}
		case "thread/resume":
			reply["result"] = map[string]any{"thread": map[string]any{"id": "11111111-1111-1111-1111-111111111111"}}
		case "turn/start":
			reply["result"] = map[string]any{"turn": map[string]any{"id": "turn-test"}}
		}
		data, _ := json.Marshal(reply)
		fmt.Fprintln(w, string(data))
		w.Flush()
		if v.Method == "turn/start" {
			if os.Getenv("CODEX_QUEUE_FAKE_RPC") == "drop" {
				return
			}
			fmt.Fprintln(w, `{"method":"turn/completed","params":{"turn":{"id":"turn-test","status":"completed"}}}`)
			w.Flush()
		}
	}
}

func TestGuardianReviewTitleFallback(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "sessions", "2026", "11", "11")
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	id := "11111111-1111-1111-1111-111111111111"
	index := `{"id":"` + id + `","thread_name":"Guardian review"}` + "\n"
	if err := os.WriteFile(filepath.Join(root, "session_index.jsonl"), []byte(index), 0600); err != nil {
		t.Fatal(err)
	}
	content := `{"type":"session_meta","payload":{"id":"` + id + `","cwd":"C:/projects/game","source":"app"}}` + "\n" +
		`{"type":"event_msg","payload":{"type":"user_message","message":"<environment_context>system only</environment_context>"}}` + "\n" +
		`{"type":"event_msg","payload":{"type":"user_message","message":"스타크래프트 캠페인 맵 제작 진행해줘"}}` + "\n"
	file := filepath.Join(dir, "rollout.jsonl")
	if err := os.WriteFile(file, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
	out, err := ScanSessions(root, 100)
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 1 || out[0].Title != "스타크래프트 캠페인 맵 제작 진행해줘" || out[0].TitleSource != "user" {
		t.Fatalf("wrong Guardian fallback %+v", out)
	}
	if out[0].Source != "Codex 앱" {
		t.Fatal(out[0].Source)
	}
}
func TestCustomTitleRetained(t *testing.T) {
	dir := t.TempDir()
	sessions := filepath.Join(dir, "sessions")
	_ = os.MkdirAll(sessions, 0700)
	id := "11111111-1111-1111-1111-111111111111"
	_ = os.WriteFile(filepath.Join(dir, "session_index.jsonl"), []byte(`{"id":"`+id+`","thread_name":"프로젝트 M01"}`+"\n"), 0600)
	_ = os.WriteFile(filepath.Join(sessions, "a.jsonl"), []byte(`{"type":"session_meta","payload":{"id":"`+id+`","cwd":"/p","source":"cli"}}`+"\n"+`{"type":"event_msg","payload":{"type":"user_message","message":"기본 기획"}}`+"\n"), 0600)
	out, _ := ScanSessions(dir, 10)
	if len(out) != 1 || out[0].Title != "프로젝트 M01" {
		t.Fatalf("incorrect custom title %+v", out)
	}
}
func TestHistoryDedupAndBound(t *testing.T) {
	path := filepath.Join(t.TempDir(), "hist.jsonl")
	data := `{"type":"event_msg","payload":{"type":"user_message","message":"안녕"}}` + "\n" + `{"type":"response_item","payload":{"type":"message","role":"user","content":[{"type":"input_text","text":"안녕"}]}}` + "\n" + `{"type":"response_item","payload":{"type":"message","role":"assistant","content":[{"type":"output_text","text":"반갑습니다"}]}}` + "\n"
	_ = os.WriteFile(path, []byte(data), 0600)
	out, err := ReadHistory(path, 100)
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 2 || out[0].Text != "안녕" || out[1].Text != "반갑습니다" {
		t.Fatalf("bad history %#v", out)
	}
}
func TestMergeSessionTitle(t *testing.T) {
	l := []Session{{ID: "1", Title: "제목 없음 · Guardian review", TitleSource: "unresolved", File: "local.jsonl"}}
	r := []Session{{ID: "1", Title: "내가 만든 앱", TitleSource: "app-server"}}
	a := MergeSessions(l, r)
	if len(a) != 1 || a[0].Title != "내가 만든 앱" || a[0].File != "local.jsonl" {
		t.Fatalf("merge wrong %+v", a)
	}
}
func TestAppServerSendAndListMock(t *testing.T) {
	t.Setenv("CODEX_QUEUE_FAKE_RPC", "1")
	s := AppServer{Executable: os.Args[0], CodexHome: t.TempDir()}
	ctx, cancel := context.WithTimeout(context.Background(), 6*time.Second)
	defer cancel()
	list, err := s.List(ctx, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 || list[0].Title != "스타크래프트 캠페인 설계" {
		t.Fatalf("list %+v", list)
	}
	out, err := s.Send(ctx, "11111111-1111-1111-1111-111111111111", "예약된 메시지\n; rm -rf /")
	if err != nil {
		t.Fatalf("send %v %q", err, out)
	}
	if !strings.Contains(out, "turn/completed") {
		t.Fatalf("unexpected out %q", out)
	}
}
func TestAppServerDisconnectAfterAcceptanceIsUncertain(t *testing.T) {
	t.Setenv("CODEX_QUEUE_FAKE_RPC", "drop")
	s := AppServer{Executable: os.Args[0], CodexHome: t.TempDir()}
	ctx, cancel := context.WithTimeout(context.Background(), 6*time.Second)
	defer cancel()
	detail, err := s.Send(ctx, "11111111-1111-1111-1111-111111111111", "Check status")
	if err == nil || !strings.Contains(detail, "completion unknown") {
		t.Fatalf("accepted turn must be uncertain, detail=%q err=%v", detail, err)
	}
}

func TestCrashRecovery(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	s, err := NewStore(path, Settings{CodexHome: "/tmp"})
	if err != nil {
		t.Fatal(err)
	}
	cipher, err := SealMessage("작업 메시지")
	if err != nil {
		t.Fatal(err)
	}
	_ = s.Update(func(st *State) error {
		st.Jobs = append(st.Jobs, Job{ID: NewID(), State: "sending", Cipher: cipher})
		return nil
	})
	b, _ := os.ReadFile(path)
	if strings.Contains(string(b), "작업 메시지") {
		t.Fatal("plaintext persisted")
	}
	restored, err := NewStore(path, Settings{})
	if err != nil {
		t.Fatal(err)
	}
	if restored.Snapshot().Jobs[0].State != "uncertain" {
		t.Fatal("unsafe auto resend")
	}
}
func TestSessionIDValidation(t *testing.T) {
	if !ValidSessionID("00000000-0000-0000-0000-000000000001") {
		t.Fatal("valid uuid rejected")
	}
	if ValidSessionID("1; powershell -Command whoami") {
		t.Fatal("injection accepted")
	}
}
