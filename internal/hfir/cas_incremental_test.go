package hfir

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"testing"

	"github.com/howlcipher/howlframe/internal/ast"
)

func newConstNode(id NodeID, val string) *Node {
	return &Node{
		ID:          id,
		Kind:        "const",
		Value:       val,
		LiteralKind: "INT",
		Type:        ast.Layout(ast.Int),
	}
}

func newBinaryNode(id NodeID, op string, left, right NodeID) *Node {
	return &Node{
		ID:    id,
		Kind:  "binary",
		Value: op,
		Type:  ast.Layout(ast.Int),
		DataInputs: []DataEdge{
			{Name: "left", SourceNode: left},
			{Name: "right", SourceNode: right},
		},
	}
}

func newTestDiskStore(t *testing.T) *DiskStore {
	t.Helper()
	ds, err := NewDiskStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return ds
}

func buildBranchingGraph(valB string) *Graph {
	g := NewGraph()
	// Branch 1: a1 -> a2 -> a3
	a1 := g.AddNode(newConstNode("a1", "10"))
	a2 := g.AddNode(newBinaryNode("a2", "+", a1, a1))
	a3 := g.AddNode(newBinaryNode("a3", "*", a2, a2))

	// Branch 2: b1 -> b2 -> b3
	b1 := g.AddNode(newConstNode("b1", valB))
	b2 := g.AddNode(newBinaryNode("b2", "-", b1, b1))
	b3 := g.AddNode(newBinaryNode("b3", "+", b2, b2))

	// Root combining both branches
	root := g.AddNode(newBinaryNode("root", "+", a3, b3))
	g.EntryNode = root
	return g
}

func TestHashDeterminism(t *testing.T) {
	node1 := newConstNode("n1", "42")
	h1, err1 := hashNode(node1)
	h2, err2 := hashNode(node1)
	if err1 != nil || err2 != nil || h1 == "" || h1 != h2 {
		t.Fatalf("hashNode must be deterministic, got %q vs %q", h1, h2)
	}

	// Module hash invariance to node order and imports/exports order
	mod1 := &Module{
		Name:       "math",
		NodeIDs:    []NodeID{"b", "a"},
		NodeHashes: map[NodeID]string{"a": "hash_a", "b": "hash_b"},
		Imports:    []string{"pkg_y", "pkg_x"},
		Exports:    []string{"fn_b", "fn_a"},
	}
	mod2 := &Module{
		Name:       "math",
		NodeIDs:    []NodeID{"a", "b"},
		NodeHashes: map[NodeID]string{"a": "hash_a", "b": "hash_b"},
		Imports:    []string{"pkg_x", "pkg_y"},
		Exports:    []string{"fn_a", "fn_b"},
	}
	if mod1.ComputeHash() != mod2.ComputeHash() {
		t.Fatalf("Module hash must be invariant to node ordering and imports/exports ordering")
	}

	// Self-hash field does not diverge from CAS key
	memStore := NewMemoryStore()
	modHash, err := PutModule(memStore, mod1)
	if err != nil {
		t.Fatalf("PutModule failed: %v", err)
	}
	if modHash != mod1.ComputeHash() {
		t.Fatalf("PutModule CAS key %s != mod.ComputeHash %s", modHash, mod1.ComputeHash())
	}
	if _, err := GetModule(memStore, modHash); err != nil {
		t.Fatalf("GetModule by compute hash failed: %v", err)
	}

	ev1 := &VerificationEvidence{
		GraphHash:       "g1",
		Target:          "bytecode",
		Verified:        true,
		InferredEffects: []Effect{{Type: "IO", Capability: "fs"}, {Type: "CPU"}},
	}
	ev2 := &VerificationEvidence{
		GraphHash:       "g1",
		Target:          "bytecode",
		Verified:        true,
		InferredEffects: []Effect{{Type: "CPU"}, {Type: "IO", Capability: "fs"}},
	}
	if ev1.ComputeHash() != ev2.ComputeHash() {
		t.Fatalf("VerificationEvidence hash must be invariant to effect slice order")
	}

	evHash, err := PutEvidence(memStore, ev1)
	if err != nil {
		t.Fatalf("PutEvidence failed: %v", err)
	}
	if evHash != ev1.ComputeHash() {
		t.Fatalf("PutEvidence CAS key %s != ev.ComputeHash %s", evHash, ev1.ComputeHash())
	}
	if _, err := GetEvidence(memStore, evHash); err != nil {
		t.Fatalf("GetEvidence by compute hash failed: %v", err)
	}

	art1 := &LoweredArtifact{Payload: []byte("bytecode_binary_data")}
	art2 := &LoweredArtifact{Payload: []byte("bytecode_binary_data")}
	if art1.ComputeHash() != art2.ComputeHash() {
		t.Fatalf("LoweredArtifact hash must be deterministic")
	}

	// Provenance normalization: line and column shifts do not invalidate NodeHash
	nodeWithLine1 := &Node{
		ID:         "n_prov",
		Kind:       "const",
		Value:      "100",
		Type:       ast.Layout(ast.Int),
		Provenance: Provenance{Filename: "foo.howl", Line: 1, Column: 5},
	}
	nodeWithLine50 := &Node{
		ID:         "n_prov",
		Kind:       "const",
		Value:      "100",
		Type:       ast.Layout(ast.Int),
		Provenance: Provenance{Filename: "foo.howl", Line: 50, Column: 12},
	}
	hL1, _ := hashNode(nodeWithLine1)
	hL50, _ := hashNode(nodeWithLine50)
	if hL1 != hL50 {
		t.Fatalf("hashNode must be invariant to line and column shifts within the same file")
	}
}

