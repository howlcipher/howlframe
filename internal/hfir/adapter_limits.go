package hfir

import "fmt"

const maxAdapterDepth = 2048
const maxAdapterExpansion = 65536

// Only closed scalar expressions are outlined. Variable reads, allocations,
// control flow and effects retain their original evaluation sites and order.
func adapterSharedPure(graph *Graph) map[NodeID]string {
	t := transportFromGraph(graph)
	byID := map[NodeID]transportNode{}
	uses := map[NodeID]int{}
	names := map[string]bool{}
	for _, n := range t.Nodes {
		byID[n.ID] = n
		names[n.Value] = true
		for _, e := range n.Inputs {
			uses[e.NodeID]++
		}
	}
	known := map[NodeID]bool{}
	visited := map[NodeID]bool{}
	var pure func(NodeID) bool
	pure = func(id NodeID) bool {
		if visited[id] {
			return known[id]
		}
		visited[id] = true
		n := byID[id]
		ok := n.Kind == "const" || n.Kind == "binary"
		for _, e := range n.Inputs {
			if !pure(e.NodeID) {
				ok = false
			}
		}
		known[id] = ok
		return ok
	}
	result := map[NodeID]string{}
	for _, n := range t.Nodes {
		if uses[n.ID] > 1 && n.Kind != "const" && pure(n.ID) {
			name := fmt.Sprintf("__hfir_shared_%s", n.ID)
			for names[name] {
				name += "_"
			}
			names[name] = true
			result[n.ID] = name
		}
	}
	return result
}

// Count expansion with saturating arithmetic before recursive validation and
// fragment allocation. Depth is independent of the amount of DAG sharing.
func validateAdapterExpansion(t candidateTransport) *Diagnostic {
	g := NewGraph()
	for _, n := range t.Nodes {
		g.AddNode(nodeFromTransport(n))
	}
	g.EntryNode = t.EntryNode
	shared := adapterSharedPure(g)
	byID := map[NodeID]transportNode{}
	for _, n := range t.Nodes {
		byID[n.ID] = n
	}
	type cost struct{ depth, size int }
	memo := map[NodeID]cost{}
	var walk func(NodeID) cost
	walk = func(id NodeID) cost {
		if c, ok := memo[id]; ok {
			return c
		}
		c := cost{1, 1}
		for _, e := range byID[id].Inputs {
			child := walk(e.NodeID)
			if child.depth+1 > c.depth {
				c.depth = child.depth + 1
			}
			size := child.size
			if _, ok := shared[e.NodeID]; ok {
				size = 1
			}
			c.size += size
			if c.size > maxAdapterExpansion {
				c.size = maxAdapterExpansion + 1
			}
		}
		memo[id] = c
		return c
	}
	c := walk(t.EntryNode)
	if c.depth > maxAdapterDepth {
		d := adapterDiagnostic("HFIR_TRANSPORT_TOO_DEEP", "candidate exceeds 2048 dependency levels", t.EntryNode)
		return &d
	}
	total := 0
	for _, fragment := range memo {
		total += fragment.size
		if total > 4*1024*1024 {
			break
		}
	}
	if c.size > maxAdapterExpansion || total > 4*1024*1024 {
		d := adapterDiagnostic("HFIR_TRANSPORT_TOO_LARGE", "candidate fragment expansion exceeds 65536 nodes", t.EntryNode)
		return &d
	}
	return nil
}
