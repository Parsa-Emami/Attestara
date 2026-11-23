package main
import (
 "context"
 "flag"
 "io"
 "attestara/internal/contracts"
 "attestara/internal/slsa"
 "attestara/internal/workflow"
)
// provenanceCommand validates actual local files and compares inert SLSA
// subject claims; it never establishes builder identity or signer trust.
func provenanceCommand(ctx context.Context,args []string,stdout,stderr io.Writer)error{
 flags:=flag.NewFlagSet("attestara provenance",flag.ContinueOnError)
 flags.SetOutput(stderr)
 manifestPath:=flags.String("input","","C04 release manifest")
 statementPath:=flags.String("statement","","in-toto Statement v1 with SLSA provenance v1")
 filesPath:=flags.String("files","","confined root containing all manifest files")
 if err:=flags.Parse(args);err!=nil{return commandError(inputExitCode,"invalid provenance arguments")}
 if flags.NArg()!=0 || *manifestPath=="" || *statementPath=="" || *filesPath==""{return commandError(inputExitCode,"provenance requires --input, --statement and --files")}
 manifest,err:=readCommandFile(*manifestPath,contracts.MaxBundleBytes)
 if err!=nil{return err}
 input,err:=contracts.DecodeInput(ctx,manifest)
 if err!=nil{return err}
 files,err:=workflow.ReadFiles(ctx,manifest,*filesPath)
 if err!=nil{return err}
 if err:=workflow.ValidateSBOM(input,files);err!=nil{return err}
 statement,err:=readCommandFile(*statementPath,1024*1024)
 if err!=nil{return err}
 expected:=make(map[string]string,len(input.Files))
 for _,file:=range input.Files{expected[file.Path]=file.Digest}
 assessment,err:=slsa.Compare(statement,expected,input.SubjectDigest)
 if err!=nil{return commandError(inputExitCode,"invalid SLSA provenance statement: "+err.Error())}
 if err:=ctx.Err();err!=nil{return commandError(engineExitCode,"provenance comparison cancelled")}
 assessment.FileBytesChecked=true
 if err:=writeJSON(stdout,assessment);err!=nil{return commandError(engineExitCode,"could not write provenance assessment")}
 if !assessment.SubjectBound{return commandError(inputExitCode,"SLSA subject claims do not match the validated release files")}
 return nil
}