func TestContentAddressedStoreBackends(t *testing.T) {
	diskStore := newTestDiskStore(t)

	stores := []struct {
		name  string
		store Store
	}{
		{"memory", NewMemoryStore()},
		{"disk", diskStore},
	}

	for _, s := range stores {
		t.Run(s.name, func(t *testing.T) {
			store := s.store

			// Node round-trip with Effects preservation
			node := &Node{
				ID:      "n1",
				Kind:    "const",
				Value:   "100",
				Type:    ast.Layout(ast.Int),
				Effects: []Effect{{Type: "CPU"}},
			}
			nh, err := PutNode(store, node)
			if err != nil {
				t.Fatalf("PutNode failed: %v", err)
			}
			expectedNH, _ := hashNode(node)
			if nh != expectedNH {
				t.Fatalf("PutNode hash %s != expected %s", nh, expectedNH)
			}
			gotNode, err := GetNode(store, nh)
			if err != nil {
				t.Fatalf("GetNode failed: %v", err)
			}
			if gotNode.ID != node.ID || gotNode.Value != node.Value {
				t.Fatalf("GetNode returned mismatched node: %+v", gotNode)
			}
			if len(gotNode.Effects) != 1 || gotNode.Effects[0].Type != "CPU" {
				t.Fatalf("GetNode lost effects metadata: %+v", gotNode.Effects)
			}

			// Module round-trip
			mod := &Module{
				Name:       "core",
				NodeIDs:    []NodeID{node.ID},
				NodeHashes: map[NodeID]string{node.ID: nh},
				Imports:    []string{"dep_a"},
				Exports:    []string{"sym_b"},
			}
			mh, err := PutModule(store, mod)
			if err != nil {
				t.Fatalf("PutModule failed: %v", err)
			}
			if mh != mod.ComputeHash() {
				t.Fatalf("PutModule returned hash %s, want %s", mh, mod.ComputeHash())
			}
			gotMod, err := GetModule(store, mh)
			if err != nil {
				t.Fatalf("GetModule failed: %v", err)
			}
			if gotMod.Name != "core" || len(gotMod.NodeIDs) != 1 {
				t.Fatalf("GetModule returned mismatched module: %+v", gotMod)
			}

			// Evidence round-trip
			evidence := &VerificationEvidence{
				GraphHash: "gh1",
				Target:    "bytecode",
				Verified:  true,
			}
			eh, err := PutEvidence(store, evidence)
			if err != nil {
				t.Fatalf("PutEvidence failed: %v", err)
			}
			if eh != evidence.ComputeHash() {
				t.Fatalf("PutEvidence returned hash %s, want %s", eh, evidence.ComputeHash())
			}
			gotEv, err := GetEvidence(store, eh)
			if err != nil {
				t.Fatalf("GetEvidence failed: %v", err)
			}
			if gotEv.GraphHash != "gh1" || !gotEv.Verified {
				t.Fatalf("GetEvidence returned mismatched evidence: %+v", gotEv)
			}

			// Artifact round-trip
			artifact := &LoweredArtifact{
				SourceGraphHash: "gh1",
				Target:          "bytecode",
				ArtifactKind:    "bytecode",
				Payload:         []byte{0x01, 0x02, 0x03, 0x04},
			}
			ah, err := PutArtifact(store, artifact)
			if err != nil {
				t.Fatalf("PutArtifact failed: %v", err)
			}
			gotArt, err := GetArtifact(store, ah)
			if err != nil {
				t.Fatalf("GetArtifact failed: %v", err)
			}
			if !bytes.Equal(gotArt.Payload, artifact.Payload) {
				t.Fatalf("GetArtifact returned mismatched payload")
			}

			// Non-existent hash check
			missingHash := hex.EncodeToString(sha256.New().Sum(nil))
			if _, err := GetNode(store, missingHash); !errors.Is(err, ErrNotFound) {
				t.Fatalf("expected ErrNotFound, got %v", err)
			}

			// Invalid hash validation and path traversal rejection
			if _, err := store.GetBlob("invalid_short"); !errors.Is(err, ErrInvalidHash) {
				t.Fatalf("expected ErrInvalidHash for short hash, got %v", err)
			}
			if _, err := store.GetBlob("../../secret.txt"); !errors.Is(err, ErrInvalidHash) {
				t.Fatalf("expected ErrInvalidHash for path traversal attempt, got %v", err)
			}
			if err := store.DeleteBlob("../../secret.txt"); !errors.Is(err, ErrInvalidHash) {
				t.Fatalf("expected ErrInvalidHash on DeleteBlob with traversal, got %v", err)
			}
		})
	}
}

