package hfir

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
)

var (
	ErrNotFound    = errors.New("hfir storage: object not found")
	ErrCorrupted   = errors.New("hfir storage: object corrupted (hash mismatch)")
	ErrInvalidHash = errors.New("hfir storage: invalid hash format")
)

func validateHash(hash string) error {
	if len(hash) != 64 {
		return ErrInvalidHash
	}
	for i := 0; i < len(hash); i++ {
		c := hash[i]
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
			return ErrInvalidHash
		}
	}
	return nil
}

func hashBytes(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func canonicalNodeForCAS(node *Node) *Node {
	if node == nil {
		return nil
	}
	clone := *node
	clone.Provenance = Provenance{Filename: node.Provenance.Filename}
	return &clone
}

func hashNode(node *Node) (string, error) {
	if node == nil {
		return "", errors.New("nil node")
	}
	canonical := canonicalNodeForCAS(node)
	data, err := json.Marshal(canonical)
	if err != nil {
		return "", err
	}
	return hashBytes(data), nil
}

// Module encapsulates a set of nodes belonging to a module namespace.
type Module struct {
	Name        string            `json:"name"`
	NodeIDs     []NodeID          `json:"node_ids"`
	NodeHashes  map[NodeID]string `json:"node_hashes"`
	Imports     []string          `json:"imports,omitempty"`
	Exports     []string          `json:"exports,omitempty"`
	ContentHash string            `json:"-"`
}

func (m *Module) Canonicalize() {
	if m == nil {
		return
	}
	m.NodeIDs = dedupeAndSort(m.NodeIDs)
	m.Imports = dedupeAndSort(m.Imports)
	m.Exports = dedupeAndSort(m.Exports)
}

func (m *Module) ComputeHash() string {
	if m == nil {
		return ""
	}
	m.Canonicalize()
	data, _ := json.Marshal(m)
	return hashBytes(data)
}

// VerificationEvidence records the proof that a graph or module passed verification.
type VerificationEvidence struct {
	GraphHash       string       `json:"graph_hash"`
	Target          string       `json:"target"`
	Verified        bool         `json:"verified"`
	Diagnostics     []Diagnostic `json:"diagnostics,omitempty"`
	InferredEffects []Effect     `json:"effects,omitempty"`
	EvidenceHash    string       `json:"-"`
}

func (v *VerificationEvidence) Canonicalize() {
	if v == nil {
		return
	}
	if len(v.InferredEffects) > 0 {
		sort.Slice(v.InferredEffects, func(i, j int) bool {
			if v.InferredEffects[i].Type != v.InferredEffects[j].Type {
				return v.InferredEffects[i].Type < v.InferredEffects[j].Type
			}
			return v.InferredEffects[i].Capability < v.InferredEffects[j].Capability
		})
	}
}

func (v *VerificationEvidence) ComputeHash() string {
	if v == nil {
		return ""
	}
	v.Canonicalize()
	data, _ := json.Marshal(v)
	return hashBytes(data)
}

// LoweredArtifact stores an emitted compiler artifact by content hash.
type LoweredArtifact struct {
	SourceGraphHash string `json:"source_graph_hash,omitempty"`
	Target          string `json:"target,omitempty"`
	ArtifactKind    string `json:"artifact_kind,omitempty"`
	Payload         []byte `json:"payload"`
	ArtifactHash    string `json:"-"`
}

func (a *LoweredArtifact) ComputeHash() string {
	if a == nil || len(a.Payload) == 0 {
		return ""
	}
	return hashBytes(a.Payload)
}

// Store is the interface for content-addressed storage of raw blobs.
type Store interface {
	PutBlob(data []byte) (string, error)
	GetBlob(hash string) ([]byte, error)
	HasBlob(hash string) bool
	DeleteBlob(hash string) error
	Clear() error
}

// MemoryStore provides thread-safe in-memory content-addressed storage.
type MemoryStore struct {
	mu    sync.RWMutex
	blobs map[string][]byte
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		blobs: make(map[string][]byte),
	}
}

