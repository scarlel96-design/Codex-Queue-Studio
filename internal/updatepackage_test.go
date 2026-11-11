package internal
import("archive/zip";"bytes";"encoding/json";"os";"path/filepath";"testing")
func fixtureUpdate(t *testing.T,tamper,extra bool)[]byte{
 t.Helper()
 files:=map[string][]byte{"CodexQueueStudio.exe":bytes.Repeat([]byte("A"),100001),"CodexQueueUpdater.exe":bytes.Repeat([]byte("B"),100001)}
 m:=UpdateManifest{Version:"3.1.0",Files:map[string]string{}}
 for n,d:=range files{m.Files[n]=sha256Text(d)}
 if tamper{files["CodexQueueStudio.exe"][1]='Z'}
 manifest,_:=json.Marshal(m);files["update-manifest.json"]=manifest
 if extra{files["nested/evil.exe"]=[]byte("bad")}
 var buf bytes.Buffer
 writer:=zip.NewWriter(&buf)
 for n,d:=range files{w,e:=writer.Create(n);if e!=nil{t.Fatal(e)};if _,e=w.Write(d);e!=nil{t.Fatal(e)}}
 if e:=writer.Close();e!=nil{t.Fatal(e)}
 return buf.Bytes()
}
func TestVerifiedUpdate(t *testing.T){
 stage,e:=StageUpdateArchive(fixtureUpdate(t,false,false),t.TempDir(),"v3.1.0")
 if e!=nil{t.Fatal(e)}
 if _,e=os.Stat(filepath.Join(stage,"CodexQueueUpdater.exe"));e!=nil{t.Fatal(e)}
}
func TestRejectInvalidUpdate(t *testing.T){
 for _,c:=range []struct{name string;data []byte;version string}{
 {"tamper",fixtureUpdate(t,true,false),"v3.1.0"},
 {"version",fixtureUpdate(t,false,false),"v3.1.1"},
 {"path",fixtureUpdate(t,false,true),"v3.1.0"},
 }{t.Run(c.name,func(t *testing.T){if _,e:=ReadUpdateArchive(c.data,c.version);e==nil{t.Fatal("accepted untrusted ZIP")}})}
}