func TestIncrementalInvalidationAndPreservation(t *testing.T) {
	store := NewMemoryStore()
	compiler := NewIncrementalCompiler(store, "bytecode")

	// 1. Initial clean build
	g1 := buildBranchingGraph("20")
	res1, err := compiler.Compile(g1)
	if err != nil {
		t.Fatalf("clean build failed: %v", err)
	}
	if res1.ReusedCache {
		t.Fatal("clean build should not report ReusedCache")
	}
	if len(res1.PreservedNodes) != 0 {
		t.Fatalf("clean build should preserve 0 nodes, got %d", len(res1.PreservedNodes))
	}
	if len(res1.ChangedNodes) != 7 {
		t.Fatalf("clean build should compile all 7 nodes, got %d", len(res1.ChangedNodes))
	}
	if res1.LoweredCount != 7 {
		t.Fatalf("clean build should lower all 7 nodes, got %d", res1.LoweredCount)
	}

	// 2. Unrelated edit in branch 2 (change b1 from "20" to "25")
	g2 := buildBranchingGraph("25")
	res2, err := compiler.Compile(g2)
	if err != nil {
		t.Fatalf("incremental build failed: %v", err)
	}

	// Acceptance criteria: unrelated edits preserve cache outside the reverse dependency closure
	preservedMap := make(map[NodeID]bool)
	for _, id := range res2.PreservedNodes {
		preservedMap[id] = true
	}
	for _, expectedPreserved := range []NodeID{"a1", "a2", "a3"} {
		if !preservedMap[expectedPreserved] {
			t.Errorf("expected branch 1 node %s to be preserved, got preserved list: %v", expectedPreserved, res2.PreservedNodes)
		}
	}

	affectedMap := make(map[NodeID]bool)
	for _, id := range res2.AffectedClosure {
		affectedMap[id] = true
	}
	for _, expectedAffected := range []NodeID{"b1", "b2", "b3", "root"} {
		if !affectedMap[expectedAffected] {
			t.Errorf("expected node %s in affected closure, got: %v", expectedAffected, res2.AffectedClosure)
		}
	}

	// Verify that preserved subgraphs avoided lowering invocations
	if res2.LoweredCount >= 7 {
		t.Fatalf("incremental lowering must reuse preserved subgraphs, got lowered count %d (want < 7)", res2.LoweredCount)
	}

	// Verify that preserved nodes are still intact in CAS
	for _, preservedID := range []NodeID{"a1", "a2", "a3"} {
		h, _ := hashNode(g2.NodeByID(preservedID))
		if !store.HasBlob(h) {
			t.Errorf("preserved node %s blob missing from CAS", preservedID)
		}
	}

	// 3. Changing EntryNode invalidates cache and triggers re-compilation
	gEntryChanged := buildBranchingGraph("25")
	gEntryChanged.EntryNode = "a3" // change entry node to a3 without altering node bodies
	resEntry, err := compiler.Compile(gEntryChanged)
	if err != nil {
		t.Fatalf("compile with changed EntryNode failed: %v", err)
	}
	if resEntry.ReusedCache {
		t.Fatal("changing EntryNode must not falsely report ReusedCache")
	}

	// 4. Reverse dependency closure on deleted nodes invalidates previous dependents
	gWithExtra := buildBranchingGraph("25")
	extra := gWithExtra.AddNode(newConstNode("extra_leaf", "99"))
	gWithExtra.NodeByID("b1").DataInputs = append(gWithExtra.NodeByID("b1").DataInputs, DataEdge{SourceNode: extra})
	resExtra, err := compiler.Compile(gWithExtra)
	if err != nil {
		t.Fatalf("compile with extra node failed: %v", err)
	}
	_ = resExtra

	// Now remove extra_leaf: b1 and its dependents must be invalidated, not marked preserved
	gRemovedExtra := buildBranchingGraph("25")
	resRemoved, err := compiler.Compile(gRemovedExtra)
	if err != nil {
		t.Fatalf("compile with removed node failed: %v", err)
	}
	removedPreservedMap := make(map[NodeID]bool)
	for _, id := range resRemoved.PreservedNodes {
		removedPreservedMap[id] = true
	}
	if removedPreservedMap["b1"] || removedPreservedMap["b2"] || removedPreservedMap["b3"] || removedPreservedMap["root"] {
		t.Fatalf("dependents of removed node must not be marked preserved, got preserved: %v", resRemoved.PreservedNodes)
	}
}

