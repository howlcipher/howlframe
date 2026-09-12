package hfir

import (
	"cmp"
	"slices"
)

// DependencyIndex maintains bidirectional dependency graphs across nodes and modules.
type DependencyIndex struct {
	Forward       map[NodeID][]NodeID `json:"forward"`
	Reverse       map[NodeID][]NodeID `json:"reverse"`
	ModuleForward map[string][]string `json:"module_forward"`
	ModuleReverse map[string][]string `json:"module_reverse"`
	NodeToModule  map[NodeID]string   `json:"node_to_module"`
}

// BuildDependencyIndex creates a deterministic forward and reverse dependency index.
func BuildDependencyIndex(graph *Graph) *DependencyIndex {
	idx := &DependencyIndex{
		Forward:       make(map[NodeID][]NodeID),
		Reverse:       make(map[NodeID][]NodeID),
		ModuleForward: make(map[string][]string),
		ModuleReverse: make(map[string][]string),
		NodeToModule:  make(map[NodeID]string),
	}
	if graph == nil {
		return idx
	}

	for _, node := range graph.Nodes {
		if node == nil {
			continue
		}
		mod := node.Module
		if mod == "" {
			mod = "main"
		}
		idx.NodeToModule[node.ID] = mod

		var deps []NodeID
		for _, in := range node.DataInputs {
			if in.SourceNode != "" {
				deps = append(deps, in.SourceNode)
			}
		}
		for _, ctrl := range node.ControlEdges {
			if ctrl != "" {
				deps = append(deps, ctrl)
			}
		}

		deps = dedupeAndSort(deps)
		idx.Forward[node.ID] = deps

		for _, dep := range deps {
			idx.Reverse[dep] = append(idx.Reverse[dep], node.ID)
		}
	}

	for id, rev := range idx.Reverse {
		idx.Reverse[id] = dedupeAndSort(rev)
	}

	for nodeID, deps := range idx.Forward {
		srcMod := idx.NodeToModule[nodeID]
		for _, depID := range deps {
			dstMod := idx.NodeToModule[depID]
			if srcMod != dstMod && srcMod != "" && dstMod != "" {
				idx.ModuleForward[srcMod] = append(idx.ModuleForward[srcMod], dstMod)
				idx.ModuleReverse[dstMod] = append(idx.ModuleReverse[dstMod], srcMod)
			}
		}
	}

	for mod, deps := range idx.ModuleForward {
		idx.ModuleForward[mod] = dedupeAndSort(deps)
	}
	for mod, rev := range idx.ModuleReverse {
		idx.ModuleReverse[mod] = dedupeAndSort(rev)
	}

	return idx
}

func (idx *DependencyIndex) bfsClosure(roots []NodeID, edgeMap map[NodeID][]NodeID) map[NodeID]bool {
	closure := make(map[NodeID]bool)
	if idx == nil || len(roots) == 0 {
		return closure
	}
	queue := append([]NodeID(nil), roots...)
	for len(queue) > 0 {
		curr := queue[0]
		queue = queue[1:]
		if closure[curr] {
			continue
		}
		closure[curr] = true
		for _, next := range edgeMap[curr] {
			if !closure[next] {
				queue = append(queue, next)
			}
		}
	}
	return closure
}

// ReverseClosure returns all nodes reachable by following reverse dependencies.
// This forms the exact invalidation closure for a set of changed nodes.
func (idx *DependencyIndex) ReverseClosure(changed []NodeID) map[NodeID]bool {
	return idx.bfsClosure(changed, idx.Reverse)
}

// ForwardClosure returns all nodes that the given nodes transitively depend upon.
func (idx *DependencyIndex) ForwardClosure(nodes []NodeID) map[NodeID]bool {
	return idx.bfsClosure(nodes, idx.Forward)
}

// IsPreserved reports whether a node lies outside the affected reverse closure.
func (idx *DependencyIndex) IsPreserved(node NodeID, affectedClosure map[NodeID]bool) bool {
	return !affectedClosure[node]
}

// AffectedModules returns the set of modules affected by a set of changed nodes.
func (idx *DependencyIndex) AffectedModules(changed []NodeID) map[string]bool {
	closure := idx.ReverseClosure(changed)
	affectedMods := make(map[string]bool)
	for id := range closure {
		if mod, ok := idx.NodeToModule[id]; ok && mod != "" {
			affectedMods[mod] = true
		}
	}
	return affectedMods
}

func dedupeAndSort[T cmp.Ordered](items []T) []T {
	if len(items) == 0 {
		return nil
	}
	seen := make(map[T]bool, len(items))
	res := make([]T, 0, len(items))
	for _, item := range items {
		if !seen[item] {
			seen[item] = true
			res = append(res, item)
		}
	}
	slices.Sort(res)
	return res
}
