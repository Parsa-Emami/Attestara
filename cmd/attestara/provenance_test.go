package main
import (
 "bytes"
 "context"
 "encoding/json"
 "os"
 "path/filepath"
 "strings"
 "testing"
 "attestara/internal/slsa"
)
func TestProvenanceCLIValidAndMismatched(t *testing.T){
 manifest:="../../tests/fixtures/attestara/C30/manifest.json"
 root:="../../tests/fixtures/attestara/C30"
 stmt:="../../tests/fixtures/attestara/V2-01/slsa-provenance.json"
 args:=[]string{"provenance","--input",manifest,"--statement",stmt,"--files",root}
 var out,diag bytes.Buffer
 if err:=run(context.Background(),args,&out,&diag);err!=nil{t.Fatalf("valid: %v %s",err,diag.String())}
 var assessment slsa.Assessment
 if err:=json.Unmarshal(out.Bytes(),&assessment);err!=nil{t.Fatal(err)}
 if !assessment.SubjectBound || !assessment.FileBytesChecked || assessment.SignerTrust!="unverified" || assessment.SubjectsChecked!=1{t.Fatalf("unexpected assessment %+v",assessment)}
 original,err:=os.ReadFile(stmt);if err!=nil{t.Fatal(err)}
 altered:=bytes.Replace(original,[]byte("ecdc32b595e2dc30f73744505e78f4f6888b76c3997098f3f1fdffcb7b7d9cd4"),[]byte(strings.Repeat("0",64)),1)
 if bytes.Equal(original,altered){t.Fatal("fixture mutation did not happen")}
 p:=filepath.Join(t.TempDir(),"mismatch.json")
 if err:=os.WriteFile(p,altered,0600);err!=nil{t.Fatal(err)}
 args[4]=p
 out.Reset();diag.Reset()
 err=run(context.Background(),args,&out,&diag)
 if err==nil || commandExitCode(err)!=inputExitCode{t.Fatalf("mismatch not rejected: %v",err)}
 if err:=json.Unmarshal(out.Bytes(),&assessment);err!=nil || assessment.SubjectBound || !assessment.FileBytesChecked || assessment.SignerTrust!="unverified"{t.Fatalf("bad mismatch result: %+v / %v",assessment,err)}
}
func TestProvenanceCLIRequiresLocalFiles(t *testing.T){
 for _,args:=range [][]string{
  {"provenance"},
  {"provenance","--input","missing","--statement","missing","--files","missing"},
  {"provenance","--input","../../tests/fixtures/attestara/C30/manifest.json","--statement","../../tests/fixtures/attestara/V2-01/slsa-provenance.json"},
 }{
  var out,diag bytes.Buffer
  err:=run(context.Background(),args,&out,&diag)
  if err==nil || commandExitCode(err)!=inputExitCode || out.Len()!=0{t.Fatalf("invalid args accepted %v / %v",args,err)}
 }
}
