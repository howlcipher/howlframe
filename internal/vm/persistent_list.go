package vm

import "fmt"

const (
	pvBits  = 4
	pvWidth = 1 << pvBits // 16
	pvMask  = pvWidth - 1 // 15
)

type pvNode struct {
	sub   []*pvNode
	items []any
}

// vmList is an immutable 16-way branching persistent vector with a small tail
// buffer (up to 16 elements). Appending copies only the tail (or one radix
// path on every 16th append), giving amortized O(1) time and O(n) retained
// memory while keeping lists immutable across shared variables and snapshots.
type vmList struct {
	length int
	shift  uint
	root   *pvNode
	tail   []any
}

var emptyVMList = &vmList{}

// newVMList constructs an immutable vmList from items and returns the count of
// pointer/value slice slots allocated across its backing chunks.
func newVMList(items []any) (*vmList, int) {
	if len(items) == 0 {
		return emptyVMList, 0
	}
	if len(items) <= pvWidth {
		tail := make([]any, len(items))
		copy(tail, items)
		return &vmList{length: len(items), tail: tail}, len(tail)
	}
	tailCount := ((len(items) - 1) & pvMask) + 1
	treeLen := len(items) - tailCount
	tail := make([]any, tailCount)
	copy(tail, items[treeLen:])
	allocSlots := len(tail)

	nodes := make([]*pvNode, 0, (treeLen+pvWidth-1)/pvWidth)
	for off := 0; off < treeLen; off += pvWidth {
		chunk := make([]any, pvWidth)
		copy(chunk, items[off:off+pvWidth])
		allocSlots += len(chunk)
		nodes = append(nodes, &pvNode{items: chunk})
	}

	var shift uint
	for len(nodes) > 1 {
		next := make([]*pvNode, 0, (len(nodes)+pvWidth-1)/pvWidth)
		for off := 0; off < len(nodes); off += pvWidth {
			end := off + pvWidth
			if end > len(nodes) {
				end = len(nodes)
			}
			sub := make([]*pvNode, end-off)
			copy(sub, nodes[off:end])
			allocSlots += len(sub)
			next = append(next, &pvNode{sub: sub})
		}
		nodes = next
		shift += pvBits
	}
	return &vmList{
		length: len(items),
		shift:  shift,
		root:   nodes[0],
		tail:   tail,
	}, allocSlots
}

func mustVMList(items []any) *vmList {
	l, _ := newVMList(items)
	return l
}

func (l *vmList) Len() int {
	if l == nil {
		return 0
	}
	return l.length
}

func (l *vmList) tailOffset() int {
	if l.length == 0 {
		return 0
	}
	return l.length - len(l.tail)
}

// At returns item i. Caller must ensure 0 <= i < l.Len().
func (l *vmList) At(i int) any {
	tailOff := l.tailOffset()
	if i >= tailOff {
		return l.tail[i-tailOff]
	}
	node := l.root
	for level := l.shift; level > 0; level -= pvBits {
		node = node.sub[(i>>level)&pvMask]
	}
	return node.items[i&pvMask]
}

// Append returns a new immutable vmList with item appended, leaving l
// unchanged, together with the number of slice slots newly allocated.
func (l *vmList) Append(item any) (*vmList, int) {
	if l == nil || l.length == 0 {
		return &vmList{length: 1, tail: []any{item}}, 1
	}
	if len(l.tail) < pvWidth {
		newTail := make([]any, len(l.tail)+1)
		copy(newTail, l.tail)
		newTail[len(l.tail)] = item
		return &vmList{
			length: l.length + 1,
			shift:  l.shift,
			root:   l.root,
			tail:   newTail,
		}, len(newTail)
	}
	newLeaf := &pvNode{items: l.tail}
	tailIdx := l.length - pvWidth
	newTail := []any{item}
	allocSlots := 1
	var newRoot *pvNode
	newShift := l.shift
	if (l.length >> pvBits) > (1 << l.shift) {
		rightBranch, rightSlots := pvNewPath(l.shift, newLeaf)
		newRoot = &pvNode{sub: []*pvNode{l.root, rightBranch}}
		newShift += pvBits
		allocSlots += 2 + rightSlots
	} else {
		var pathSlots int
		newRoot, pathSlots = pvPushTail(l.shift, l.root, newLeaf, tailIdx)
		allocSlots += pathSlots
	}
	return &vmList{
		length: l.length + 1,
		shift:  newShift,
		root:   newRoot,
		tail:   newTail,
	}, allocSlots
}

func pvNewPath(level uint, node *pvNode) (*pvNode, int) {
	if level == 0 {
		return node, 0
	}
	child, childSlots := pvNewPath(level-pvBits, node)
	return &pvNode{sub: []*pvNode{child}}, 1 + childSlots
}

func pvPushTail(level uint, parent, tailNode *pvNode, tailIdx int) (*pvNode, int) {
	if level == 0 {
		return tailNode, 0
	}
	subIdx := (tailIdx >> level) & pvMask
	prevLen := 0
	if parent != nil {
		prevLen = len(parent.sub)
	}
	newLen := prevLen
	if subIdx >= newLen {
		newLen = subIdx + 1
	}
	newSub := make([]*pvNode, newLen)
	if parent != nil {
		copy(newSub, parent.sub)
	}
	allocSlots := newLen
	if level == pvBits {
		newSub[subIdx] = tailNode
	} else {
		var child *pvNode
		if parent != nil && subIdx < len(parent.sub) {
			child = parent.sub[subIdx]
		}
		pushed, childSlots := pvPushTail(level-pvBits, child, tailNode, tailIdx)
		newSub[subIdx] = pushed
		allocSlots += childSlots
	}
	return &pvNode{sub: newSub}, allocSlots
}

// ForEach calls fn for each element in order until fn returns false.
func (l *vmList) ForEach(fn func(any) bool) bool {
	if l == nil || l.length == 0 {
		return true
	}
	if l.root != nil && !pvWalkNode(l.root, l.shift, fn) {
		return false
	}
	for _, item := range l.tail {
		if !fn(item) {
			return false
		}
	}
	return true
}

func pvWalkNode(node *pvNode, level uint, fn func(any) bool) bool {
	if level == 0 {
		for _, item := range node.items {
			if !fn(item) {
				return false
			}
		}
		return true
	}
	for _, child := range node.sub {
		if !pvWalkNode(child, level-pvBits, fn) {
			return false
		}
	}
	return true
}

// Slice returns a fresh []any copy of all items in l.
func (l *vmList) Slice() []any {
	if l == nil || l.length == 0 {
		return []any{}
	}
	out := make([]any, 0, l.length)
	l.ForEach(func(item any) bool {
		out = append(out, item)
		return true
	})
	return out
}

func (l *vmList) String() string {
	return fmt.Sprint(l.Slice())
}

// sliceFromList converts any list representation (*vmList or []any) to []any.
func sliceFromList(v any) ([]any, bool) {
	switch list := v.(type) {
	case *vmList:
		return list.Slice(), true
	case []any:
		return list, true
	default:
		return nil, false
	}
}

// vmTypeName reports the runtime value type label used in user-facing error
// messages, mapping persistent lists to Go's slice label ("[]interface {}") so
// runtime errors stay compatible with differential checks.
func vmTypeName(v any) string {
	if _, ok := v.(*vmList); ok {
		return "[]interface {}"
	}
	return fmt.Sprintf("%T", v)
}
