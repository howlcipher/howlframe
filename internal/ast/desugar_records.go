package ast

// Records and JSON sugar (heldout-v3 records slice). Every form expands into
// existing, already lowered constructs (let, do, for, when, set, dict,
// map_get, map_set, map_keys, list_get, append, is_nil, nz, map_list,
// parse_json, encode_json, read_text, write_file), so it adds no opcode,
// capability or effect. read_json/write_json reach the filesystem only
// through the read_file/write_file they expand to, so they need exactly the
// filesystem:read / filesystem:write grant those need and fail closed
// without it. Go/JS targets reject every form with UNSUPPORTED_ON_TARGET.
//
//   (rec k v ...)          record literal: (dict (k v) ...). A key is a name
//                          (used as its own string: (rec id 1) has key "id")
//                          or a string literal; values are expressions.
//   (get m k1 k2 ...)      nested path read. An integer literal step indexes
//                          a list (list_get); every other step is a map key
//                          (map_get), so a missing key gives nil.
//   (put m k1 ... kn v)    nested path write in place: missing or nil
//                          intermediate map keys get a fresh (dict); integer
//                          literal intermediate steps index a list. The last
//                          key must be a map key. Statement position.
//   (read_json path)       (parse_json Any (read_text path)).
//   (write_json path v)    (write_file path (encode_json v)). encode_json
//                          writes object keys in sorted (UTF-8 byte) order,
//                          compact, so output is stable.
//   (pluck lst k)          list of (map_get r k) for each record r.
//   (select lst k ...)     list of records with only the keys k (names or
//                          strings, like rec); a missing key becomes null.
//   (group_by x lst key)   dict from (str key) to the elements with that key,
//                          in input order.
//   (merge a b ...)        new dict with the keys of a, then b, ... (later
//                          wins). The operands are not modified.

// RecordHeads lists every head added by this file, for the Go/JS target
// rejection and the reserved-call list.
var RecordHeads = []string{"rec", "get", "put", "read_json", "write_json", "pluck", "select", "group_by", "merge"}

// IsRecordSugar reports whether head is a form of this file.
func IsRecordSugar(head string) bool {
	for _, h := range RecordHeads {
		if h == head {
			return true
		}
	}
	return false
}

func rnList(at *Node, kids ...*Node) *Node {
	return &Node{Type: "List", Line: at.Line, Column: at.Column, Filename: at.Filename, Children: kids}
}

func rnSym(name string, at *Node) *Node { return symNode(name, at) }

func rnTemp(prefix string, at *Node) string { return compactTemp("rc" + prefix) }

func rnIsIntLit(n *Node) bool { return n != nil && n.Type == "INT" }

// rnKey turns a rec/select key (a name or a string literal) into a STRING.
func rnKey(head string, n *Node) (*Node, bool) {
	switch n.Type {
	case "SYMBOL":
		return strNode(n.Value, n), true
	case "STRING":
		return strNode(n.Value, n), true
	}
	ReportError(head+" keys must be names or string literals", n.Line, n.Column)
	return nil, false
}

// rnStep reads one path step from the symbol t.
func rnStep(t string, key *Node, at *Node) *Node {
	if rnIsIntLit(key) {
		return rnList(at, rnSym("list_get", at), rnSym(t, at), key)
	}
	return rnList(at, rnSym("map_get", at), rnSym(t, at), key)
}

func rnLet(at *Node, body *Node, pairs ...*Node) *Node {
	return rnList(at, rnSym("let", at), rnList(at, pairs...), body)
}

