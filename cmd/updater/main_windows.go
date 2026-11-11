//go:build windows

// Codex Queue Studio update helper. Replaces only a verified, staged executable.
//
package main

import (
	"errors"
 "crypto/sha256"
 "encoding/hex"
 "encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"
	"unsafe"
 "codexqueuestudiov3/internal"
)

func message(s string) {
	u := syscall.NewLazyDLL("user32.dll").NewProc("MessageBoxW")
	p, _ := syscall.UTF16PtrFromString(s)
	t, _ := syscall.UTF16PtrFromString("Codex Queue Studio Updater")
	u.Call(0, uintptr(unsafe.Pointer(p)), uintptr(unsafe.Pointer(t)), 0x10)
}
func wait(pid int) error {
	kernel := syscall.NewLazyDLL("kernel32.dll")
	open := kernel.NewProc("OpenProcess")
	h, _, err := open.Call(0x00100000, 0, uintptr(pid))
	if h == 0 {
 if errors.Is(err,syscall.Errno(87)){return nil} // parent already exited
 return fmt.Errorf("프로세스를 열지 못했습니다: %w",err)
}
	defer kernel.NewProc("CloseHandle").Call(h)
	r, _, e := kernel.NewProc("WaitForSingleObject").Call(h, 90000)
	if r != 0 {
		return fmt.Errorf("기존 프로그램 종료 대기 실패: %w", e)
	}
	return nil
}
func verifyStage(stage string) error {
 manifest,err:=os.ReadFile(filepath.Join(stage,"update-manifest.json"))
 if err!=nil{return err}
 var m internal.UpdateManifest
 if err=json.Unmarshal(manifest,&m);err!=nil{return err}
 if m.Version==""||len(m.Files)!=2{return errors.New("잘못된 매니페스트")}
 for _,name:=range []string{"CodexQueueStudio.exe","CodexQueueUpdater.exe"} {
  data,err:=os.ReadFile(filepath.Join(stage,name));if err!=nil{return err}
  if len(data)<100000||len(data)>internal.MaxUpdateBinary{return errors.New("잘못된 업데이트 실행 파일 크기")}
  digest:=sha256.Sum256(data)
  if !strings.EqualFold(hex.EncodeToString(digest[:]),m.Files[name]) {return fmt.Errorf("매니페스트 해시 불일치: %s",name)}
 }
 return nil
}
// atomicWriteExecutable prevents a crash from leaving a partially written EXE.
func atomicWriteExecutable(directory,name string,data []byte)error{
 tmp,err:=os.CreateTemp(directory,".cqs-incoming-*.tmp")
 if err!=nil{return err}
 path:=tmp.Name()
 defer os.Remove(path)
 if err:=tmp.Chmod(0700);err!=nil{_ =tmp.Close();return err}
 if _,err=tmp.Write(data);err!=nil{_ =tmp.Close();return err}
 if err=tmp.Sync();err!=nil{_ =tmp.Close();return err}
 if err=tmp.Close();err!=nil{return err}
 return os.Rename(path,filepath.Join(directory,name))
}
func install(pid int,orig,stage string)error{
 if pid<=0||!filepath.IsAbs(orig)||!filepath.IsAbs(stage){return errors.New("업데이트 경로 또는 PID가 올바르지 않습니다")}
 if !strings.EqualFold(filepath.Base(orig),"CodexQueueStudio.exe"){return errors.New("대상 실행 파일명이 올바르지 않습니다")}
 if err:=verifyStage(stage);err!=nil{return err}
 if err:=wait(pid);err!=nil{return err}
 directory:=filepath.Dir(orig)
 names:=[]string{"CodexQueueStudio.exe","CodexQueueUpdater.exe"}
 backed:=[]string{}
 replaced:=[]string{}
 rollback:=func(){
  for _,name:=range replaced{_ =os.Remove(filepath.Join(directory,name))}
  for _,name:=range backed{_ =os.Rename(filepath.Join(directory,name)+".backup",filepath.Join(directory,name))}
 }
 for _,name:=range names{
  dst:=filepath.Join(directory,name)
  if _,err:=os.Stat(dst);err==nil{
   backup:=dst+".backup";_ =os.Remove(backup)
   var moveErr error
   for i:=0;i<30;i++{
    moveErr=os.Rename(dst,backup)
    if moveErr==nil{break}
    time.Sleep(500*time.Millisecond)
   }
   if moveErr!=nil{rollback();return fmt.Errorf("백업 실패: %s: %w",name,moveErr)}
   backed=append(backed,name)
  }
  input,err:=os.ReadFile(filepath.Join(stage,name))
  if err!=nil{rollback();return err}
  if err:=atomicWriteExecutable(directory,name,input);err!=nil{rollback();return fmt.Errorf("적용 실패: %s: %w",name,err)}
  replaced=append(replaced,name)
 }
 cmd:=exec.Command(orig)
 if err:=cmd.Start();err!=nil{rollback();return fmt.Errorf("재실행 실패: %w",err)}
 if err:=awaitAppWindow(cmd.Process,30*time.Second);err!=nil{
  _=cmd.Process.Kill()
  _,_=cmd.Process.Wait()
  rollback()
  return fmt.Errorf("업데이트 후 시작 검사 실패: %w",err)
 }
 _=cmd.Process.Release()
 for _,name:=range backed{_ =os.Remove(filepath.Join(directory,name)+".backup")}
 _=os.RemoveAll(stage)
 return nil
}
// Confirm the new native window belongs to the launched PID.
// File replacement alone is not sufficient to declare update success.
func awaitAppWindow(proc *os.Process,timeout time.Duration)error{
 user:=syscall.NewLazyDLL("user32.dll")
 find:=user.NewProc("FindWindowW")
 owner:=user.NewProc("GetWindowThreadProcessId")
 title,_:=syscall.UTF16PtrFromString("CodexQueueStudioNativeV3")
 end:=time.Now().Add(timeout)
 for time.Now().Before(end){
  hwnd,_,_:=find.Call(uintptr(unsafe.Pointer(title)),0)
  if hwnd!=0{
   var pid uint32
   owner.Call(hwnd,uintptr(unsafe.Pointer(&pid)))
   if pid==uint32(proc.Pid){return nil}
  }
  h,_,_:=syscall.NewLazyDLL("kernel32.dll").NewProc("OpenProcess").Call(0x00100000,0,uintptr(proc.Pid))
  if h==0{return errors.New("앱 프로세스가 종료되었습니다")}
  waited,_,_:=syscall.NewLazyDLL("kernel32.dll").NewProc("WaitForSingleObject").Call(h,0)
  syscall.NewLazyDLL("kernel32.dll").NewProc("CloseHandle").Call(h)
  if waited==0{return errors.New("앱 프로세스가 즉시 종료되었습니다")}
  time.Sleep(350*time.Millisecond)
 }
 return errors.New("새 앱 창 확인 시간 초과")
}
func main() {
	pid := flag.Int("wait-pid", 0, "")
	target := flag.String("target", "", "")
	source := flag.String("source", "", "")
	flag.Parse()
	if err := install(*pid, *target, *source); err != nil {
		message(err.Error())
	}
}