func (m *MemoryStore) PutBlob(data []byte) (string, error) {
	hash := hashBytes(data)
	m.mu.Lock()
	defer m.mu.Unlock()
	buf := make([]byte, len(data))
	copy(buf, data)
	m.blobs[hash] = buf
	return hash, nil
}

func (m *MemoryStore) GetBlob(hash string) ([]byte, error) {
	if err := validateHash(hash); err != nil {
		return nil, err
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	data, ok := m.blobs[hash]
	if !ok {
		return nil, ErrNotFound
	}
	if hashBytes(data) != hash {
		return nil, ErrCorrupted
	}
	res := make([]byte, len(data))
	copy(res, data)
	return res, nil
}

func (m *MemoryStore) HasBlob(hash string) bool {
	if err := validateHash(hash); err != nil {
		return false
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	_, ok := m.blobs[hash]
	return ok
}

func (m *MemoryStore) DeleteBlob(hash string) error {
	if err := validateHash(hash); err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.blobs, hash)
	return nil
}

func (m *MemoryStore) Clear() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.blobs = make(map[string][]byte)
	return nil
}

// DiskStore provides durable, filesystem-backed content-addressed storage.
type DiskStore struct {
	mu      sync.RWMutex
	baseDir string
}

func NewDiskStore(baseDir string) (*DiskStore, error) {
	for _, sub := range []string{"objects", "tmp"} {
		if err := os.MkdirAll(filepath.Join(baseDir, sub), 0755); err != nil {
			return nil, fmt.Errorf("create disk store dir %s: %w", sub, err)
		}
	}
	return &DiskStore{baseDir: baseDir}, nil
}

func (d *DiskStore) objectPath(hash string) (string, error) {
	if err := validateHash(hash); err != nil {
		return "", err
	}
	return filepath.Join(d.baseDir, "objects", hash[:2], hash[2:]), nil
}

func (d *DiskStore) PutBlob(data []byte) (string, error) {
	hash := hashBytes(data)
	targetPath, err := d.objectPath(hash)
	if err != nil {
		return "", err
	}

	d.mu.Lock()
	defer d.mu.Unlock()

	// If existing file is valid, reuse it; if corrupted, overwrite
	if existing, err := os.ReadFile(targetPath); err == nil && hashBytes(existing) == hash {
		return hash, nil
	}

	if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
		return "", fmt.Errorf("mkdir object prefix: %w", err)
	}

	tmpFile, err := os.CreateTemp(filepath.Join(d.baseDir, "tmp"), "put-*")
	if err != nil {
		return "", fmt.Errorf("temp file: %w", err)
	}
	tmpName := tmpFile.Name()
	defer os.Remove(tmpName)

	if _, err := tmpFile.Write(data); err != nil {
		_ = tmpFile.Close()
		return "", fmt.Errorf("write blob: %w", err)
	}
	if err := tmpFile.Close(); err != nil {
		return "", fmt.Errorf("close temp: %w", err)
	}

	if err := os.Rename(tmpName, targetPath); err != nil {
		return "", fmt.Errorf("commit object: %w", err)
	}
	return hash, nil
}

func (d *DiskStore) GetBlob(hash string) ([]byte, error) {
	targetPath, err := d.objectPath(hash)
	if err != nil {
		return nil, err
	}
	d.mu.RLock()
	defer d.mu.RUnlock()

	data, err := os.ReadFile(targetPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("read blob: %w", err)
	}
	if hashBytes(data) != hash {
		return nil, ErrCorrupted
	}
	return data, nil
}

func (d *DiskStore) HasBlob(hash string) bool {
	targetPath, err := d.objectPath(hash)
	if err != nil {
		return false
	}
	d.mu.RLock()
	defer d.mu.RUnlock()
	_, statErr := os.Stat(targetPath)
	return statErr == nil
}

