// Package storage implements the bounded-memory, single-host ADR-0003 file port.
// Exact JSON bytes are retained because C04 input_digest includes whitespace.
package storage

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	json "encoding/json/v2"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"unicode/utf8"

	"attestara/internal/contracts"
)

var (
	ErrLocked      = errors.New("storage writer is locked")
	ErrConflict    = errors.New("operation ID already has different content")
	ErrNotFound    = errors.New("operation not found")
	ErrCorrupt     = errors.New("storage state is missing or corrupt")
	ErrInvalid     = errors.New("invalid storage transaction")
	ErrUnavailable = errors.New("storage I/O unavailable")
	ErrRecovery    = errors.New("close and reopen storage to reconcile an uncertain write")
	ErrClosed      = errors.New("storage is closed")
	ErrQuota       = errors.New("storage quota exceeded")
)

// Record carries the existing C04 wire contracts and manifest-path-keyed bytes.
// Callers must not mutate its slices/maps while Commit runs. Owner is metadata,
// never authentication. Load returns independent bytes, validated with C04.
type Record struct {
	OperationID string
	InputJSON   []byte
	ResultJSON  []byte
	Files       map[string][]byte
}

type Port interface {
	Commit(context.Context, Record) error
	Load(context.Context, string) (Record, error)
	Close() error
}

// Adapter owns an exclusive OS lock until Close. Calls do not queue internally.
type Adapter struct {
	root              *os.Root
	lock              *os.File
	mu                sync.Mutex
	closed, uncertain bool
	checkpoint        func(string) error // Deterministic syscall failures/process exits in package tests.
}

var _ Port = (*Adapter)(nil)

const header = "{\"schema_version\":1}\n"

// Two 100 MiB wire documents expand by 4/3 in base64; the remaining space
// bounds the operation ID and envelope.
const maxLine = int(3 * contracts.MaxBundleBytes)

const (
	maxOperations    = 1000
	maxArtifactRefs  = 10000
	maxHistoryBytes  = 1 << 30
	maxObjectBytes   = 1 << 30
	maxManifestBytes = 1 << 30
)

type entry struct {
	OperationID string `json:"operation_id"`
	Input       []byte `json:"input"`
	Result      []byte `json:"result"`
	Checksum    string `json:"checksum,omitempty"`
}

func digest(b []byte) string  { s := sha256.Sum256(b); return "sha256:" + hex.EncodeToString(s[:]) }
func checksum(e entry) string { e.Checksum = ""; b, _ := json.Marshal(e); return digest(b) }
func storageError(err error) error {
	var p *os.PathError
	if errors.As(err, &p) {
		err = p.Err
	}
	return fmt.Errorf("%w: %w", ErrUnavailable, err)
}
func (a *Adapter) point(name string) error {
	if a.checkpoint != nil {
		return a.checkpoint(name)
	}
	return nil
}
func syncDir(root *os.Root, name string) error {
	f, err := root.Open(name)
	if err != nil {
		return err
	}
	defer f.Close()
	return f.Sync()
}
func regular(root *os.Root, name string) error {
	s, err := root.Lstat(name)
	if err != nil {
		return err
	}
	if !s.Mode().IsRegular() || s.Mode().Perm()&0077 != 0 || s.Sys().(*syscall.Stat_t).Nlink != 1 || s.Sys().(*syscall.Stat_t).Uid != uint32(os.Geteuid()) {
		return ErrCorrupt
	}
	return nil
}

