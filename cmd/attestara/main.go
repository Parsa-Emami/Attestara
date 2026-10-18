package main

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	json "encoding/json/v2"
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"
	"unicode/utf8"

	"attestara/internal/contracts"
	"attestara/internal/decision"
	"attestara/internal/export"
	"attestara/internal/policy"
	"attestara/internal/report"
)

const (
	policyExitCode = 1
	inputExitCode  = 2
	engineExitCode = 3
	maxRuleBytes   = 4096
)

type commandFailure struct {
	code    int
	message string
}

func (e *commandFailure) Error() string { return e.message }

func commandError(code int, message string) error {
	return &commandFailure{code: code, message: message}
}

type policyOutcomeError struct{ decision string }

func (e *policyOutcomeError) Error() string { return "policy decision is " + e.decision }

func commandExitCode(err error) int {
	var outcome *policyOutcomeError
	if errors.As(err, &outcome) {
		return policyExitCode
	}
	var failure *commandFailure
	if errors.As(err, &failure) {
		return failure.code
	}
	var contractError *contracts.Error
	if errors.As(err, &contractError) {
		switch contractError.Code {
		case "invalid_json", "invalid_input", "unsupported_version", "limit_exceeded":
			return inputExitCode
		default:
			return engineExitCode
		}
	}
	return engineExitCode
}

func isDomainCommand(args []string) bool {
	if len(args) == 0 {
		return false
	}
	switch args[0] {
	case "evaluate", "verify", "export":
		return true
	default:
		return false
	}
}

type health struct {
	SchemaVersion int    `json:"schemaVersion"`
	Status        string `json:"status"`
	Scope         string `json:"scope"`
	Version       string `json:"version"`
	GoVersion     string `json:"goVersion"`
}

func currentHealth() health { return health{1, "ok", "engine-process", "0.0.1", runtime.Version()} }

func handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		if r.URL.Path != "/healthz" {
			w.WriteHeader(404)
			io.WriteString(w, `{"error":"not_found"}`)
			return
		}
		if r.Method != "GET" && r.Method != "HEAD" {
			w.Header().Set("Allow", "GET, HEAD")
			w.WriteHeader(405)
			io.WriteString(w, `{"error":"method_not_allowed"}`)
			return
		}
		if r.Method == "GET" {
			_ = writeJSON(w, currentHealth())
		}
	})
}

func run(ctx context.Context, args []string, stdout, stderr io.Writer) error {
	if len(args) > 0 {
		switch args[0] {
		case "evaluate":
			return evaluateCommand(ctx, args[1:], stdout, stderr)
		case "verify":
			return passportCommand(ctx, args[1:], stdout, stderr, false)
		case "export":
			return passportCommand(ctx, args[1:], stdout, stderr, true)
		}
	}
	flags := flag.NewFlagSet("attestara", flag.ContinueOnError)
	flags.SetOutput(stderr)
	address := flags.String("listen", "127.0.0.1:0", "loopback listen address")
	version := flags.Bool("version", false, "print engine process version as JSON")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return errors.New("unexpected arguments; user source execution is unsupported")
	}
	if *version {
		return writeJSON(stdout, currentHealth())
	}
	host, port, err := net.SplitHostPort(*address)
	number, portErr := strconv.Atoi(port)
	if err != nil || portErr != nil || number < 0 || number > 65535 || (host != "127.0.0.1" && host != "::1") {
		return errors.New("listen address must be numeric loopback with port 0..65535")
	}
	if ctx.Err() != nil {
		return nil
	}
	listener, err := net.Listen("tcp", *address)
	if err != nil {
		return err
	}
	defer listener.Close()
	server := &http.Server{Handler: handler(), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 5 * time.Second, WriteTimeout: 5 * time.Second, IdleTimeout: 5 * time.Second, MaxHeaderBytes: 16384}
	served := make(chan error, 1)
	go func() { served <- server.Serve(listener) }()
	if err := writeJSON(stdout, struct {
		Event   string `json:"event"`
		Address string `json:"address"`
	}{"listening", listener.Addr().String()}); err != nil {
		server.Close()
		return err
	}
	select {
	case err := <-served:
		if !errors.Is(err, http.ErrServerClosed) {
			return err
		}
	case <-ctx.Done():
		// https://pkg.go.dev/net/http#Server.Shutdown: wait for shutdown before exiting.
		deadline, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		if err := server.Shutdown(deadline); err != nil {
			server.Close()
			return err
		}
	}
	return nil
}

