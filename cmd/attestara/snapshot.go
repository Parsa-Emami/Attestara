package main

import (
	"archive/zip"
	"attestara/internal/storage"
	"bytes"
	"context"
	"crypto/ed25519"
	"encoding/hex"
	json "encoding/json/v2"
	"errors"
	"flag"
	"io"
	"io/fs"

	"attestara/internal/bundle"
	"attestara/internal/contracts"
	"attestara/internal/report"
	"attestara/internal/verify"
	"attestara/internal/workflow"
)

func outputPassport(stdout io.Writer, p report.Passport) error {
	if err := writeJSON(stdout, p); err != nil {
		return commandError(engineExitCode, "could not write passport output")
	}
	if p.PolicyDecision != "pass" {
		return &policyOutcomeError{decision: p.PolicyDecision}
	}
	return nil
}

func snapshotCommand(ctx context.Context, args []string, stdout, stderr io.Writer, restore bool) error {
	flags := flag.NewFlagSet("snapshot", flag.ContinueOnError)
	flags.SetOutput(stderr)
	store := flags.String("store", "", "local store")
	id := flags.String("operation-id", "", "immutable operation identifier")
	source, target := "", ""
	var backup, output *string
	if restore {
		backup = flags.String("backup", "", "source backup directory")
	} else {
		output = flags.String("output", "", "new backup directory")
	}
	if err := flags.Parse(args); err != nil {
		return commandError(inputExitCode, "invalid snapshot arguments")
	}
	if restore {
		source, target = *backup, *store
	} else {
		source, target = *store, *output
	}
	if flags.NArg() != 0 || source == "" || target == "" || *id == "" {
		return commandError(inputExitCode, "snapshot requires source, new destination and --operation-id")
	}
	passport, err := workflow.Copy(ctx, source, *id, target)
	if err != nil {
		return commandError(engineExitCode, "snapshot unavailable or destination exists; existing data was not replaced")
	}
	return outputPassport(stdout, passport)
}

type bundleAssessment struct {
	SchemaVersion      string `json:"schema_version"`
	FileIntegrity      string `json:"file_integrity"`
	CryptographicValid bool   `json:"cryptographic_valid"`
	SignerTrusted      bool   `json:"signer_trusted"`
	ManifestTrusted    bool   `json:"manifest_trusted"`
}

