package main

import("testing";"strings";"codexqueuestudiov3/internal")
func updateReleaseForTest(tag string,pre,complete bool) updateRelease{
 r:=updateRelease{Tag:tag,Prerelease:pre}
 if complete{r.Assets=append(r.Assets,struct{
  Name string `json:"name"`;URL string `json:"browser_download_url"`;Digest string `json:"digest"`;Size int64 `json:"size"`
 }{Name:"CodexQueueStudio-Windows-x64.zip",Size:200000})}
 r.Assets=append(r.Assets,struct{
  Name string `json:"name"`;URL string `json:"browser_download_url"`;Digest string `json:"digest"`;Size int64 `json:"size"`
 }{Name:"CodexQueueStudio-update-Windows-x64.zip",Size:500000,Digest:"sha256:"+strings.Repeat("f",64),URL:"https://github.com/scarlel96-design/Codex-Queue-Studio/releases/download/"+tag+"/CodexQueueStudio-update-Windows-x64.zip"})
 return r
}
func TestPreviewAndProductionVersionTransitions(t *testing.T){
 for _,tt:=range []struct{tag string;pre bool;current string;ok bool}{
 {"v3.2.0",true,"3.1.0",true},
 {"v3.1.0",true,"3.1.0",false},
 {"v1.0.0",false,"3.2.0",true},
 {"v1.0.0",false,"3.1.0",true},
 {"v0.9.0",false,"3.2.0",false},
 {"v3.2.0",true,"1.0.0",false},
 {"v1.0.1",false,"1.0.0",true},
 {"v1.0.0",false,"1.0.0",false},
 }{
  got:=updatePriority(tt.tag,tt.pre,tt.current)>=0
  if got!=tt.ok{t.Errorf("%s -> %s = %v, expected %v",tt.current,tt.tag,got,tt.ok)}
 }
}
func TestUpdateAssetContractAndChannel(t *testing.T){
 current:=updateReleaseForTest("v3.1.0",true,true)
 target:=updateReleaseForTest("v3.2.0",true,true)
 broken:=updateReleaseForTest("v3.3.0",true,false)
 selected,err:=chooseUpdate([]updateRelease{broken,current,target},"3.1.0")
 if err!=nil||selected==nil||selected.Tag!="v3.2.0"||!selected.Prerelease{t.Fatalf("bad preview: %+v %v",selected,err)}
 stable:=updateReleaseForTest("v1.0.0",false,true)
 selected,err=chooseUpdate([]updateRelease{target,stable},"3.2.0")
 if err!=nil||selected==nil||selected.Tag!="v1.0.0"{t.Fatalf("bad promotion: %+v %v",selected,err)}
 selected,err=chooseUpdate([]updateRelease{target},"1.0.0")
 if err!=nil||selected!=nil{t.Fatalf("stable must ignore preview: %+v %v",selected,err)}
 if selected,err=chooseUpdate([]updateRelease{broken},"3.1.0");selected!=nil||err==nil{t.Fatal("invalid package accepted")}
}

func TestActiveMessageBlocksUpdate(t *testing.T){
 core,err:=NewCore(t.TempDir());if err!=nil{t.Fatal(err)}
 err=core.store.Update(func(st *internal.State)error{
  st.Jobs=append(st.Jobs,internal.Job{ID:"active",State:"sending"})
  return nil
 });if err!=nil{t.Fatal(err)}
 if core.CanInstallUpdate()==nil{t.Fatal("active Codex request did not block update")}
 err=core.store.Update(func(st *internal.State)error{st.Jobs[0].State="completed";return nil});if err!=nil{t.Fatal(err)}
 if err=core.CanInstallUpdate();err!=nil{t.Fatal(err)}
}
