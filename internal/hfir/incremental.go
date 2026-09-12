package hfir

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"sort"

	"github.com/howlcipher/howlframe/internal/bytecode"
)

// CompileResult represents the outcome of an incremental compilation step.
type CompileResult struct {
	Graph           *Graph
	Manifest        *ArtifactManifest
	Artifact        *LoweredArtifact
	Evidence        *VerificationEvidence
	ChangedNodes    []NodeID
	AffectedClosure []NodeID
	PreservedNodes  []NodeID
	ReusedCache     bool
	Diagnostics     []Diagnostic
	LoweredCount    int
}

// IncrementalCompiler compiles HFIR graphs using content-addressed caching.
type IncrementalCompiler struct {
	store       Store
	target      string
	lastGraph   *Graph
	lastIndex   *DependencyIndex
	lastMan     *ArtifactManifest
	cachedInsts map[NodeID][]bytecode.BCInstruction
}

// NewIncrementalCompiler initializes an incremental compiler with backing storage.
func NewIncrementalCompiler(store Store, target string) *IncrementalCompiler {
	return &IncrementalCompiler{
		store:  store,
		target: target,
	}
}

// SetPreviousState restores previously cached compilation state.
func (c *IncrementalCompiler) SetPreviousState(graph *Graph, index *DependencyIndex, manifest *ArtifactManifest) {
	c.lastGraph = graph
	c.lastIndex = index
	c.lastMan = manifest
}

// Compile incrementally verifies, lowers, and caches the HFIR graph.
func (c *IncrementalCompiler) Compile(graph *Graph) (*CompileResult, error) {
	if graph == nil {
		return nil, errors.New("cannot compile nil graph")
	}

	index := BuildDependencyIndex(graph)

	var changedNodes []NodeID
	var affectedClosure []NodeID
	var preservedNodes []NodeID

	if c.lastMan == nil || c.lastGraph == nil {
		// Clean build: all nodes are new
		for _, node := range graph.Nodes {
			if node != nil {
				changedNodes = append(changedNodes, node.ID)
			}
		}
		affectedClosure = append([]NodeID(nil), changedNodes...)
	} else {
		// Incremental build: detect modified or added nodes
		currentHashes := make(map[NodeID]string)
		for _, node := range graph.Nodes {
			if node != nil {
				h, _ := hashNode(node)
				currentHashes[node.ID] = h
				if prevHash, exists := c.lastMan.NodeHashes[node.ID]; !exists || prevHash != h {
					changedNodes = append(changedNodes, node.ID)
				}
			}
		}

		// Detect removed nodes
		var removedNodes []NodeID
		for prevID := range c.lastMan.NodeHashes {
			if _, exists := currentHashes[prevID]; !exists {
				changedNodes = append(changedNodes, prevID)
				removedNodes = append(removedNodes, prevID)
			}
		}

		// Invalidate if entry node or overall graph hash changed
		if c.lastMan.EntryNode != graph.EntryNode || c.lastMan.GraphHash != GraphHash(graph) {
			if graph.EntryNode != "" {
				changedNodes = append(changedNodes, graph.EntryNode)
			}
		}

		closureSet := index.ReverseClosure(changedNodes)

		// Consult previous index and manifest for reverse dependencies of removed nodes
		if c.lastIndex != nil && len(removedNodes) > 0 {
			for id := range c.lastIndex.ReverseClosure(removedNodes) {
				closureSet[id] = true
			}
		} else if c.lastMan != nil && len(c.lastMan.ReverseDependencies) > 0 && len(removedNodes) > 0 {
			queue := append([]NodeID(nil), removedNodes...)
			visited := make(map[NodeID]bool)
			for len(queue) > 0 {
				curr := queue[0]
				queue = queue[1:]
				if visited[curr] {
					continue
				}
				visited[curr] = true
				closureSet[curr] = true
				for _, next := range c.lastMan.ReverseDependencies[curr] {
					if !visited[next] {
						queue = append(queue, next)
					}
				}
			}
		}

		for id := range closureSet {
			affectedClosure = append(affectedClosure, id)
		}
		sort.Slice(affectedClosure, func(i, j int) bool { return affectedClosure[i] < affectedClosure[j] })

		for _, node := range graph.Nodes {
			if node != nil && !closureSet[node.ID] {
				preservedNodes = append(preservedNodes, node.ID)
			}
		}
	}

	changedNodes = dedupeAndSort(changedNodes)
	preservedNodes = dedupeAndSort(preservedNodes)

	// If no nodes changed, check whether cached compilation can be reused safely
	if len(changedNodes) == 0 && c.lastMan != nil && c.lastMan.Target == c.target &&
		c.lastMan.GraphHash == GraphHash(graph) && c.lastMan.EntryNode == graph.EntryNode {
		if err := c.lastMan.ValidateAgainstStore(c.store); err == nil {
			evidence, errEv := GetEvidence(c.store, c.lastMan.VerificationEvidenceHash)
			artifact, errArt := GetArtifact(c.store, c.lastMan.ArtifactHash)

			if errEv == nil && errArt == nil && evidence != nil && evidence.Verified &&
				evidence.GraphHash == c.lastMan.GraphHash && evidence.Target == c.target &&
				artifact != nil && hashBytes(artifact.Payload) == c.lastMan.ArtifactHash {
				return &CompileResult{
					Graph:           graph,
					Manifest:        c.lastMan,
					Artifact:        artifact,
					Evidence:        evidence,
					ChangedNodes:    nil,
					AffectedClosure: nil,
					PreservedNodes:  preservedNodes,
					ReusedCache:     true,
					Diagnostics:     evidence.Diagnostics,
					LoweredCount:    0,
				}, nil
			}
		}
	}

	// Verify the graph
	verifier := NewVerifier(graph, c.target)
	diags := verifier.Verify()

	var blocking []Diagnostic
	for _, d := range diags {
		if d.Severity == SeverityError {
			blocking = append(blocking, d)
		}
	}
	if len(blocking) > 0 {
		return &CompileResult{
			Graph:           graph,
			ChangedNodes:    changedNodes,
			AffectedClosure: affectedClosure,
			PreservedNodes:  preservedNodes,
			Diagnostics:     diags,
		}, fmt.Errorf("verification failed with %d blocking diagnostics", len(blocking))
	}

	// Lower artifact incrementally (skipping lowering for preserved subgraphs)
	artifactPayload, artifactKind, loweredCount, newCachedInsts, err := c.lowerArtifact(graph, preservedNodes)
	if err != nil {
		return nil, err
	}

	// Persist nodes to CAS
	for _, node := range graph.Nodes {
		if node != nil {
			if _, err := PutNode(c.store, node); err != nil {
				return nil, fmt.Errorf("cache node %s: %w", node.ID, err)
			}
		}
	}

	// Store module entities in CAS
	modules, _ := buildGraphModules(graph)
	for _, mod := range modules {
		if _, err := PutModule(c.store, mod); err != nil {
			return nil, fmt.Errorf("cache module %s: %w", mod.Name, err)
		}
	}

	evidence := &VerificationEvidence{
		GraphHash:       GraphHash(graph),
		Target:          c.target,
		Verified:        true,
		Diagnostics:     diags,
		InferredEffects: collectEffects(graph),
	}
	evHash, err := PutEvidence(c.store, evidence)
	if err != nil {
		return nil, fmt.Errorf("cache evidence: %w", err)
	}

	artifact := &LoweredArtifact{
		SourceGraphHash: evidence.GraphHash,
		Target:          c.target,
		ArtifactKind:    artifactKind,
		Payload:         artifactPayload,
	}
	artHash, err := PutArtifact(c.store, artifact)
	if err != nil {
		return nil, fmt.Errorf("cache artifact: %w", err)
	}

	manifest := NewArtifactManifest(graph, c.target, index, evHash, artHash, artifactKind)
	if _, err := PutManifest(c.store, manifest); err != nil {
		return nil, fmt.Errorf("cache manifest: %w", err)
	}

	c.lastGraph = graph
	c.lastIndex = index
	c.lastMan = manifest
	c.cachedInsts = newCachedInsts

	return &CompileResult{
		Graph:           graph,
		Manifest:        manifest,
		Artifact:        artifact,
		Evidence:        evidence,
		ChangedNodes:    changedNodes,
		AffectedClosure: affectedClosure,
		PreservedNodes:  preservedNodes,
		ReusedCache:     false,
		Diagnostics:     diags,
		LoweredCount:    loweredCount,
	}, nil
}

