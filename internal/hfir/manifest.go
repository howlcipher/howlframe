package hfir

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
)

const ArtifactManifestSchemaVersion = "hfir-manifest/v1"

// ArtifactManifest is a reproducible record of build inputs, hashes, and emitted artifacts.
type ArtifactManifest struct {
	SchemaVersion            string              `json:"schema_version"`
	GraphHash                string              `json:"graph_hash"`
	Target                   string              `json:"target"`
	EntryNode                NodeID              `json:"entry_node,omitempty"`
	NodeHashes               map[NodeID]string   `json:"node_hashes"`
	ModuleHashes             map[string]string   `json:"module_hashes,omitempty"`
	ForwardDependencies      map[NodeID][]NodeID `json:"forward_dependencies"`
	ReverseDependencies      map[NodeID][]NodeID `json:"reverse_dependencies"`
	VerificationEvidenceHash string              `json:"verification_evidence_hash"`
	ArtifactHash             string              `json:"artifact_hash"`
	ArtifactKind             string              `json:"artifact_kind"`
	ManifestHash             string              `json:"-"`
}

func buildGraphModules(graph *Graph) ([]*Module, map[string]string) {
	if graph == nil {
		return nil, nil
	}
	moduleNodes := make(map[string][]NodeID)
	nodeHashes := make(map[NodeID]string)
	for _, node := range graph.Nodes {
		if node != nil {
			h, _ := hashNode(node)
			nodeHashes[node.ID] = h
			mod := node.Module
			if mod == "" {
				mod = "main"
			}
			moduleNodes[mod] = append(moduleNodes[mod], node.ID)
		}
	}
	modules := make([]*Module, 0, len(moduleNodes))
	moduleHashes := make(map[string]string, len(moduleNodes))
	for modName, nids := range moduleNodes {
		sortedIDs := dedupeAndSort(nids)
		mod := &Module{
			Name:       modName,
			NodeIDs:    sortedIDs,
			NodeHashes: make(map[NodeID]string, len(sortedIDs)),
		}
		for _, nid := range sortedIDs {
			mod.NodeHashes[nid] = nodeHashes[nid]
		}
		mod.Canonicalize()
		h := mod.ComputeHash()
		moduleHashes[modName] = h
		modules = append(modules, mod)
	}
	return modules, moduleHashes
}

// NewArtifactManifest constructs a canonical, deterministic manifest.
func NewArtifactManifest(
	graph *Graph,
	target string,
	index *DependencyIndex,
	evidenceHash string,
	artifactHash string,
	artifactKind string,
) *ArtifactManifest {
	nodeHashes := make(map[NodeID]string)
	if graph != nil {
		for _, node := range graph.Nodes {
			if node != nil {
				h, _ := hashNode(node)
				nodeHashes[node.ID] = h
			}
		}
	}

	_, moduleHashes := buildGraphModules(graph)

	var fwd, rev map[NodeID][]NodeID
	if index != nil {
		fwd = index.Forward
		rev = index.Reverse
	} else {
		fwd = make(map[NodeID][]NodeID)
		rev = make(map[NodeID][]NodeID)
	}

	m := &ArtifactManifest{
		SchemaVersion:            ArtifactManifestSchemaVersion,
		GraphHash:                GraphHash(graph),
		Target:                   target,
		NodeHashes:               nodeHashes,
		ModuleHashes:             moduleHashes,
		ForwardDependencies:      fwd,
		ReverseDependencies:      rev,
		VerificationEvidenceHash: evidenceHash,
		ArtifactHash:             artifactHash,
		ArtifactKind:             artifactKind,
	}

	if graph != nil {
		m.EntryNode = graph.EntryNode
	}
	m.ManifestHash = m.ComputeHash()
	return m
}

// ComputeHash returns the deterministic SHA-256 fingerprint of the manifest.
func (m *ArtifactManifest) ComputeHash() string {
	if m == nil {
		return ""
	}
	shallow := *m
	shallow.ManifestHash = ""
	data, _ := json.Marshal(shallow)
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}

// Serialize returns formatted canonical JSON for the manifest.
func (m *ArtifactManifest) Serialize() ([]byte, error) {
	return json.MarshalIndent(m, "", "  ")
}

// ValidateAgainstStore ensures all referenced entities exist in the CAS without corruption.
func (m *ArtifactManifest) ValidateAgainstStore(s Store) error {
	if m == nil {
		return errors.New("nil manifest")
	}
	if expected := m.ComputeHash(); m.ManifestHash != "" && m.ManifestHash != expected {
		return fmt.Errorf("%w: manifest hash mismatch (have %s, computed %s)", ErrCorrupted, m.ManifestHash, expected)
	}

	// 1. Verify verification evidence exists and is valid
	evidence, err := GetEvidence(s, m.VerificationEvidenceHash)
	if err != nil {
		return fmt.Errorf("verify evidence in store: %w", err)
	}
	if !evidence.Verified {
		return errors.New("manifest references unverified evidence")
	}
	if evidence.GraphHash != m.GraphHash {
		return fmt.Errorf("%w: evidence graph hash mismatch", ErrCorrupted)
	}
	if evidence.Target != m.Target {
		return fmt.Errorf("%w: evidence target mismatch", ErrCorrupted)
	}

	// 2. Verify artifact payload blob exists and matches hash
	raw, err := s.GetBlob(m.ArtifactHash)
	if err != nil {
		return fmt.Errorf("verify artifact in store: %w", err)
	}
	if hashBytes(raw) != m.ArtifactHash {
		return fmt.Errorf("%w: artifact payload mismatch", ErrCorrupted)
	}

	// 3. Verify modules
	for modName, expectedHash := range m.ModuleHashes {
		mod, err := GetModule(s, expectedHash)
		if err != nil {
			return fmt.Errorf("module %s: %w", modName, err)
		}
		if mod.ComputeHash() != expectedHash {
			return fmt.Errorf("%w: module %s hash mismatch", ErrCorrupted, modName)
		}
	}

	// 4. Verify nodes
	nodeIDs := make([]NodeID, 0, len(m.NodeHashes))
	for id := range m.NodeHashes {
		nodeIDs = append(nodeIDs, id)
	}
	sort.Slice(nodeIDs, func(i, j int) bool { return nodeIDs[i] < nodeIDs[j] })

	for _, id := range nodeIDs {
		expectedHash := m.NodeHashes[id]
		node, err := GetNode(s, expectedHash)
		if err != nil {
			return fmt.Errorf("node %s: %w", id, err)
		}
		actual, _ := hashNode(node)
		if actual != expectedHash {
			return fmt.Errorf("%w: node %s hash mismatch", ErrCorrupted, id)
		}
	}

	return nil
}