func TestCleanBuildsReproduceManifests(t *testing.T) {
	graphA := buildBranchingGraph("42")
	graphB := buildBranchingGraph("42")

	storeA := NewMemoryStore()
	storeB := NewMemoryStore()

	compilerA := NewIncrementalCompiler(storeA, "bytecode")
	compilerB := NewIncrementalCompiler(storeB, "bytecode")

	resA, err := compilerA.Compile(graphA)
	if err != nil {
		t.Fatalf("compile A failed: %v", err)
	}
	resB, err := compilerB.Compile(graphB)
	if err != nil {
		t.Fatalf("compile B failed: %v", err)
	}

	// Acceptance criteria: clean builds reproduce manifests
	if resA.Manifest.ManifestHash != resB.Manifest.ManifestHash {
		t.Fatalf("manifest hashes diverge: %s vs %s", resA.Manifest.ManifestHash, resB.Manifest.ManifestHash)
	}
	if resA.Manifest.ArtifactHash != resB.Manifest.ArtifactHash {
		t.Fatalf("artifact hashes diverge: %s vs %s", resA.Manifest.ArtifactHash, resB.Manifest.ArtifactHash)
	}
	if resA.Manifest.VerificationEvidenceHash != resB.Manifest.VerificationEvidenceHash {
		t.Fatalf("evidence hashes diverge: %s vs %s", resA.Manifest.VerificationEvidenceHash, resB.Manifest.VerificationEvidenceHash)
	}

	// Validate against their respective stores
	if err := resA.Manifest.ValidateAgainstStore(storeA); err != nil {
		t.Fatalf("ValidateAgainstStore A failed: %v", err)
	}
	if err := resB.Manifest.ValidateAgainstStore(storeB); err != nil {
		t.Fatalf("ValidateAgainstStore B failed: %v", err)
	}

	// Permuted node insertion order must also reproduce identical manifest hashes
	graphPermuted := NewGraph()
	// Insert branch 2 first, then root, then branch 1
	graphPermuted.AddNode(newConstNode("b1", "42"))
	graphPermuted.AddNode(newBinaryNode("b2", "-", "b1", "b1"))
	graphPermuted.AddNode(newBinaryNode("b3", "+", "b2", "b2"))
	graphPermuted.AddNode(newBinaryNode("root", "+", "a3", "b3"))
	graphPermuted.AddNode(newConstNode("a1", "10"))
	graphPermuted.AddNode(newBinaryNode("a2", "+", "a1", "a1"))
	graphPermuted.AddNode(newBinaryNode("a3", "*", "a2", "a2"))
	graphPermuted.EntryNode = "root"

	storePermuted := NewMemoryStore()
	compilerPermuted := NewIncrementalCompiler(storePermuted, "bytecode")
	resPermuted, err := compilerPermuted.Compile(graphPermuted)
	if err != nil {
		t.Fatalf("compile permuted failed: %v", err)
	}
	if resPermuted.Manifest.ManifestHash != resA.Manifest.ManifestHash {
		t.Fatalf("permuted node order produced manifest hash %s, want %s",
			resPermuted.Manifest.ManifestHash, resA.Manifest.ManifestHash)
	}
	if resPermuted.Manifest.VerificationEvidenceHash != resA.Manifest.VerificationEvidenceHash {
		t.Fatalf("permuted node order produced evidence hash %s, want %s",
			resPermuted.Manifest.VerificationEvidenceHash, resA.Manifest.VerificationEvidenceHash)
	}
}