func (c *IncrementalCompiler) lowerArtifact(graph *Graph, preservedNodes []NodeID) ([]byte, string, int, map[NodeID][]bytecode.BCInstruction, error) {
	if c.target == "bytecode" {
		preservedCache := make(map[NodeID][]bytecode.BCInstruction)
		if c.cachedInsts != nil {
			for _, id := range preservedNodes {
				if insts, ok := c.cachedInsts[id]; ok {
					preservedCache[id] = insts
				}
			}
		}
		prog, newCache, loweredCount, diags := LowerToBytecodeIncremental(graph, preservedCache)
		if len(diags) > 0 {
			for _, d := range diags {
				if d.Severity == SeverityError {
					return nil, "", loweredCount, nil, fmt.Errorf("lowering to bytecode: %s", d.Message)
				}
			}
		}
		var buf bytes.Buffer
		if err := bytecode.WriteArtifact(&buf, prog); err != nil {
			return nil, "", loweredCount, nil, fmt.Errorf("encode bytecode artifact: %w", err)
		}
		return buf.Bytes(), "bytecode", loweredCount, newCache, nil
	}

	// Target-independent or canonical fallback artifact
	data, err := json.Marshal(graph)
	if err != nil {
		return nil, "", len(graph.Nodes), nil, fmt.Errorf("serialize graph artifact: %w", err)
	}
	return data, "canonical_graph", len(graph.Nodes), nil, nil
}

func collectEffects(graph *Graph) []Effect {
	if graph == nil {
		return nil
	}
	nodes := append([]*Node(nil), graph.Nodes...)
	sort.Slice(nodes, func(i, j int) bool { return nodes[i].ID < nodes[j].ID })
	var effects []Effect
	for _, node := range nodes {
		if node != nil && len(node.Effects) > 0 {
			effects = append(effects, node.Effects...)
		}
	}
	return effects
}