func desugarRecords(node *Node) bool {
	head := node.Children[0].Value
	args := node.Children[1:]
	at := node
	switch head {
	case "rec":
		if len(args)%2 != 0 {
			ReportError("rec expects key/value pairs", node.Line, node.Column)
			return false
		}
		kids := []*Node{rnSym("dict", at)}
		for i := 0; i < len(args); i += 2 {
			k, ok := rnKey(head, args[i])
			if !ok {
				return false
			}
			kids = append(kids, rnList(at, k, args[i+1]))
		}
		node.Children = kids
	case "get":
		// (let (T0 m) (let (T1 step) ... stepN))
		t := rnTemp("G", at)
		var build func(cur string, keys []*Node) *Node
		build = func(cur string, keys []*Node) *Node {
			if len(keys) == 1 {
				return rnStep(cur, keys[0], at)
			}
			nt := rnTemp("G", at)
			return rnLet(at, build(nt, keys[1:]), rnSym(nt, at), rnStep(cur, keys[0], at))
		}
		node.Children = rnLet(at, build(t, args[1:]), rnSym(t, at), args[0]).Children
	case "put":
		if rnIsIntLit(args[len(args)-2]) {
			ReportError("put: the last key must be a map key, not a list index", node.Line, node.Column)
			return false
		}
		t := rnTemp("P", at)
		var build func(cur string, keys []*Node) *Node
		build = func(cur string, keys []*Node) *Node {
			if len(keys) == 1 {
				return rnList(at, rnSym("map_set", at), rnSym(cur, at), keys[0], args[len(args)-1])
			}
			nt := rnTemp("P", at)
			if rnIsIntLit(keys[0]) {
				return rnLet(at, build(nt, keys[1:]), rnSym(nt, at), rnStep(cur, keys[0], at))
			}
			kt := rnTemp("K", at)
			// (let (K key) (do (when (is_nil (map_get cur K)) (map_set cur K (dict)))
			//                 (let (N (map_get cur K)) rest)))
			// No reassignment, so the checker never has to unify N's type.
			fill := rnList(at, rnSym("when", at), rnList(at, rnSym("is_nil", at), rnList(at, rnSym("map_get", at), rnSym(cur, at), rnSym(kt, at))),
				rnList(at, rnSym("map_set", at), rnSym(cur, at), rnSym(kt, at), rnList(at, rnSym("dict", at))))
			rest := rnLet(at, build(nt, keys[1:]), rnSym(nt, at), rnList(at, rnSym("map_get", at), rnSym(cur, at), rnSym(kt, at)))
			return rnLet(at, rnList(at, rnSym("do", at), fill, rest), rnSym(kt, at), keys[0])
		}
		// #161: put evaluates to the updated (root) dict on every path, so
		// (encode_json (put d k v)) sees a value instead of an empty stack.
		body := rnList(at, rnSym("do", at), build(t, args[1:len(args)-1]), rnSym(t, at))
		node.Children = rnLet(at, body, rnSym(t, at), args[0]).Children
	case "read_json":
		node.Children = []*Node{rnSym("parse_json", at), rnSym("Any", at), rnList(at, rnSym("read_text", at), args[0])}
	case "write_json":
		node.Children = []*Node{rnSym("write_file", at), args[0], rnList(at, rnSym("encode_json", at), args[1])}
	case "pluck":
		x, k := rnTemp("X", at), rnTemp("K", at)
		node.Children = rnLet(at,
			rnList(at, rnSym("map_list", at), rnSym(x, at), args[0], rnList(at, rnSym("map_get", at), rnSym(x, at), rnSym(k, at))),
			rnSym(k, at), args[1]).Children
	case "select":
		x := rnTemp("X", at)
		d := []*Node{rnSym("dict", at)}
		for _, a := range args[1:] {
			k, ok := rnKey(head, a)
			if !ok {
				return false
			}
			d = append(d, rnList(at, k, rnList(at, rnSym("map_get", at), rnSym(x, at), strNode(k.Value, at))))
		}
		node.Children = []*Node{rnSym("map_list", at), rnSym(x, at), args[0], rnList(at, d...)}
	case "group_by":
		if args[0].Type != "SYMBOL" {
			ReportError("group_by expects a variable name as argument 1", node.Line, node.Column)
			return false
		}
		r, k, g := rnTemp("R", at), rnTemp("K", at), rnTemp("G", at)
		x := args[0]
		inner := rnLet(at,
			rnList(at, rnSym("do", at),
				rnList(at, rnSym("append", at), rnSym(g, at), rnSym(x.Value, at)),
				rnList(at, rnSym("map_set", at), rnSym(r, at), rnSym(k, at), rnSym(g, at))),
			rnSym(k, at), rnList(at, rnSym("str", at), args[2]),
			rnSym(g, at), rnList(at, rnSym("nz", at), rnList(at, rnSym("map_get", at), rnSym(r, at), rnSym(k, at)), rnList(at, rnSym("list", at))))
		node.Children = rnLet(at,
			rnList(at, rnSym("do", at), rnList(at, rnSym("for", at), x, args[1], inner), rnSym(r, at)),
			rnSym(r, at), rnList(at, rnSym("dict", at))).Children
	case "merge":
		r := rnTemp("R", at)
		pairs := []*Node{rnSym(r, at), rnList(at, rnSym("dict", at))}
		body := []*Node{rnSym("do", at)}
		for _, a := range args {
			s, k := rnTemp("S", at), rnTemp("K", at)
			pairs = append(pairs, rnSym(s, at), a)
			body = append(body, rnList(at, rnSym("for", at), rnSym(k, at), rnList(at, rnSym("map_keys", at), rnSym(s, at)),
				rnList(at, rnSym("map_set", at), rnSym(r, at), rnSym(k, at), rnList(at, rnSym("map_get", at), rnSym(s, at), rnSym(k, at)))))
		}
		body = append(body, rnSym(r, at))
		node.Children = rnLet(at, rnList(at, body...), pairs...).Children
	default:
		return false
	}
	return true
}
