package hfir

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"reflect"
	"strconv"
	"unicode/utf8"
)

// CompactRepairSchemaVersion identifies the process-local graph repair transport.
const CompactRepairSchemaVersion = "hfir-compact-repair/v1"

// CompactRepairSession retains trusted authority inside one process. Its public
// request and context ID cannot recreate that authority. It does not edit source,
// persist evidence, authenticate external signers, or grant execution permission.
// A session may be reused against an identical original candidate; it provides no
// one-time replay prevention. Callers must not concurrently mutate candidates.
type CompactRepairSession struct {
	context RepairContext
	request []byte
	id      string
}

type compactRepairNode struct {
	Node             transportNode `json:"node"`
	ExpectedNodeHash string        `json:"expected_node_hash"`
}

type compactRepairRequest struct {
	SchemaVersion           string              `json:"schema_version"`
	ContextID               string              `json:"context_id,omitempty"`
	GraphHash               string              `json:"graph_hash"`
	GraphVersion            string              `json:"graph_version"`
	Nodes                   []compactRepairNode `json:"editable_nodes"`
	AllowedReferenceNodeIDs []NodeID            `json:"allowed_reference_node_ids"`
	MaxOperations           int                 `json:"max_operations"`
}

type compactRepairResponse struct {
	SchemaVersion string            `json:"schema_version"`
	ContextID     string            `json:"context_id"`
	Operations    []repairOperation `json:"operations"`
}

// NewCompactRepairSession accepts only an intact, trusted semantic or localized
// context. It recomputes authority from the current graph and retains that fresh
// context rather than any caller-owned slice or map. JSON-decoded contexts have
// no private integrity seal and are deliberately insufficient.
func NewCompactRepairSession(candidate Candidate, trusted RepairContext) (*CompactRepairSession, error) {
	if !compactCandidateTextValid(candidate) || candidate.Hash == "" || GraphHash(candidate.Graph) != candidate.Hash || trusted.GraphHash != candidate.Hash {
		return nil, fmt.Errorf("current canonical candidate graph is required")
	}
	if trusted.integrityHash == "" || trusted.integrityHash != semanticRepairContextHash(trusted) {
		return nil, fmt.Errorf("intact trusted repair context is required")
	}
	var fresh RepairContext
	var err error
	if len(trusted.CoreNodeIDs) > 0 && sameNodeIDSlice(trusted.TargetNodeIDs, trusted.CoreNodeIDs) {
		fresh, err = NewLocalizedRepairContext(candidate, trusted.CoreNodeIDs)
	} else {
		fresh, err = NewSemanticRepairContext(candidate, trusted.TargetNodeIDs)
	}
	if err != nil || !sameSemanticRepairContext(trusted, fresh) {
		return nil, fmt.Errorf("repair context is not the trusted derived region")
	}
	// RepairNodeView contains DataEdge.Type, whose nested maps/pointers must not
	// retain aliases into the caller's graph. Clone the complete public context.
	sealed, err := json.Marshal(fresh)
	if err != nil {
		return nil, fmt.Errorf("cannot retain trusted context")
	}
	var retained RepairContext
	if err = json.Unmarshal(sealed, &retained); err != nil {
		return nil, fmt.Errorf("cannot retain trusted context")
	}
	retained.integrityHash = fresh.integrityHash
	fresh = retained
	request := compactRepairRequest{SchemaVersion: CompactRepairSchemaVersion, GraphHash: candidate.Hash, GraphVersion: candidate.Graph.Version, AllowedReferenceNodeIDs: append([]NodeID(nil), fresh.AllowedReferenceNodeIDs...), MaxOperations: fresh.MaxOperations}
	graphTransport := transportFromGraph(candidate.Graph)
	for _, id := range fresh.EditableNodeIDs {
		for _, node := range graphTransport.Nodes {
			if node.ID == id {
				request.Nodes = append(request.Nodes, compactRepairNode{Node: node, ExpectedNodeHash: NodeHash(candidate.Graph.NodeByID(id))})
				break
			}
		}
	}
	payload, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("cannot encode compact repair request")
	}
	digest := sha256.Sum256(append([]byte("hfir-compact-repair-context/v1\x00"), payload...))
	request.ContextID = hex.EncodeToString(digest[:])
	encoded, err := json.Marshal(request)
	if err != nil || len(encoded) > maxTransportBytes {
		return nil, fmt.Errorf("compact repair request exceeds transport bounds")
	}
	return &CompactRepairSession{context: fresh, request: encoded, id: request.ContextID}, nil
}

// RequestJSON returns an independent deterministic model view. The context ID is
// an identifier bound to private session state, not a signature or bearer grant.
func (s *CompactRepairSession) RequestJSON() []byte {
	if s == nil {
		return nil
	}
	return append([]byte(nil), s.request...)
}