func corruptBlob(s *MemoryStore, hash string, payload string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.blobs[hash] = []byte(payload)
}

func TestCacheCorruptionCannotBypassVerification(t *testing.T) {
	store := NewMemoryStore()
	compiler := NewIncrementalCompiler(store, "bytecode")

	graph := buildBranchingGraph("99")
	res, err := compiler.Compile(graph)
	if err != nil {
		t.Fatalf("initial compile failed: %v", err)
	}

	// 1. Corrupt verification evidence in the store
	evHash := res.Manifest.VerificationEvidenceHash
	if !store.HasBlob(evHash) {
		t.Fatalf("expected evidence blob %s to exist", evHash)
	}
	corruptBlob(store, evHash, "corrupted_evidence_payload")

	// GetEvidence now returns ErrCorrupted
	if _, err := GetEvidence(store, evHash); !errors.Is(err, ErrCorrupted) {
		t.Fatalf("expected ErrCorrupted for tampered evidence, got: %v", err)
	}

	// Manifest validation fails closed
	if err := res.Manifest.ValidateAgainstStore(store); err == nil {
		t.Fatal("manifest validation must fail on corrupted evidence")
	}

	// Incremental compiler detects corruption and recovers by re-verifying
	recRes, err := compiler.Compile(graph)
	if err != nil {
		t.Fatalf("compiler recovery compile failed: %v", err)
	}
	if recRes.ReusedCache {
		t.Fatal("compiler must not reuse cache when evidence is corrupted")
	}
	if !recRes.Evidence.Verified {
		t.Fatal("recovered compilation must have verified evidence")
	}

	// 2. Corrupt artifact payload in store
	artHash := recRes.Manifest.ArtifactHash
	corruptBlob(store, artHash, "corrupted_bytecode_binary")

	if _, err := GetArtifact(store, artHash); !errors.Is(err, ErrCorrupted) {
		t.Fatalf("expected ErrCorrupted for tampered artifact, got: %v", err)
	}

	// Compiler recovers by re-lowering artifact
	recRes2, err := compiler.Compile(graph)
	if err != nil {
		t.Fatalf("recovery from corrupted artifact failed: %v", err)
	}
	if recRes2.ReusedCache {
		t.Fatal("compiler must not reuse cache when artifact is corrupted")
	}
	if recRes2.Artifact == nil || len(recRes2.Artifact.Payload) == 0 {
		t.Fatal("recovered compilation must have valid artifact payload")
	}

	// 3. Target mismatch falls closed (cannot reuse bytecode artifact for wasm)
	wasmCompiler := NewIncrementalCompiler(store, "wasm")
	wasmCompiler.SetPreviousState(graph, BuildDependencyIndex(graph), recRes2.Manifest)
	wasmRes, err := wasmCompiler.Compile(graph)
	if err != nil {
		t.Fatalf("wasm compile failed: %v", err)
	}
	if wasmRes.ReusedCache {
		t.Fatal("compiler must not reuse cache across different targets")
	}

	// 4. Broken graph with invalid references fails verification and is not cached as verified
	brokenGraph := NewGraph()
	badNode := brokenGraph.AddNode(&Node{
		ID:         "bad",
		Kind:       "binary",
		Value:      "+",
		Type:       ast.Layout(ast.Int),
		DataInputs: []DataEdge{{Name: "left", SourceNode: "non_existent"}, {Name: "right", SourceNode: "also_missing"}},
	})
	brokenGraph.EntryNode = badNode
	_, errBroken := compiler.Compile(brokenGraph)
	if errBroken == nil {
		t.Fatal("broken graph with invalid references must fail compilation")
	}

	// Restore clean state and assert cache hit works when uncorrupted
	cleanRes, err := compiler.Compile(graph)
	if err != nil {
		t.Fatalf("clean recompile failed: %v", err)
	}
	if !cleanRes.ReusedCache {
		t.Fatal("uncorrupted compilation must report ReusedCache == true")
	}
}