func verifyCommand(ctx context.Context, args []string, stdout, stderr io.Writer) error {
	flags := flag.NewFlagSet("verify", flag.ContinueOnError)
	flags.SetOutput(stderr)
	passportPath := flags.String("passport", "", "C19 passport")
	inputPath := flags.String("input", "", "C04 manifest")
	filesPath := flags.String("files", "", "confined root")
	archivePath := flags.String("archive", "", "C09 ZIP")
	signaturePath := flags.String("signature", "", "detached signature descriptor JSON")
	trustPath := flags.String("trust", "", "independent configured signer policy JSON")
	store := flags.String("store", "", "local store")
	id := flags.String("operation-id", "", "immutable operation identifier")
	if err := flags.Parse(args); err != nil || flags.NArg() != 0 {
		return commandError(inputExitCode, "invalid verify arguments")
	}
	full := *inputPath != "" || *filesPath != "" || *archivePath != "" || *signaturePath != "" || *trustPath != ""
	if *store != "" || *id != "" {
		if full || *passportPath != "" || *store == "" || *id == "" {
			return commandError(inputExitCode, "snapshot verify requires only --store and --operation-id")
		}
		r, err := workflow.Load(ctx, *store, *id)
		if err != nil {
			return commandError(engineExitCode, "snapshot unavailable or corrupt")
		}
		p, err := workflow.Passport(ctx, r)
		if err != nil {
			return err
		}
		return outputPassport(stdout, p)
	}
	if !full {
		return passportCommand(ctx, args, stdout, stderr, false)
	}
	if (*archivePath == "" && (*inputPath == "" || *filesPath == "")) || (*archivePath != "" && (*inputPath != "" || *filesPath != "")) {
		return commandError(inputExitCode, "verify requires --input and --files, or --archive")
	}
	var manifest []byte
	var input contracts.Input
	var err error
	if *archivePath != "" {
		data, e := readCommandFile(*archivePath, contracts.MaxBundleBytes)
		if e != nil {
			return e
		}
		input, err = bundle.ReadArchive(data)
		if err != nil {
			return err
		}
		r, e := zip.NewReader(bytes.NewReader(data), int64(len(data)))
		if e != nil {
			return commandError(inputExitCode, "invalid ZIP")
		}
		manifest, e = fs.ReadFile(r, "manifest.json")
		if e != nil {
			return commandError(inputExitCode, "ZIP manifest unavailable")
		}
	} else {
		manifest, err = readCommandFile(*inputPath, contracts.MaxBundleBytes)
		if err != nil {
			return err
		}
		if _, err = workflow.ReadFiles(ctx, manifest, *filesPath); err != nil {
			return err
		}
		input, err = contracts.DecodeInput(ctx, manifest)
		if err != nil {
			return err
		}
	}
	var passport *report.Passport
	if *passportPath != "" {
		b, e := readCommandFile(*passportPath, contracts.MaxBundleBytes)
		if e != nil {
			return e
		}
		p, e := decodePassport(b)
		if e != nil {
			return e
		}
		if !report.MatchesInput(p, input) {
			return commandError(inputExitCode, "passport binding differs from manifest")
		}
		passport = &p
	}
	policy := verify.TrustPolicy{}
	if *trustPath != "" {
		b, e := readCommandFile(*trustPath, 64*1024)
		if e != nil {
			return e
		}
		var configuration struct {
			SchemaVersion  string            `json:"schema_version"`
			TrustedSigners map[string]string `json:"trusted_signers"`
		}
		if e = json.Unmarshal(b, &configuration, json.RejectUnknownMembers(true)); e != nil || configuration.SchemaVersion != "1" || configuration.TrustedSigners == nil {
			return commandError(inputExitCode, "invalid independent trust configuration")
		}
		for id, text := range configuration.TrustedSigners {
			key, e := hex.DecodeString(text)
			if e != nil || len(key) != ed25519.PublicKeySize || !validCommandText(id) {
				return commandError(inputExitCode, "invalid configured signer key")
			}
			policy[id] = key
		}
	}
	var signer string
	var key, signature []byte
	if *signaturePath != "" {
		b, e := readCommandFile(*signaturePath, 16384)
		if e != nil {
			return e
		}
		var detached struct {
			SchemaVersion string `json:"schema_version"`
			SignerID      string `json:"signer_id"`
			PublicKey     string `json:"public_key_hex"`
			Signature     string `json:"signature_hex"`
		}
		if e = json.Unmarshal(b, &detached, json.RejectUnknownMembers(true)); e != nil || detached.SchemaVersion != "1" {
			return commandError(inputExitCode, "invalid signature descriptor")
		}
		signer = detached.SignerID
		key, e = hex.DecodeString(detached.PublicKey)
		if e != nil {
			return commandError(inputExitCode, "invalid signature key")
		}
		signature, e = hex.DecodeString(detached.Signature)
		if e != nil || len(signature) == 0 {
			return commandError(inputExitCode, "invalid signature bytes")
		}
	}
	assessment, err := verify.VerifySignature(ctx, manifest, signer, key, signature, policy)
	if err != nil {
		return err
	}
	summary := bundleAssessment{"1", "verified", assessment.CryptographicValid, assessment.SignerTrusted, assessment.Trusted()}
	if len(signature) > 0 && !assessment.CryptographicValid {
		return commandError(inputExitCode, "manifest signature rejected")
	}
	if passport != nil {
		if err := writeJSON(stderr, summary); err != nil {
			return commandError(engineExitCode, "could not write assessment")
		}
		return outputPassport(stdout, *passport)
	}
	if err := writeJSON(stdout, summary); err != nil {
		return commandError(engineExitCode, "could not write assessment")
	}
	return nil
}

// Open/Close reconciles and validates the complete bounded store without keeping
// its writer lock while the operator uses the CLI. This is a point-in-time probe.
func probeStore(path string) string {
	a, err := storage.Open(path)
	if errors.Is(err, storage.ErrLocked) {
		return "locked"
	}
	if errors.Is(err, storage.ErrCorrupt) {
		return "corrupt"
	}
	if err != nil {
		return "unavailable"
	}
	if a.Close() != nil {
		return "unavailable"
	}
	return "ready"
}