// Apply validates the compact response against retained authority, reconstructs
// the legacy protected-node preconditions locally, and delegates the complete
// atomic verification/lowering gate to ApplyRepair. It never mutates the input.
func (s *CompactRepairSession) Apply(candidate Candidate, data []byte) (Candidate, []Diagnostic) {
	reject := func(code, message string) (Candidate, []Diagnostic) {
		return Candidate{}, []Diagnostic{adapterDiagnostic(code, message, "")}
	}
	if s == nil || s.id == "" || s.context.integrityHash == "" {
		return reject("HFIR_REPAIR_SCOPE", "trusted compact repair session is required")
	}
	if !compactCandidateTextValid(candidate) {
		return reject("HFIR_REPAIR_STALE", "current canonical textual candidate is required")
	}
	if !utf8.Valid(data) {
		return reject("HFIR_TRANSPORT_INVALID", "compact repair JSON must be valid UTF-8")
	}
	var response compactRepairResponse
	if diagnostic := strictDecode(data, &response); diagnostic != nil {
		return Candidate{}, []Diagnostic{*diagnostic}
	}
	if !compactJSONScalarsValid(data) {
		return reject("HFIR_TRANSPORT_INVALID", "compact repair JSON contains an unpaired Unicode surrogate")
	}
	if response.SchemaVersion != CompactRepairSchemaVersion {
		return reject("HFIR_REPAIR_INVALID", "unsupported compact repair schema version")
	}
	if response.ContextID != s.id {
		return reject("HFIR_REPAIR_SCOPE", "compact repair response does not match retained context")
	}
	delta := repairTransport{SchemaVersion: SemanticRepairSchemaVersion, ExpectedGraphHash: s.context.GraphHash, ExpectedGraphVersion: s.context.ExpectedGraphVersion, ProtectedNodeHashes: s.context.ProtectedNodeHashes, Operations: response.Operations}
	encoded, err := json.Marshal(delta)
	if err != nil {
		return reject("HFIR_TRANSPORT_INVALID", "cannot encode compact repair response")
	}
	return ApplyRepair(candidate, s.context, encoded)
}

// Validate text before JSON can silently replace malformed bytes. These are
// exactly the text fields crossing the candidate transport boundary.
func compactCandidateTextValid(candidate Candidate) bool {
	if candidate.Graph == nil || !compactTextValueValid(reflect.ValueOf(candidate.Graph), 0) || !utf8.ValidString(candidate.Graph.Version) || !utf8.ValidString(string(candidate.Graph.EntryNode)) {
		return false
	}
	for _, n := range candidate.Graph.Nodes {
		if n == nil {
			return false
		}
		for _, v := range []string{string(n.ID), n.Kind, n.Value, n.LiteralKind, n.Provenance.Filename, n.Module} {
			if !utf8.ValidString(v) {
				return false
			}
		}
		for _, e := range n.DataInputs {
			if !utf8.ValidString(e.Name) || !utf8.ValidString(string(e.SourceNode)) {
				return false
			}
		}
		for _, id := range n.ControlEdges {
			if !utf8.ValidString(string(id)) {
				return false
			}
		}
	}
	return true
}

// Reject lossy JSON text, including nested type metadata. Depth is bounded so
// cyclic or excessive caller-built metadata cannot recurse without limit.
func compactTextValueValid(v reflect.Value, depth int) bool {
	if depth > 64 {
		return false
	}
	switch v.Kind() {
	case reflect.Pointer, reflect.Interface:
		return v.IsNil() || compactTextValueValid(v.Elem(), depth+1)
	case reflect.String:
		return utf8.ValidString(v.String())
	case reflect.Struct:
		for i := 0; i < v.NumField(); i++ {
			f := v.Type().Field(i)
			if f.PkgPath == "" && f.Tag.Get("json") != "-" && !compactTextValueValid(v.Field(i), depth+1) {
				return false
			}
		}
	case reflect.Slice, reflect.Array:
		for i := 0; i < v.Len(); i++ {
			if !compactTextValueValid(v.Index(i), depth+1) {
				return false
			}
		}
	case reflect.Map:
		it := v.MapRange()
		for it.Next() {
			if !compactTextValueValid(it.Key(), depth+1) || !compactTextValueValid(it.Value(), depth+1) {
				return false
			}
		}
	}
	return true
}

// JSON permits escapes which encoding/json replaces lossily. After strictDecode
// has established JSON syntax, scan only string escapes to require scalar values.
// Escaped backslashes are skipped and cannot introduce a Unicode escape.
func compactJSONScalarsValid(data []byte) bool {
	for i := 0; i < len(data); i++ {
		if data[i] != '"' {
			continue
		}
		i++
		for i < len(data) && data[i] != '"' {
			if data[i] != '\\' {
				i++
				continue
			}
			i++
			if i >= len(data) {
				return false
			}
			if data[i] != 'u' {
				i++
				continue
			}
			if i+5 > len(data) {
				return false
			}
			unit, err := strconv.ParseUint(string(data[i+1:i+5]), 16, 16)
			if err != nil {
				return false
			}
			i += 5
			if unit >= 0xd800 && unit <= 0xdbff {
				if i+6 > len(data) || data[i] != '\\' || data[i+1] != 'u' {
					return false
				}
				low, err := strconv.ParseUint(string(data[i+2:i+6]), 16, 16)
				if err != nil || low < 0xdc00 || low > 0xdfff {
					return false
				}
				i += 6
			} else if unit >= 0xdc00 && unit <= 0xdfff {
				return false
			}
		}
	}
	return true
}