func evaluateCommand(parent context.Context, args []string, stdout, stderr io.Writer) error {
	flags := flag.NewFlagSet("attestara evaluate", flag.ContinueOnError)
	flags.SetOutput(stderr)
	inputPath := flags.String("input", "", "path to a C04 input JSON file")
	rulePath := flags.String("policy", "", "path to a C14 rule JSON file")
	evidencePath := flags.String("evidence", "", "path to the evidence JSON file; omit to evaluate as unknown")
	freshness := flags.String("freshness", "", "caller-classified evidence freshness: fresh or stale")
	if err := flags.Parse(args); err != nil {
		return commandError(inputExitCode, "invalid evaluate arguments")
	}
	if flags.NArg() != 0 || *inputPath == "" || *rulePath == "" || (*freshness != "fresh" && *freshness != "stale") {
		return commandError(inputExitCode, "evaluate requires --input, --policy and --freshness fresh|stale; --evidence is optional")
	}

	inputJSON, err := readCommandFile(*inputPath, contracts.MaxBundleBytes)
	if err != nil {
		return err
	}
	input, err := contracts.DecodeInput(parent, inputJSON)
	if err != nil {
		return err
	}
	ruleJSON, err := readCommandFile(*rulePath, maxRuleBytes)
	if err != nil {
		return err
	}
	var evidenceBytes []byte
	if *evidencePath != "" {
		evidenceBytes, err = readCommandFile(*evidencePath, contracts.MaxBundleBytes)
		if err != nil {
			return err
		}
	}
	result, err := policy.Evaluate(parent, inputJSON, ruleJSON, evidenceBytes)
	if err != nil {
		return err
	}
	classified := decision.Fresh
	if *freshness == "stale" {
		classified = decision.Stale
	}
	result, err = decision.Explain(parent, result, classified)
	if err != nil {
		return err
	}
	passport, err := report.GeneratePassport(parent, input, result)
	if err != nil {
		return err
	}
	if err := writeJSON(stdout, passport); err != nil {
		return commandError(engineExitCode, "could not write passport output")
	}
	if passport.PolicyDecision != "pass" {
		return &policyOutcomeError{decision: passport.PolicyDecision}
	}
	return nil
}

func passportCommand(ctx context.Context, args []string, stdout, stderr io.Writer, exporting bool) error {
	name := "verify"
	if exporting {
		name = "export"
	}
	flags := flag.NewFlagSet("attestara "+name, flag.ContinueOnError)
	flags.SetOutput(stderr)
	passportPath := flags.String("passport", "", "path to a C19 passport JSON file")
	var inputPath, filesPath, outputPath *string
	if exporting {
		inputPath = flags.String("input", "", "path to a C04 input manifest JSON file")
		filesPath = flags.String("files", "", "root directory of referenced files")
		outputPath = flags.String("output", "", "new ZIP output path")
	}
	if err := flags.Parse(args); err != nil {
		return commandError(inputExitCode, "invalid "+name+" arguments")
	}
	if exporting && (*inputPath != "" || *filesPath != "" || *outputPath != "") {
		if flags.NArg() != 0 || *passportPath != "" || *inputPath == "" || *filesPath == "" || *outputPath == "" {
			return commandError(inputExitCode, "ZIP export requires --input, --files and --output without --passport")
		}
		return exportBundle(ctx, *inputPath, *filesPath, *outputPath)
	}
	if flags.NArg() != 0 || *passportPath == "" {
		return commandError(inputExitCode, name+" requires --passport <path>")
	}
	data, err := readCommandFile(*passportPath, contracts.MaxBundleBytes)
	if err != nil {
		return err
	}
	passport, err := decodePassport(data)
	if err != nil {
		return err
	}
	if !exporting {
		fmt.Fprintln(stderr, "passport digest verified; signer and bundle trust were not assessed")
	}
	if err := writeJSON(stdout, passport); err != nil {
		return commandError(engineExitCode, "could not write passport output")
	}
	if passport.PolicyDecision != "pass" {
		return &policyOutcomeError{decision: passport.PolicyDecision}
	}
	return nil
}

func exportBundle(ctx context.Context, inputPath, filesPath, outputPath string) error {
	manifest, err := readCommandFile(inputPath, contracts.MaxBundleBytes)
	if err != nil {
		return err
	}
	root, err := os.OpenRoot(filesPath)
	if err != nil {
		return commandError(inputExitCode, "file root is unavailable")
	}
	defer root.Close()
	archive, err := export.Build(ctx, manifest, root)
	if err != nil {
		return err
	}
	directory := filepath.Dir(outputPath)
	temporary, err := os.CreateTemp(directory, ".attestara-export-*")
	if err != nil {
		return commandError(engineExitCode, "could not create ZIP output")
	}
	defer os.Remove(temporary.Name())
	if _, err := temporary.Write(archive); err != nil {
		temporary.Close()
		return commandError(engineExitCode, "could not write ZIP output")
	}
	if err := temporary.Sync(); err != nil {
		temporary.Close()
		return commandError(engineExitCode, "could not sync ZIP output")
	}
	if err := temporary.Close(); err != nil {
		return commandError(engineExitCode, "could not close ZIP output")
	}
	if err := os.Link(temporary.Name(), outputPath); err != nil {
		return commandError(engineExitCode, "could not publish ZIP output; target must not exist")
	}
	return nil
}

