package internal
import("encoding/json";"os";"path/filepath";"strings";"testing")
func TestOnlyCodexSessionsAndExactTitle(t *testing.T) {
 home:=t.TempDir();_ =os.MkdirAll(filepath.Join(home,"sessions"),0700)
 list:=[]struct{id,source,name,prompt string}{
 {"00000000-0000-0000-0000-000000000001","cli","명시적으로 지정한 정확한 제목","원문"},
 {"00000000-0000-0000-0000-000000000002","chatgpt","일반 채팅","잡담"},
 {"00000000-0000-0000-0000-000000000003","atlas","GPT 기록","잡담"},
 {"00000000-0000-0000-0000-000000000004","unknown","알 수 없음","메시지"},
 {"00000000-0000-0000-0000-000000000005","app","Guardian review","프로젝트 코드 수정"},
 }
 var index strings.Builder
 for i,item:=range list{
  b,_:=json.Marshal(map[string]string{"id":item.id,"thread_name":item.name})
  index.Write(b);index.WriteByte('\n')
  meta,_:=json.Marshal(map[string]any{"type":"session_meta","payload":map[string]any{"id":item.id,"cwd":"C:/code/project","source":item.source}})
  msg,_:=json.Marshal(map[string]any{"type":"event_msg","payload":map[string]string{"type":"user_message","message":item.prompt}})
  name:=filepath.Join(home,"sessions",string(rune('a'+i))+".jsonl")
  _=os.WriteFile(name,append(append(meta,'\n'),append(msg,'\n')...),0600)
 }
 _=os.WriteFile(filepath.Join(home,"session_index.jsonl"),[]byte(index.String()),0600)
 got,err:=ScanSessions(home,20);if err!=nil{t.Fatal(err)}
 if len(got)!=2{t.Fatalf("expected only Codex threads: %+v",got)}
 names:=map[string]string{}
 for _,s:=range got{names[s.ID]=s.Title}
 if names[list[0].id]!=list[0].name{t.Fatalf("explicit title changed: %+v",names)}
 if names[list[4].id]!=list[4].prompt{t.Fatalf("Guardian placeholder is displayed: %+v",names)}
}
func TestCodexSourceExclusion(t *testing.T){
 cases:=[]struct{kind,origin,parent string;allowed bool}{
 {"\"cli\"","","",true},{"\"vscode\"","","",true},{"\"appServer\"","","",true},
 {"{\"custom\":\"chatgpt\"}","","",false},{"{\"subAgent\":{\"review\":{}}}","","",false},
 {"\"cli\"","","parent-id",false},{"\"vscode\"","chatgpt-desktop","",false},
 {"\"unknown\"","","",false},
 }
 for _,c:=range cases{
  _,ok:=classifySource(json.RawMessage(c.kind),c.origin,c.parent)
  if ok!=c.allowed{t.Errorf("source %s classified %v",c.kind,ok)}
 }
}
func TestServerNameBeatsIndex(t *testing.T){
 result:=MergeSessions([]Session{{ID:"i",Title:"old",TitleSource:"index",File:"local"}},[]Session{{ID:"i",Title:"new",TitleSource:"app-server"}})
 if len(result)!=1||result[0].Title!="new"||result[0].File!="local"{t.Fatalf("%+v",result)}
}
