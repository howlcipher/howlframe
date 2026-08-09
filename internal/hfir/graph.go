package hfir

import (
	"encoding/json"
	"fmt"
	"github.com/howlcipher/howlframe/internal/ast"
)

type NodeID string

type Graph struct {
	Version   string           `json:"version"`
	Nodes     []*Node          `json:"nodes"`
	EntryNode NodeID           `json:"entry_node,omitempty"`
	nodeMap   map[NodeID]*Node `json:"-"`
	nextID    uint64           `json:"-"`
}

type Node struct {
	ID           NodeID       `json:"id"`
	Kind         string       `json:"kind"`
	Module       string       `json:"module,omitempty"`
	Provenance   Provenance   `json:"provenance"`
	DataInputs   []DataEdge   `json:"data_inputs,omitempty"`
	ControlEdges []NodeID     `json:"control_edges,omitempty"`
	Effects      []Effect     `json:"effects,omitempty"`
	Type         ast.TypeInfo `json:"type"`
	Value        string       `json:"value,omitempty"`
}

type Provenance struct {
	Filename string `json:"filename"`
	Line     int    `json:"line"`
	Column   int    `json:"column"`
}

type DataEdge struct {
	Name       string       `json:"name,omitempty"`
	SourceNode NodeID       `json:"source_node,omitempty"`
	Type       ast.TypeInfo `json:"type,omitempty"`
}

type Effect struct {
	Type       string `json:"type"`
	Capability string `json:"capability,omitempty"`
}

func NewGraph() *Graph {
	return &Graph{
		Version: "v1",
		Nodes:   make([]*Node, 0),
		nodeMap: make(map[NodeID]*Node),
		nextID:  1,
	}
}

func (g *Graph) AddNode(n *Node) NodeID {
	if n.ID == "" {
		n.ID = NodeID(fmt.Sprintf("n%d", g.nextID))
		g.nextID++
	}
	g.Nodes = append(g.Nodes, n)
	g.nodeMap[n.ID] = n
	return n.ID
}

func (g *Graph) Serialize() ([]byte, error) {
	return json.MarshalIndent(g, "", "  ")
}