func readCommandFile(path string, maxBytes int64) ([]byte, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, commandError(inputExitCode, "input file is unavailable")
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() || info.Size() < 0 || info.Size() > maxBytes {
		return nil, commandError(inputExitCode, "input file is invalid or exceeds its byte limit")
	}
	data, err := io.ReadAll(io.LimitReader(file, maxBytes+1))
	if err != nil || int64(len(data)) > maxBytes {
		return nil, commandError(inputExitCode, "input file is unavailable or exceeds its byte limit")
	}
	return data, nil
}

func decodePassport(data []byte) (report.Passport, error) {
	var passport report.Passport
	if len(data) == 0 || int64(len(data)) > contracts.MaxBundleBytes {
		return report.Passport{}, commandError(inputExitCode, "passport is empty or exceeds its byte limit")
	}
	if err := json.Unmarshal(data, &passport, json.RejectUnknownMembers(true)); err != nil {
		return report.Passport{}, commandError(inputExitCode, "passport JSON is invalid")
	}
	if !validPassport(passport) {
		return report.Passport{}, commandError(inputExitCode, "passport version, fields or digest are invalid")
	}
	return passport, nil
}

func validPassport(passport report.Passport) bool {
	if passport.SchemaVersion != "1" || !validCommandText(passport.ReleaseID) || !validCommandText(passport.SourceRevision) ||
		!validCommandDigest(passport.SubjectDigest) || !validCommandDigest(passport.EvidenceDigest) || !validCommandDigest(passport.PolicyDigest) ||
		!validCommandDigest(passport.ReportDigest) || passport.EvidenceReferences == nil || len(passport.EvidenceReferences) > contracts.MaxFiles || passport.Warnings == nil {
		return false
	}
	switch passport.Status {
	case "complete", "partial", "unverified":
	default:
		return false
	}
	switch passport.PolicyDecision {
	case "pass", "fail", "unknown":
	default:
		return false
	}
	if passport.Status == "complete" && passport.PolicyDecision == "unknown" {
		return false
	}
	if passport.Verification != "unverified" && passport.Verification != "rejected" {
		return false
	}
	if passport.Status == "unverified" && passport.Verification != "unverified" {
		return false
	}
	paths := make(map[string]struct{}, len(passport.EvidenceReferences))
	var lastPath, lastDigest string
	for _, reference := range passport.EvidenceReferences {
		if !contracts.ValidManifestPath(reference.Path) || !validCommandDigest(reference.Digest) {
			return false
		}
		if _, exists := paths[reference.Path]; exists {
			return false
		}
		paths[reference.Path] = struct{}{}
		if lastPath != "" && (reference.Path < lastPath || reference.Path == lastPath && reference.Digest < lastDigest) {
			return false
		}
		lastPath, lastDigest = reference.Path, reference.Digest
	}
	for _, warning := range passport.Warnings {
		if !validCommandText(warning) {
			return false
		}
	}
	payload := passport
	payload.ReportDigest = ""
	encoded, err := json.Marshal(payload, json.FormatNilSliceAsNull(true))
	if err != nil {
		return false
	}
	digest := sha256.Sum256(encoded)
	want := "sha256:" + hex.EncodeToString(digest[:])
	return subtle.ConstantTimeCompare([]byte(passport.ReportDigest), []byte(want)) == 1
}

func validCommandText(value string) bool {
	return strings.TrimSpace(value) != "" && utf8.RuneCountInString(value) <= 4096
}

func validCommandDigest(value string) bool {
	if len(value) != len("sha256:")+64 || !strings.HasPrefix(value, "sha256:") || value != strings.ToLower(value) {
		return false
	}
	_, err := hex.DecodeString(value[len("sha256:"):])
	return err == nil
}

func writeJSON(w io.Writer, value any) error {
	data, err := json.Marshal(value, json.FormatNilSliceAsNull(true))
	if err != nil {
		return err
	}
	output := append(data, '\n')
	written, err := w.Write(output)
	if err != nil {
		return err
	}
	if written != len(output) {
		return io.ErrShortWrite
	}
	return nil
}

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	args := os.Args[1:]
	if err := run(ctx, args, os.Stdout, os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, err)
		if isDomainCommand(args) {
			os.Exit(commandExitCode(err))
		}
		os.Exit(1)
	}
}
