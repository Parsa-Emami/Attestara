package slsa
import (
 "bytes"
 "encoding/json"
 "strings"
 "testing"
)
const digestA="aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
const digestB="bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
func fixtureStatement(subjects string)[]byte{return []byte(`{"_type":"https://in-toto.io/Statement/v1","predicateType":"https://slsa.dev/provenance/v1","subject":`+subjects+`,"predicate":{"buildDefinition":{},"runDetails":{}}}`)}
func expectedFiles()map[string]string{return map[string]string{"release.bin":"sha256:"+digestA,"sbom.json":"sha256:"+digestB}}
func TestCompareValidAndMismatchedSubjects(t *testing.T){
 cases:=[]struct{name,subjects string;bound bool}{
  {"single correct",`[{"name":"release.bin","digest":{"sha256":"`+digestA+`"}}]`,true},
  {"two correct",`[{"name":"release.bin","digest":{"sha256":"`+digestA+`"}},{"name":"sbom.json","digest":{"sha256":"`+digestB+`"}}]`,true},
  {"wrong digest",`[{"name":"release.bin","digest":{"sha256":"`+digestB+`"}}]`,false},
  {"missing file",`[{"name":"unknown.bin","digest":{"sha256":"`+digestA+`"}}]`,false},
  {"other file only",`[{"name":"sbom.json","digest":{"sha256":"`+digestB+`"}}]`,false},
  {"other file wrong",`[{"name":"release.bin","digest":{"sha256":"`+digestA+`"}},{"name":"sbom.json","digest":{"sha256":"`+digestA+`"}}]`,false},
 }
 for _,tc:=range cases {t.Run(tc.name,func(t *testing.T){
  doc:=fixtureStatement(tc.subjects)
  original:=bytes.Clone(doc)
  actual,err:=Compare(doc,expectedFiles(),"sha256:"+digestA)
  if err!=nil || actual.SubjectBound!=tc.bound || actual.SchemaVersion!="1" || actual.FileBytesChecked || actual.SignerTrust!="unverified" || actual.SubjectsChecked!=strings.Count(tc.subjects,`"name"`){t.Fatalf("got %+v / %v",actual,err)}
  if !bytes.Equal(doc,original){t.Fatal("input mutated")}
 })}
}
func TestRejectUnsafeStatements(t *testing.T){
 cases:=[]struct{name string;data []byte}{
  {"empty",nil},
  {"duplicate root",[]byte(`{"_type":"x","_type":"x"}`)},
  {"duplicate nested",[]byte(`{"_type":"https://in-toto.io/Statement/v1","predicateType":"https://slsa.dev/provenance/v1","subject":[{"name":"release.bin","digest":{"sha256":"`+digestA+`","sha256":"`+digestB+`"}}],"predicate":{}}`)},
  {"wrong predicate",bytes.Replace(fixtureStatement(`[{"name":"release.bin","digest":{"sha256":"`+digestA+`"}}]`),[]byte("provenance/v1"),[]byte("provenance/v0.2"),1)},
  {"missing predicate",[]byte(`{"_type":"https://in-toto.io/Statement/v1","predicateType":"https://slsa.dev/provenance/v1","subject":[{"name":"release.bin","digest":{"sha256":"`+digestA+`"}}]}`)},
  {"traversal",fixtureStatement(`[{"name":"../escape","digest":{"sha256":"`+digestA+`"}}]`)},
  {"duplicate subject",fixtureStatement(`[{"name":"release.bin","digest":{"sha256":"`+digestA+`"}},{"name":"release.bin","digest":{"sha256":"`+digestA+`"}}]`)},
  {"uppercase hash",fixtureStatement(`[{"name":"release.bin","digest":{"sha256":"`+strings.ToUpper(digestA)+`"}}]`)},
  {"no subjects",fixtureStatement(`[]`)},
  {"trailing",append(fixtureStatement(`[{"name":"release.bin","digest":{"sha256":"`+digestA+`"}}]`),[]byte(" true")...)},
  {"invalid utf8",[]byte{0xff,0xfe}},
  {"oversized",bytes.Repeat([]byte(" "),maxStatementBytes+1)},
 }
 for _,tc:=range cases{t.Run(tc.name,func(t *testing.T){if a,err:=Compare(tc.data,expectedFiles(),"sha256:"+digestA);err==nil || a!=(Assessment{}){t.Fatalf("accepted %+v / %v",a,err)}})}
}
func TestRejectDeepAndUnsupportedDigest(t *testing.T){
 source:=fixtureStatement(`[{"name":"release.bin","digest":{"sha256":"`+digestA+`"}}]`)
 var parsed map[string]json.RawMessage
 if err:=json.Unmarshal(source,&parsed);err!=nil{t.Fatal(err)}
 parsed["predicate"]=json.RawMessage(strings.Repeat(`{"nested":`,maxDepth+3)+`{}`+strings.Repeat(`}`,maxDepth+3))
 deep,err:=json.Marshal(parsed);if err!=nil{t.Fatal(err)}
 if _,err:=Compare(deep,expectedFiles(),"sha256:"+digestA);err==nil{t.Fatal("deep predicate accepted")}
 if _,err:=Compare(source,expectedFiles(),"sha512:"+digestA);err==nil{t.Fatal("unsupported expected digest accepted")}
}
