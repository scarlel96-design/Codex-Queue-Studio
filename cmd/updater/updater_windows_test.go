//go:build windows
package main
import("bytes";"os";"path/filepath";"testing")
func TestAtomicExecutableWrite(t *testing.T){
 dir:=t.TempDir()
 content:=bytes.Repeat([]byte("codex"),1000)
 if err:=atomicWriteExecutable(dir,"CodexQueueStudio.exe",content);err!=nil{t.Fatal(err)}
 stored,err:=os.ReadFile(filepath.Join(dir,"CodexQueueStudio.exe"))
 if err!=nil||!bytes.Equal(content,stored){t.Fatal("atomic copy mismatch",err)}
 entries,err:=os.ReadDir(dir);if err!=nil{t.Fatal(err)}
 if len(entries)!=1{t.Fatalf("temporary update file leaked: %+v",entries)}
}