func (d *DiskStore) DeleteBlob(hash string) error {
	targetPath, err := d.objectPath(hash)
	if err != nil {
		return err
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	if remErr := os.Remove(targetPath); remErr != nil && !os.IsNotExist(remErr) {
		return remErr
	}
	return nil
}

func (d *DiskStore) Clear() error {
	d.mu.Lock()
	defer d.mu.Unlock()
	_ = os.RemoveAll(filepath.Join(d.baseDir, "objects"))
	_ = os.RemoveAll(filepath.Join(d.baseDir, "tmp"))
	if err := os.MkdirAll(filepath.Join(d.baseDir, "objects"), 0755); err != nil {
		return err
	}
	return os.MkdirAll(filepath.Join(d.baseDir, "tmp"), 0755)
}

// Generic typed helpers for storing/loading HFIR entities in CAS.

func storeJSONEntity(s Store, val any) (string, error) {
	data, err := json.Marshal(val)
	if err != nil {
		return "", fmt.Errorf("marshal entity: %w", err)
	}
	return s.PutBlob(data)
}

func loadJSONEntity(s Store, hash string, out any) error {
	data, err := s.GetBlob(hash)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(data, out); err != nil {
		return fmt.Errorf("unmarshal: %w", err)
	}
	return nil
}

func PutNode(s Store, node *Node) (string, error) {
	if node == nil {
		return "", errors.New("cannot store nil node")
	}
	canonical := canonicalNodeForCAS(node)
	return storeJSONEntity(s, canonical)
}

func GetNode(s Store, hash string) (*Node, error) {
	var node Node
	if err := loadJSONEntity(s, hash, &node); err != nil {
		return nil, err
	}
	return &node, nil
}

func PutModule(s Store, mod *Module) (string, error) {
	if mod == nil {
		return "", errors.New("cannot store nil module")
	}
	mod.Canonicalize()
	hash, err := storeJSONEntity(s, mod)
	if err != nil {
		return "", err
	}
	mod.ContentHash = hash
	return hash, nil
}

func GetModule(s Store, hash string) (*Module, error) {
	var mod Module
	if err := loadJSONEntity(s, hash, &mod); err != nil {
		return nil, err
	}
	mod.ContentHash = hash
	return &mod, nil
}

func PutEvidence(s Store, evidence *VerificationEvidence) (string, error) {
	if evidence == nil {
		return "", errors.New("cannot store nil evidence")
	}
	evidence.Canonicalize()
	hash, err := storeJSONEntity(s, evidence)
	if err != nil {
		return "", err
	}
	evidence.EvidenceHash = hash
	return hash, nil
}

func GetEvidence(s Store, hash string) (*VerificationEvidence, error) {
	var evidence VerificationEvidence
	if err := loadJSONEntity(s, hash, &evidence); err != nil {
		return nil, err
	}
	evidence.EvidenceHash = hash
	return &evidence, nil
}

func PutArtifact(s Store, artifact *LoweredArtifact) (string, error) {
	if artifact == nil {
		return "", errors.New("cannot store nil artifact")
	}
	hash, err := s.PutBlob(artifact.Payload)
	if err != nil {
		return "", err
	}
	artifact.ArtifactHash = hash
	return hash, nil
}

func GetArtifact(s Store, hash string) (*LoweredArtifact, error) {
	payload, err := s.GetBlob(hash)
	if err != nil {
		return nil, err
	}
	return &LoweredArtifact{
		Payload:      payload,
		ArtifactHash: hash,
	}, nil
}

func PutManifest(s Store, manifest *ArtifactManifest) (string, error) {
	if manifest == nil {
		return "", errors.New("cannot store nil manifest")
	}
	hash, err := storeJSONEntity(s, manifest)
	if err != nil {
		return "", err
	}
	manifest.ManifestHash = hash
	return hash, nil
}

func GetManifest(s Store, hash string) (*ArtifactManifest, error) {
	var manifest ArtifactManifest
	if err := loadJSONEntity(s, hash, &manifest); err != nil {
		return nil, err
	}
	manifest.ManifestHash = hash
	return &manifest, nil
}
