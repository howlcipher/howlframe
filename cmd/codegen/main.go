package main

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"zero/internal/bytecode"
)

func main() {
	opcodes := make([]bytecode.OpcodeSpec, 0, len(bytecode.Registry))
	for _, spec := range bytecode.Registry {
		opcodes = append(opcodes, spec)
	}

	sort.Slice(opcodes, func(i, j int) bool {
		return opcodes[i].Name < opcodes[j].Name
	})

	generatePython(opcodes)
	generateMarkdown(opcodes)
}

func generatePython(opcodes []bytecode.OpcodeSpec) {
	var py strings.Builder

	py.WriteString(`from pydantic import BaseModel, Field
from typing import List, Dict, Union, Literal, Annotated, Any

`)

	var classNames []string

	for _, spec := range opcodes {
		className := snakeToCamel(spec.Name) + "Instruction"
		classNames = append(classNames, className)

		py.WriteString(fmt.Sprintf("class %s(BaseModel):\n", className))
		py.WriteString(fmt.Sprintf("    op: Literal[\"%s\"]\n", spec.Name))

		strIdx, intIdx := 1, 1
		for _, opType := range spec.Operands {
			if opType == bytecode.OperandString {
				fieldName := "string_operand"
				if strIdx > 1 {
					fieldName = fmt.Sprintf("string_operand_%d", strIdx)
				}
				py.WriteString(fmt.Sprintf("    %s: str\n", fieldName))
				strIdx++
			} else if opType == bytecode.OperandInt {
				fieldName := "int_operand"
				if intIdx > 1 {
					fieldName = fmt.Sprintf("int_operand_%d", intIdx)
				}
				py.WriteString(fmt.Sprintf("    %s: int\n", fieldName))
				intIdx++
			} else if opType == bytecode.OperandValue {
				py.WriteString("    value_operand: Any\n")
			}
		}

		if len(spec.Operands) == 0 {
			py.WriteString("    pass\n")
		}
		py.WriteString("\n")
	}

	py.WriteString("Instruction = Annotated[\n    Union[\n")
	for _, name := range classNames {
		py.WriteString(fmt.Sprintf("        %s,\n", name))
	}
	py.WriteString("    ],\n    Field(discriminator=\"op\")\n]\n\n")

	py.WriteString(`class Function(BaseModel):
    params: List[str]
    instructions: List[Instruction]

class BytecodeProgram(BaseModel):
    version: int
    functions: Dict[str, Function]
    main: List[Instruction]
`)

	os.WriteFile("orchestrator_schema.py", []byte(py.String()), 0644)
}

func generateMarkdown(opcodes []bytecode.OpcodeSpec) {
	var md strings.Builder

	md.WriteString("# Zero Bytecode Instruction Reference\n\n")
	md.WriteString("| Opcode | Operands | Pops | Pushes | Capability | Description |\n")
	md.WriteString("|---|---|---|---|---|---|\n")

	for _, spec := range opcodes {
		var ops []string
		for _, o := range spec.Operands {
			ops = append(ops, string(o))
		}
		opsStr := strings.Join(ops, ", ")

		popsStr := fmt.Sprintf("%d", spec.Pops)
		if spec.Pops == -1 {
			popsStr = "var"
		}

		md.WriteString(fmt.Sprintf("| `%s` | %s | %s | %d | %s | %s |\n",
			spec.Name, opsStr, popsStr, spec.Pushes, spec.Capability, spec.Description))
	}

	os.WriteFile("bytecode_reference.md", []byte(md.String()), 0644)
}

func snakeToCamel(s string) string {
	parts := strings.Split(s, "_")
	for i := range parts {
		parts[i] = strings.Title(strings.ToLower(parts[i]))
	}
	return strings.Join(parts, "")
}