func TestDiskStoreCorruptionRecovery(t *testing.T) {
	diskStore := newTestDiskStore(t)

	compiler := NewIncrementalCompiler(diskStore, "bytecode")
	graph := buildBranchingGraph("50")

	res, err := compiler.Compile(graph)
	if err != nil {
		t.Fatalf("initial disk compile failed: %v", err)
	}

	// Directly mutate object file on disk
	evPath, err := diskStore.objectPath(res.Manifest.VerificationEvidenceHash)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(evPath, []byte("tampered disk bytes"), 0644); err != nil {
		t.Fatal(err)
	}

	if _, err := GetEvidence(diskStore, res.Manifest.VerificationEvidenceHash); !errors.Is(err, ErrCorrupted) {
		t.Fatalf("disk store must return ErrCorrupted on tampered file, got %v", err)
	}

	// Incremental compile recovers automatically and atomically overwrites tampered file
	recRes, err := compiler.Compile(graph)
	if err != nil {
		t.Fatalf("disk corruption recovery failed: %v", err)
	}
	if recRes.ReusedCache {
		t.Fatal("disk corruption must not bypass verification or reuse cache")
	}

	// Verify disk store file is now repaired and valid
	repairedBytes, err := os.ReadFile(evPath)
	if err != nil {
		t.Fatalf("failed to read repaired file: %v", err)
	}
	if hashBytes(repairedBytes) != res.Manifest.VerificationEvidenceHash {
		t.Fatalf("repaired file on disk still has hash mismatch")
	}
	if _, err := GetEvidence(diskStore, res.Manifest.VerificationEvidenceHash); err != nil {
		t.Fatalf("GetEvidence on repaired store failed: %v", err)
	}

	// Subsequent compile can now safely reuse cache
	cacheHitRes, err := compiler.Compile(graph)
	if err != nil {
		t.Fatalf("subsequent compile failed: %v", err)
	}
	if !cacheHitRes.ReusedCache {
		t.Fatal("expected cache hit after successful corruption recovery")
	}
}

func TestDependencyIndexAndClosures(t *testing.T) {
	g := buildBranchingGraph("10")
	idx := BuildDependencyIndex(g)

	// Test ForwardClosure
	fwdRoot := idx.ForwardClosure([]NodeID{"root"})
	if !fwdRoot["a3"] || !fwdRoot["b3"] || !fwdRoot["a2"] || !fwdRoot["b2"] || !fwdRoot["a1"] || !fwdRoot["b1"] {
		t.Errorf("ForwardClosure of root should contain all nodes, got: %v", fwdRoot)
	}

	// Test IsPreserved
	affected := idx.ReverseClosure([]NodeID{"b1"})
	if idx.IsPreserved("a1", affected) != true {
		t.Errorf("a1 should be preserved when b1 changes")
	}
	if idx.IsPreserved("root", affected) != false {
		t.Errorf("root should not be preserved when b1 changes")
	}

	// Test AffectedModules
	mods := idx.AffectedModules([]NodeID{"b1"})
	if !mods["main"] {
		t.Errorf("main module should be affected")
	}
}

func TestManifestSerialization(t *testing.T) {
	g := buildBranchingGraph("15")
	idx := BuildDependencyIndex(g)
	m := NewArtifactManifest(g, "bytecode", idx, "ev_hash", "art_hash", "bytecode")
	data, err := m.Serialize()
	if err != nil {
		t.Fatalf("Serialize failed: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("Serialize produced empty data")
	}
}
