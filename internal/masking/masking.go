// Package masking compiles semantic checker types into provider-neutral
// constraints for token and logit masking.
package masking

import (
	"reflect"
	"sort"
	"strings"
	"zero/internal/ast"
	"zero/internal/checker"
)

// FormatV1 identifies the serialized mask plan schema.
const FormatV1 = "zero.mask_plan/v1"

var unconstrainedTokenClasses = []string{
	"null",
	"boolean",
	"integer",
	"number",
	"string",
	"array",
	"object",
}

// DelimiterPlan describes the structural tokens surrounding a collection or
// object value.
type DelimiterPlan struct {
	Open              string `json:"open"`
	Close             string `json:"close"`
	ItemSeparator     string `json:"item_separator"`
	KeyValueSeparator string `json:"key_value_separator,omitempty"`
}

// FieldPlan constrains one required field in a struct value.
type FieldPlan struct {
	Name       string   `json:"name"`
	Required   bool     `json:"required"`
	Constraint TypePlan `json:"constraint"`
}

// TypePlan describes the JSON-compatible token classes and structure accepted
// for one semantic Zero type. Token classes are symbolic so a downstream
// decoder can map them to provider-specific token IDs.
type TypePlan struct {
	Kind         ast.ValueKind  `json:"kind"`
	Name         string         `json:"name,omitempty"`
	TokenClasses []string       `json:"token_classes"`
	Literals     []string       `json:"literals,omitempty"`
	Encoding     string         `json:"encoding,omitempty"`
	Delimiters   *DelimiterPlan `json:"delimiters,omitempty"`
	Key          *TypePlan      `json:"key,omitempty"`
	Element      *TypePlan      `json:"element,omitempty"`
	Fields       []FieldPlan    `json:"fields,omitempty"`
}

// NamedTypePlan associates a checker struct name with its constraint.
type NamedTypePlan struct {
	Name       string   `json:"name"`
	Constraint TypePlan `json:"constraint"`
}

// ParamPlan constrains a function parameter by its stable signature index.
type ParamPlan struct {
	Index      int      `json:"index"`
	Constraint TypePlan `json:"constraint"`
}

// FunctionPlan describes the input and output constraints for a checked
// function signature.
type FunctionPlan struct {
	Name   string      `json:"name"`
	Params []ParamPlan `json:"params"`
	Return TypePlan    `json:"return"`
}

// ProgramPlan contains all named struct and function constraints collected by
// the semantic checker.
type ProgramPlan struct {
	Format    string          `json:"format"`
	Structs   []NamedTypePlan `json:"structs"`
	Functions []FunctionPlan  `json:"functions"`
}

// CompileType compiles one semantic type into a provider-neutral mask plan.
func CompileType(info ast.TypeInfo) TypePlan {
	return compileType(&info, make(map[*ast.TypeInfo]bool))
}

// CompileAnalysis compiles checker structs and function signatures into a
// deterministic program-level mask plan. A nil analysis produces an empty
// plan.
func CompileAnalysis(analysis *checker.Analysis) ProgramPlan {
	plan := ProgramPlan{
		Format:    FormatV1,
		Structs:   []NamedTypePlan{},
		Functions: []FunctionPlan{},
	}
	if analysis == nil {
		return plan
	}

	structNames := sortedKeys(analysis.Structs)
	for _, name := range structNames {
		plan.Structs = append(plan.Structs, NamedTypePlan{
			Name:       name,
			Constraint: CompileType(analysis.Structs[name]),
		})
	}

	functionNames := sortedKeys(analysis.Functions)
	for _, name := range functionNames {
		info := analysis.Functions[name]
		function := FunctionPlan{
			Name:   name,
			Params: make([]ParamPlan, 0, len(info.Params)),
			Return: CompileType(info.Return),
		}
		for index, param := range info.Params {
			function.Params = append(function.Params, ParamPlan{
				Index:      index,
				Constraint: CompileType(param),
			})
		}
		plan.Functions = append(plan.Functions, function)
	}

	return plan
}

