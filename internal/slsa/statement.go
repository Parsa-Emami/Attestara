// Package slsa performs bounded, inert comparisons of provenance subjects.
// It never verifies signatures, builder identity, source revisions or SLSA levels.
package slsa

import (
 "bytes"
 "encoding/json"
 "errors"
 "fmt"
 "io"
 "regexp"
 "strings"
 "unicode/utf8"
)

const (
 maxStatementBytes = 1024 * 1024
 maxDepth = 32
 maxTokens = 10000
 maxSubjects = 2000
 statementType = "https://in-toto.io/Statement/v1"
 provenanceType = "https://slsa.dev/provenance/v1"
)
var (
 pathPattern = regexp.MustCompile(`^[A-Za-z0-9_-][A-Za-z0-9._-]*(/[A-Za-z0-9_-][A-Za-z0-9._-]*)*`)
 digestPattern = regexp.MustCompile(`^[a-f0-9]{64}$`)
)
// Assessment never asserts provenance authenticity or SLSA compliance.
type Assessment struct {
 SchemaVersion string `json:"schema_version"`
 SubjectBound bool `json:"subject_bound"`
 SubjectsChecked int `json:"subjects_checked"`
 FileBytesChecked bool `json:"file_bytes_checked"`
 SignerTrust string `json:"signer_trust"`
}
type subject struct {
 Name string `json:"name"`
 Digest map[string]string `json:"digest"`
}
type statement struct {
 Type string `json:"_type"`
 PredicateType string `json:"predicateType"`
 Subject []subject `json:"subject"`
 Predicate json.RawMessage `json:"predicate"`
}
// Reject duplicate keys anywhere, even inside an opaque predicate object.
// All inputs are bounded; JSON values are inert and are never executed.
func validateDistinctJSON(data []byte) error {
 d := json.NewDecoder(bytes.NewReader(data))
 d.UseNumber()
 remaining := maxTokens
 var walk func(int) error
 walk = func(depth int) error {
  if depth>maxDepth || remaining<=0 { return errors.New("statement JSON exceeds nesting or token limit") }
  remaining--
  tok,err:=d.Token()
  if err!=nil { return errors.New("statement JSON is malformed") }
  delim,ok:=tok.(json.Delim)
  if !ok { return nil }
  switch delim {
  case '{':
   seen:=make(map[string]struct{})
   for d.More() {
    keyToken,err:=d.Token()
    key,ok:=keyToken.(string)
    if err!=nil || !ok { return errors.New("statement JSON has an invalid object key") }
    if _,exists:=seen[key];exists { return errors.New("statement JSON has duplicate member names") }
    seen[key]=struct{}{}
    remaining--
    if remaining<=0 { return errors.New("statement JSON exceeds token limit") }
    if err:=walk(depth+1);err!=nil { return err }
   }
   if end,err:=d.Token();err!=nil || end!=json.Delim('}') { return errors.New("statement JSON is malformed") }
  case '[':
   for d.More() { if err:=walk(depth+1);err!=nil { return err } }
   if end,err:=d.Token();err!=nil || end!=json.Delim(']') { return errors.New("statement JSON is malformed") }
  default:
   return errors.New("statement JSON has an invalid delimiter")
  }
  return nil
 }
 if err:=walk(0);err!=nil{return err}
 if _,err:=d.Token();err!=io.EOF {return errors.New("statement JSON has trailing data")}
 return nil
}
// Compare validates statement type, SLSA predicate type, portable subject paths
// and canonical SHA-256 hashes. Every claim must match the separately validated
// manifest; at least one must match its designated subject digest.
// The predicate's builder/source assertions remain UNVERIFIED and opaque.
// File bytes must be validated separately with bundle.ReadRoot/ReadArchive.
func Compare(data []byte,expectedFiles map[string]string,subjectDigest string)(Assessment,error){
 zero:=Assessment{}
 if len(data)==0 || len(data)>maxStatementBytes {return zero,errors.New("statement must be nonempty and at most 1 MiB")}
 if !utf8.Valid(data) {return zero,errors.New("statement must be valid UTF-8")}
 if err:=validateDistinctJSON(data);err!=nil {return zero,err}
 var doc statement
 if err:=json.Unmarshal(data,&doc);err!=nil {return zero,errors.New("statement has an invalid shape")}
 if doc.Type!=statementType || doc.PredicateType!=provenanceType {return zero,errors.New("unsupported in-toto statement or SLSA predicate version")}
 if len(doc.Subject)==0 || len(doc.Subject)>maxSubjects {return zero,errors.New("statement subject count must be between 1 and 2000")}
 if len(doc.Predicate)==0 || doc.Predicate[0]!='{' {return zero,errors.New("SLSA predicate must be a JSON object")}
 if len(subjectDigest)!=len("sha256:")+64 || !strings.HasPrefix(subjectDigest,"sha256:") || !digestPattern.MatchString(strings.TrimPrefix(subjectDigest,"sha256:")) {return zero,errors.New("expected subject digest must be canonical SHA-256")}
 matched,designated:=true,false
 seen:=make(map[string]struct{},len(doc.Subject))
 for _,s:=range doc.Subject{
  if len(s.Name)>4096 || !pathPattern.MatchString(s.Name){return zero,errors.New("subject name is not a portable manifest path")}
  if _,exists:=seen[s.Name];exists{return zero,errors.New("statement repeats a subject name")}
  seen[s.Name]=struct{}{}
  hash,has:=s.Digest["sha256"]
  if !has || !digestPattern.MatchString(hash) {return zero,fmt.Errorf("subject %d has no canonical SHA-256 digest",len(seen))}
  canonical:="sha256:"+hash
  if expectedFiles[s.Name]!=canonical {matched=false}
  if expectedFiles[s.Name]==subjectDigest && canonical==subjectDigest {designated=true}
 }
 return Assessment{SchemaVersion:"1",SubjectBound:matched&&designated,SubjectsChecked:len(doc.Subject),FileBytesChecked:false,SignerTrust:"unverified"},nil
}