// Open creates a private store in an existing parent directory. It refuses
// symlinks, shared permissions, unknown managed entries and missing references.
// It never replaces a corrupt manifest with an empty snapshot.
func Open(dir string) (_ *Adapter, err error) {
	dir = filepath.Clean(dir)
	if err = os.Mkdir(dir, 0700); err != nil && !errors.Is(err, os.ErrExist) {
		return nil, storageError(err)
	}
	info, err := os.Lstat(dir)
	if err != nil {
		return nil, storageError(err)
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 || info.Mode().Perm()&0077 != 0 || info.Sys().(*syscall.Stat_t).Uid != uint32(os.Geteuid()) {
		return nil, ErrCorrupt
	}
	root, err := os.OpenRoot(dir)
	if err != nil {
		return nil, storageError(err)
	}
	a := &Adapter{root: root}
	defer func() {
		if err != nil {
			a.Close()
		}
	}()
	a.lock, err = root.OpenFile("lock", os.O_CREATE|os.O_RDWR|syscall.O_NOFOLLOW, 0600)
	if err != nil {
		return nil, storageError(err)
	}
	if err = regular(root, "lock"); err != nil {
		return nil, ErrCorrupt
	}
	if err = syscall.Flock(int(a.lock.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		if errors.Is(err, syscall.EWOULDBLOCK) {
			return nil, ErrLocked
		}
		return nil, storageError(err)
	}
	for _, name := range []string{"objects", "staging"} {
		if err = root.Mkdir(name, 0700); err != nil && !errors.Is(err, os.ErrExist) {
			return nil, storageError(err)
		}
		s, e := root.Lstat(name)
		if e != nil || !s.IsDir() || s.Mode()&os.ModeSymlink != 0 || s.Mode().Perm()&0077 != 0 || s.Sys().(*syscall.Stat_t).Uid != uint32(os.Geteuid()) {
			return nil, ErrCorrupt
		}
	}
	if err = a.entries(".", func(name string) error {
		switch name {
		case "objects", "staging", "lock", "manifest", "manifest.next":
			return nil
		}
		return ErrCorrupt
	}); err != nil {
		return nil, err
	}
	if _, err = root.Lstat("manifest"); errors.Is(err, os.ErrNotExist) {
		empty := true
		if err = a.entries("objects", func(string) error { empty = false; return nil }); err != nil {
			return nil, err
		}
		if !empty {
			return nil, ErrCorrupt
		}
		if _, e := root.Lstat("manifest.next"); e == nil {
			if e = regular(root, "manifest.next"); e != nil {
				return nil, ErrCorrupt
			}
			next, e := root.OpenFile("manifest.next", os.O_RDONLY|syscall.O_NOFOLLOW, 0)
			if e != nil {
				return nil, storageError(e)
			}
			b, e := io.ReadAll(io.LimitReader(next, int64(len(header)+1)))
			next.Close()
			if e != nil || !bytes.HasPrefix([]byte(header), b) {
				return nil, ErrCorrupt
			}
			if e = root.Remove("manifest.next"); e != nil {
				return nil, storageError(e)
			}
		} else if !errors.Is(e, os.ErrNotExist) {
			return nil, storageError(e)
		}
		if e := a.writeFile("manifest.next", []byte(header), "initialize"); e != nil {
			return nil, e
		}
		if e := root.Rename("manifest.next", "manifest"); e != nil {
			return nil, storageError(e)
		}
	} else if err != nil {
		return nil, storageError(err)
	}
	if err = a.recover(context.Background()); err != nil {
		return nil, err
	}
	if err = syncDir(root, "."); err != nil {
		return nil, storageError(err)
	}
	{
		p, e := os.Open(filepath.Dir(dir))
		if e != nil {
			return nil, storageError(e)
		}
		e = p.Sync()
		p.Close()
		if e != nil {
			return nil, storageError(e)
		}
	}
	return a, nil
}

func validate(ctx context.Context, r Record, files bool) (contracts.Input, error) {
	if strings.TrimSpace(r.OperationID) == "" || utf8.RuneCountInString(r.OperationID) > 4096 || !utf8.ValidString(r.OperationID) {
		return contracts.Input{}, ErrInvalid
	}
	v, err := contracts.DecodeInput(ctx, r.InputJSON)
	if err != nil {
		return v, err
	}
	result, err := contracts.DecodeResult(ctx, r.ResultJSON)
	if err != nil {
		return v, err
	}
	if result.InputDigest != digest(r.InputJSON) {
		return v, ErrInvalid
	}
	if o := result.Result; o != nil && (o.SubjectDigest != v.SubjectDigest || o.EvidenceDigest != v.EvidenceDigest || o.PolicyDigest != v.PolicyDigest) {
		return v, ErrInvalid
	}
	if o := result.Result; o != nil && result.SchemaVersion == "2" {
		manifest := make(map[string]string, len(v.Files))
		for _, f := range v.Files {
			manifest[f.Path] = f.Digest
		}
		for _, ref := range o.EvidenceReferences {
			if manifest[ref.Path] != ref.Digest || ref.Digest != v.EvidenceDigest {
				return v, ErrInvalid
			}
		}
	}
	if files {
		if len(r.Files) != len(v.Files) {
			return v, ErrInvalid
		}
		for _, f := range v.Files {
			if err := ctx.Err(); err != nil {
				return v, err
			}
			b, ok := r.Files[f.Path]
			if !ok || int64(len(b)) != *f.SizeBytes || digest(b) != f.Digest {
				return v, ErrInvalid
			}
		}
	}
	return v, nil
}

func lines(data []byte, atEOF bool) (int, []byte, error) {
	if n := bytes.IndexByte(data, '\n'); n >= 0 {
		return n + 1, data[:n], nil
	}
	if atEOF && len(data) != 0 {
		return 0, nil, ErrCorrupt
	}
	return 0, nil, nil
}

type storeUsage struct {
	operations     int
	artifactRefs   int
	historyBytes   int64
	objectBytes    int64
	manifestBytes  int64
	rewrittenBytes int64
	objects        map[string]int64
}

func checkQuota(current storeUsage, input contracts.Input, nextLineBytes int64) error {
	if current.operations >= maxOperations || current.artifactRefs+len(input.Files) > maxArtifactRefs || current.rewrittenBytes > maxManifestBytes-nextLineBytes {
		return ErrQuota
	}
	var addedHistory, addedObjects int64
	pending := make(map[string]int64, len(input.Files))
	for _, file := range input.Files {
		size := *file.SizeBytes
		if addedHistory > maxHistoryBytes-size {
			return ErrQuota
		}
		addedHistory += size
		if existing, ok := current.objects[file.Digest]; ok {
			if existing != size {
				return ErrInvalid
			}
			continue
		}
		if existing, ok := pending[file.Digest]; ok {
			if existing != size {
				return ErrInvalid
			}
			continue
		}
		pending[file.Digest] = size
		if addedObjects > maxObjectBytes-size {
			return ErrQuota
		}
		addedObjects += size
	}
	if current.historyBytes > maxHistoryBytes-addedHistory || current.objectBytes > maxObjectBytes-addedObjects {
		return ErrQuota
	}
	return nil
}

// scan bounds retained history and returns only capped counters and digest sizes.
func (a *Adapter) scan(ctx context.Context, visit func(entry, contracts.Input) error) (storeUsage, error) {
	usage := storeUsage{objects: make(map[string]int64)}
	if err := regular(a.root, "manifest"); err != nil {
		return usage, ErrCorrupt
	}
	info, err := a.root.Lstat("manifest")
	if err != nil {
		return usage, ErrCorrupt
	}
	usage.manifestBytes = info.Size()
	if usage.manifestBytes > maxManifestBytes {
		return usage, ErrQuota
	}
	usage.rewrittenBytes = int64(len(header))
	f, err := a.root.OpenFile("manifest", os.O_RDONLY|syscall.O_NOFOLLOW, 0)
	if err != nil {
		return usage, ErrCorrupt
	}
	defer f.Close()
	s := bufio.NewScanner(f)
	s.Buffer(make([]byte, 64*1024), maxLine)
	s.Split(lines)
	if !s.Scan() || string(s.Bytes())+"\n" != header {
		return usage, ErrCorrupt
	}
	ids := make(map[string]struct{})
	for s.Scan() {
		if err := ctx.Err(); err != nil {
			return usage, err
		}
		var e entry
		if err := json.Unmarshal(s.Bytes(), &e, json.RejectUnknownMembers(true)); err != nil || e.Checksum != checksum(e) {
			return usage, ErrCorrupt
		}
		v, err := validate(ctx, Record{OperationID: e.OperationID, InputJSON: e.Input, ResultJSON: e.Result}, false)
		if err != nil {
			if ctx.Err() != nil {
				return usage, ctx.Err()
			}
			return usage, ErrCorrupt
		}
		if _, exists := ids[e.OperationID]; exists {
			return usage, ErrCorrupt
		}
		ids[e.OperationID] = struct{}{}
		usage.operations++
		if usage.operations > maxOperations {
			return usage, ErrQuota
		}
		line, err := json.Marshal(e)
		if err != nil {
			return usage, ErrCorrupt
		}
		usage.rewrittenBytes += int64(len(line) + 1)
		if usage.rewrittenBytes > maxManifestBytes {
			return usage, ErrQuota
		}
		for _, file := range v.Files {
			if usage.artifactRefs == maxArtifactRefs {
				return usage, ErrQuota
			}
			usage.artifactRefs++
			size := *file.SizeBytes
			if usage.historyBytes > maxHistoryBytes-size {
				return usage, ErrQuota
			}
			usage.historyBytes += size
			if prior, exists := usage.objects[file.Digest]; exists {
				if prior != size {
					return usage, ErrCorrupt
				}
				continue
			}
			if len(usage.objects) == maxArtifactRefs || usage.objectBytes > maxObjectBytes-size {
				return usage, ErrQuota
			}
			usage.objects[file.Digest] = size
			usage.objectBytes += size
		}
		if err := visit(e, v); err != nil {
			return usage, err
		}
	}
	if s.Err() != nil {
		return usage, ErrCorrupt
	}
	return usage, ctx.Err()
}
func (a *Adapter) find(ctx context.Context, id string) (entry, error) {
	var found entry
	count := 0
	_, err := a.scan(ctx, func(e entry, _ contracts.Input) error {
		if e.OperationID == id {
			count++
			found = e
		}
		return nil
	})
	if err != nil {
		return entry{}, err
	}
	if count > 1 {
		return entry{}, ErrCorrupt
	}
	if count == 0 {
		return entry{}, ErrNotFound
	}
	return found, nil
}
func (a *Adapter) object(ctx context.Context, f contracts.File, collect bool) ([]byte, error) {
	name := "objects/" + f.Digest[7:]
	if err := regular(a.root, name); err != nil {
		return nil, ErrCorrupt
	}
	file, err := a.root.OpenFile(name, os.O_RDONLY|syscall.O_NOFOLLOW, 0)
	if err != nil {
		return nil, ErrCorrupt
	}
	defer file.Close()
	h := sha256.New()
	var out bytes.Buffer
	var w io.Writer = h
	if collect {
		w = io.MultiWriter(h, &out)
	}
	buf := make([]byte, 64*1024)
	var total int64
	for {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		n, e := file.Read(buf)
		total += int64(n)
		if total > *f.SizeBytes {
			return nil, ErrCorrupt
		}
		if n > 0 {
			if _, err := w.Write(buf[:n]); err != nil {
				return nil, ErrCorrupt
			}
		}
		if e == io.EOF {
			break
		}
		if e != nil {
			return nil, ErrCorrupt
		}
	}
	if total != *f.SizeBytes || "sha256:"+hex.EncodeToString(h.Sum(nil)) != f.Digest {
		return nil, ErrCorrupt
	}
	return out.Bytes(), nil
}
func (a *Adapter) entries(dir string, visit func(string) error) error {
	f, err := a.root.Open(dir)
	if err != nil {
		return storageError(err)
	}
	defer f.Close()
	for {
		names, err := f.Readdirnames(64)
		for _, name := range names {
			if e := visit(name); e != nil {
				return e
			}
		}
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return storageError(err)
		}
	}
}
func (a *Adapter) recover(ctx context.Context) error {
	// Validate every reference before deleting any orphan.
	usage, err := a.scan(ctx, func(_ entry, v contracts.Input) error {
		for _, f := range v.Files {
			if _, err := a.object(ctx, f, false); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return err
	}
	if err := a.entries("staging", func(name string) error {
		if name != "blob" {
			return ErrCorrupt
		}
		if err := regular(a.root, "staging/blob"); err != nil {
			return ErrCorrupt
		}
		return storageErrorOrNil(a.root.Remove("staging/blob"))
	}); err != nil {
		return err
	}
	if _, err := a.root.Lstat("manifest.next"); err == nil {
		if err := regular(a.root, "manifest.next"); err != nil {
			return ErrCorrupt
		}
		if err := a.root.Remove("manifest.next"); err != nil {
			return storageError(err)
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return storageError(err)
	}
	if err := a.entries("objects", func(name string) error {
		b, err := hex.DecodeString(name)
		if err != nil || len(b) != 32 || hex.EncodeToString(b) != name {
			return ErrCorrupt
		}
		if err := regular(a.root, "objects/"+name); err != nil {
			return ErrCorrupt
		}
		if _, used := usage.objects["sha256:"+name]; !used {
			info, err := a.root.Lstat("objects/" + name)
			if err != nil || info.Size() > contracts.MaxBundleBytes {
				return ErrCorrupt
			}
			size := info.Size()
			if _, err := a.object(ctx, contracts.File{Digest: "sha256:" + name, SizeBytes: &size}, false); err != nil {
				return err
			}
			return storageErrorOrNil(a.root.Remove("objects/" + name))
		}
		return nil
	}); err != nil {
		return err
	}
	for _, dir := range []string{"objects", "staging", "."} {
		if err := syncDir(a.root, dir); err != nil {
			return storageError(err)
		}
	}
	return nil
}
func (a *Adapter) enter() error {
	if a.closed {
		return ErrClosed
	}
	if a.uncertain {
		return ErrRecovery
	}
	return nil
}

// Commit publishes one snapshot, including durable operation/idempotency state.
// Any I/O failure requires reopening. In particular, failure after rename may
// mean committed-without-acknowledgement; replay resolves that uncertainty.
func (a *Adapter) Commit(ctx context.Context, r Record) (err error) {
	if !a.mu.TryLock() {
		return ErrLocked
	}
	defer a.mu.Unlock()
	if err = a.enter(); err != nil {
		return err
	}
	v, err := validate(ctx, r, true)
	if err != nil {
		return err
	}
	e := entry{OperationID: r.OperationID, Input: r.InputJSON, Result: r.ResultJSON}
	e.Checksum = checksum(e)
	var old entry
	found := false
	usage, err := a.scan(ctx, func(candidate entry, _ contracts.Input) error {
		if candidate.OperationID == r.OperationID {
			old, found = candidate, true
		}
		return nil
	})
	if err != nil {
		return err
	}
	if found {
		if old.Checksum != e.Checksum {
			return ErrConflict
		}
		for _, f := range v.Files {
			if _, err := a.object(ctx, f, false); err != nil {
				return err
			}
		}
		return nil
	}
	line, err := json.Marshal(e)
	if err != nil {
		return ErrInvalid
	}
	if err = checkQuota(usage, v, int64(len(line)+1)); err != nil {
		return err
	}
	// Once writes begin, a failed commit must be reconciled before reuse.
	defer func() {
		if err != nil {
			a.uncertain = true
		}
	}()
	for _, f := range v.Files {
		if err = ctx.Err(); err != nil {
			return err
		}
		name := "objects/" + f.Digest[7:]
		if _, err = a.root.Lstat(name); err == nil {
			if _, err = a.object(ctx, f, false); err != nil {
				return err
			}
			continue
		} else if !errors.Is(err, os.ErrNotExist) {
			return storageError(err)
		}
		if err = a.writeFile("staging/blob", r.Files[f.Path], "object"); err != nil {
			return err
		}
		if err = a.point("object.publish"); err != nil {
			return storageError(err)
		}
		if err = a.root.Rename("staging/blob", name); err != nil {
			return storageError(err)
		}
		if _, err = a.object(ctx, f, false); err != nil {
			return err
		}
	}
	if err = a.point("objects.sync"); err != nil {
		return storageError(err)
	}
	if err = syncDir(a.root, "objects"); err != nil {
		return storageError(err)
	}
	if err = a.point("staging.sync"); err != nil {
		return storageError(err)
	}
	if err = syncDir(a.root, "staging"); err != nil {
		return storageError(err)
	}
	if err = a.point("objects.ready"); err != nil {
		return storageError(err)
	}
	if err = a.publish(ctx, e); err != nil {
		return err
	}
	return nil
}
func (a *Adapter) writeFile(name string, b []byte, phase string) error {
	f, err := a.root.OpenFile(name, os.O_CREATE|os.O_EXCL|os.O_WRONLY|syscall.O_NOFOLLOW, 0600)
	if err != nil {
		return storageError(err)
	}
	defer f.Close()
	if err = a.point(phase + ".write"); err != nil {
		return storageError(err)
	}
	if _, err = io.Copy(f, bytes.NewReader(b)); err != nil {
		return storageError(err)
	}
	if err = a.point(phase + ".sync"); err != nil {
		return storageError(err)
	}
	if err = f.Sync(); err != nil {
		return storageError(err)
	}
	return storageErrorOrNil(f.Close())
}
func storageErrorOrNil(err error) error {
	if err != nil {
		return storageError(err)
	}
	return nil
}
func (a *Adapter) publish(ctx context.Context, e entry) error {
	f, err := a.root.OpenFile("manifest.next", os.O_CREATE|os.O_EXCL|os.O_WRONLY|syscall.O_NOFOLLOW, 0600)
	if err != nil {
		return storageError(err)
	}
	defer f.Close()
	if err = a.point("manifest.write"); err != nil {
		return storageError(err)
	}
	if _, err = io.WriteString(f, header); err != nil {
		return storageError(err)
	}
	if _, err = a.scan(ctx, func(old entry, _ contracts.Input) error {
		b, err := json.Marshal(old)
		if err != nil {
			return err
		}
		b = append(b, '\n')
		_, err = f.Write(b)
		return storageErrorOrNil(err)
	}); err != nil {
		return err
	}
	b, err := json.Marshal(e)
	if err != nil {
		return ErrInvalid
	}
	b = append(b, '\n')
	if _, err = f.Write(b); err != nil {
		return storageError(err)
	}
	if err = a.point("manifest.sync"); err != nil {
		return storageError(err)
	}
	if err = f.Sync(); err != nil {
		return storageError(err)
	}
	if err = f.Close(); err != nil {
		return storageError(err)
	}
	if err = ctx.Err(); err != nil {
		return err
	}
	if err = a.point("manifest.publish"); err != nil {
		return storageError(err)
	}
	if err = a.root.Rename("manifest.next", "manifest"); err != nil {
		return storageError(err)
	}
	// Cancellation after publication cannot turn a durable success into a rollback.
	if err = a.point("directory.sync"); err != nil {
		return storageError(err)
	}
	if err = syncDir(a.root, "."); err != nil {
		return storageError(err)
	}
	if err = a.point("committed"); err != nil {
		return storageError(err)
	}
	return nil
}
func (a *Adapter) Load(ctx context.Context, id string) (Record, error) {
	if !a.mu.TryLock() {
		return Record{}, ErrLocked
	}
	defer a.mu.Unlock()
	if err := a.enter(); err != nil {
		return Record{}, err
	}
	e, err := a.find(ctx, id)
	if err != nil {
		return Record{}, err
	}
	v, err := validate(ctx, Record{OperationID: e.OperationID, InputJSON: e.Input, ResultJSON: e.Result}, false)
	if err != nil {
		return Record{}, ErrCorrupt
	}
	r := Record{OperationID: e.OperationID, InputJSON: e.Input, ResultJSON: e.Result, Files: make(map[string][]byte, len(v.Files))}
	for _, f := range v.Files {
		b, err := a.object(ctx, f, true)
		if err != nil {
			return Record{}, err
		}
		r.Files[f.Path] = b
	}
	return r, nil
}
func (a *Adapter) Close() error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.closed {
		return nil
	}
	a.closed = true
	var err error
	if a.lock != nil {
		err = a.lock.Close()
	}
	return errors.Join(err, a.root.Close())
}