func compileType(info *ast.TypeInfo, active map[*ast.TypeInfo]bool) TypePlan {
	if info == nil {
		unknown := ast.Layout(ast.Unknown)
		return compileType(&unknown, active)
	}
	if active[info] {
		return unknownPlan("unknown")
	}
	active[info] = true
	defer delete(active, info)

	kind := info.Kind
	if kind == "" {
		kind = ast.Unknown
	}
	plan := primitivePlan(kind, info.Name)

	switch kind {
	case ast.Bool:
		plan.TokenClasses = []string{"boolean"}
		plan.Literals = []string{"false", "true"}
	case ast.Int:
		plan.TokenClasses = []string{"integer"}
	case ast.Float:
		plan.TokenClasses = []string{"number"}
	case ast.String:
		plan.TokenClasses = []string{"string"}
	case ast.Bytes:
		plan.TokenClasses = []string{"string"}
		plan.Encoding = "bytes"
	case ast.Void:
		plan.TokenClasses = []string{"empty"}
	case ast.List:
		plan.TokenClasses = []string{"array"}
		plan.Delimiters = collectionDelimiters("[", "]")
		plan.Element = compileNested(info.Element, active)
	case ast.Dict:
		plan.TokenClasses = []string{"object"}
		plan.Delimiters = objectDelimiters()
		plan.Key = compileNested(info.Key, active)
		plan.Element = compileNested(info.Element, active)
	case ast.Struct:
		plan.TokenClasses = []string{"object"}
		plan.Delimiters = objectDelimiters()
		for _, name := range canonicalFieldNames(*info) {
			field := info.Fields[name]
			plan.Fields = append(plan.Fields, FieldPlan{
				Name:       name,
				Required:   true,
				Constraint: compileType(&field, active),
			})
		}
	case ast.Any, ast.Unknown:
		plan.TokenClasses = append([]string(nil), unconstrainedTokenClasses...)
	default:
		plan.Kind = ast.Unknown
		plan.TokenClasses = append([]string(nil), unconstrainedTokenClasses...)
	}

	return plan
}

func primitivePlan(kind ast.ValueKind, name string) TypePlan {
	if name == "" {
		name = string(kind)
	}
	return TypePlan{
		Kind:         kind,
		Name:         name,
		TokenClasses: []string{},
	}
}

func unknownPlan(name string) TypePlan {
	plan := primitivePlan(ast.Unknown, name)
	plan.TokenClasses = append([]string(nil), unconstrainedTokenClasses...)
	return plan
}

func compileNested(info *ast.TypeInfo, active map[*ast.TypeInfo]bool) *TypePlan {
	if info == nil {
		unknown := ast.Layout(ast.Unknown)
		plan := compileType(&unknown, active)
		return &plan
	}
	plan := compileType(info, active)
	return &plan
}

func collectionDelimiters(open, close string) *DelimiterPlan {
	return &DelimiterPlan{
		Open:          open,
		Close:         close,
		ItemSeparator: ",",
	}
}

func objectDelimiters() *DelimiterPlan {
	return &DelimiterPlan{
		Open:              "{",
		Close:             "}",
		ItemSeparator:     ",",
		KeyValueSeparator: ":",
	}
}

func canonicalFieldNames(info ast.TypeInfo) []string {
	names := sortedKeys(info.Fields)
	result := make([]string, 0, len(names))
	for _, name := range names {
		if isCheckerAlias(info, name) {
			continue
		}
		result = append(result, name)
	}
	return result
}

func isCheckerAlias(info ast.TypeInfo, name string) bool {
	if name == "" || name[:1] != strings.ToUpper(name[:1]) {
		return false
	}
	sourceName := strings.ToLower(name[:1]) + name[1:]
	sourceField, ok := info.Fields[sourceName]
	if !ok || !reflect.DeepEqual(info.Fields[name], sourceField) {
		return false
	}
	offset, hasOffset := info.FieldOffsets[name]
	sourceOffset, hasSourceOffset := info.FieldOffsets[sourceName]
	return hasOffset && hasSourceOffset && offset == sourceOffset
}

func sortedKeys[V any](values map[string]V) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
