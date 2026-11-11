package internal

import (
 "archive/zip"
 "bytes"
 "crypto/sha256"
 "encoding/hex"
 "encoding/json"
 "errors"
 "fmt"
 "io"
 "os"
 "path/filepath"
 "strings"
)
const MaxUpdateArchive=48<<20
const MaxUpdateBinary=30<<20
var updateFiles=[]string{"CodexQueueStudio.exe","CodexQueueUpdater.exe"}
type UpdateManifest struct {
 Version string `json:"version"`
 Files map[string]string `json:"files"`
}
func sha256Text(data []byte) string {sum:=sha256.Sum256(data);return hex.EncodeToString(sum[:])}
// ReadUpdateArchive only accepts the declared update files and their digests.
func ReadUpdateArchive(raw []byte,version string)(map[string][]byte,error){
 if len(raw)==0||len(raw)>MaxUpdateArchive{return nil,errors.New("업데이트 ZIP 크기가 잘못되었습니다")}
 zr,err:=zip.NewReader(bytes.NewReader(raw),int64(len(raw)))
 if err!=nil{return nil,err}
 files:=make(map[string][]byte,3)
 for _,f:=range zr.File{
  if f.Name!="update-manifest.json"&&f.Name!=updateFiles[0]&&f.Name!=updateFiles[1]{return nil,fmt.Errorf("예상하지 못한 업데이트 파일: %s",f.Name)}
  if _,ok:=files[f.Name];ok{return nil,errors.New("중복 업데이트 파일")}
  if !f.Mode().IsRegular()||f.UncompressedSize64>MaxUpdateBinary{return nil,errors.New("업데이트 파일 유형 또는 크기가 잘못되었습니다")}
  r,err:=f.Open();if err!=nil{return nil,err}
  data,err:=io.ReadAll(io.LimitReader(r,MaxUpdateBinary+1));_ =r.Close()
  if err!=nil{return nil,err}
  if len(data)>MaxUpdateBinary||uint64(len(data))!=f.UncompressedSize64{return nil,errors.New("업데이트 파일 크기 불일치")}
  files[f.Name]=data
 }
 var m UpdateManifest
 if err:=json.Unmarshal(files["update-manifest.json"],&m);err!=nil{return nil,fmt.Errorf("매니페스트 오류: %w",err)}
 if m.Version!=strings.TrimPrefix(version,"v")||len(m.Files)!=len(updateFiles){return nil,errors.New("업데이트 버전 또는 파일 수 불일치")}
 for _,name:=range updateFiles{
  data,ok:=files[name]
  if !ok||len(data)<100000||!strings.EqualFold(sha256Text(data),m.Files[name]){return nil,fmt.Errorf("업데이트 파일 무결성 검증 실패: %s",name)}
 }
 return files,nil
}
func StageUpdateArchive(raw []byte,parent,version string)(string,error){
 files,err:=ReadUpdateArchive(raw,version)
 if err!=nil{return "",err}
 if err:=os.MkdirAll(parent,0700);err!=nil{return "",err}
 stage,err:=os.MkdirTemp(parent,"codex-update-")
 if err!=nil{return "",err}
 success:=false
 defer func(){if !success {_=os.RemoveAll(stage)}}()
 for _,name:=range []string{"CodexQueueStudio.exe","CodexQueueUpdater.exe","update-manifest.json"}{
  if err:=os.WriteFile(filepath.Join(stage,name),files[name],0600);err!=nil{return "",err}
 }
 success=true
 return stage,nil
}
