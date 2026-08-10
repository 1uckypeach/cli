// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"go/format"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// typedInputConfigs is an explicit migration allowlist. The transformer is
// domain-wide, but generating a fragment does not claim that every Sheets
// shortcut is ready for the Typed runner. Add a command only together with its
// behavior-parity review and production migration.
var typedInputConfigs = map[string]typedInputConfig{
	"+batch-update": {
		FieldOverrides: map[string]typedFieldOverride{
			"operations": {FromType: "string", GoType: "json.RawMessage", Encoding: "json"},
		},
	},
	"+csv-put": {
		RequiredOverrides: map[string]typedRequiredOverride{
			// The canonical source marks --start-cell as both required and
			// defaulted. Production also accepts hidden --range as an alternative,
			// so the Typed field is optional/defaulted and a handwritten
			// exactly-one Relation preserves the actual invocation contract.
			"start-cell": {From: "required", To: "optional"},
		},
	},
}

type typedInputConfig struct {
	RequiredOverrides map[string]typedRequiredOverride
	FieldOverrides    map[string]typedFieldOverride
}

type typedFieldOverride struct {
	FromType string
	GoType   string
	Encoding string
}

type typedRequiredOverride struct {
	From string
	To   string
}

const typedInputsHeader = `// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

// Code generated from data/flag-defs.json; DO NOT EDIT.

package sheets

import (
	"encoding/json"

	"github.com/larksuite/cli/shortcuts/common"
)

// These fragments are generated only for shortcuts explicitly opted into a
// behavior-reviewed Typed migration. Relations, aliases, hidden compatibility
// metadata, hooks, authorization, Data, and Output remain handwritten.
`

func genTypedInputs(dir string) {
	raw, err := os.ReadFile(filepath.Join(dir, "data", "flag-defs.json"))
	if err != nil {
		log.Fatalf("gen typed inputs: %v", err)
	}
	var defs map[string]commandDef
	if err := json.Unmarshal(raw, &defs); err != nil {
		log.Fatalf("gen typed inputs: decode flag-defs.json: %v", err)
	}
	out, err := renderTypedInputs(defs, typedInputConfigs)
	if err != nil {
		log.Fatalf("gen typed inputs: %v", err)
	}
	path := filepath.Join(dir, "typed_inputs_gen.go")
	if err := os.WriteFile(path, out, 0o644); err != nil {
		log.Fatalf("gen typed inputs: write %s: %v", filepath.Base(path), err)
	}
	fmt.Printf("wrote %s (%d bytes)\n", filepath.Base(path), len(out))
}

func renderTypedInputs(defs map[string]commandDef, configs map[string]typedInputConfig) ([]byte, error) {
	commands := make([]string, 0, len(configs))
	for command := range configs {
		commands = append(commands, command)
	}
	sort.Strings(commands)

	var b bytes.Buffer
	b.WriteString(typedInputsHeader)
	seenTypeNames := make(map[string]string, len(commands))
	for _, command := range commands {
		definition, ok := defs[command]
		if !ok {
			return nil, fmt.Errorf("typed input command %q is absent from flag-defs.json", command)
		}
		config := configs[command]
		usedOverrides := make(map[string]struct{}, len(config.RequiredOverrides)+len(config.FieldOverrides))
		typeName := typedGoName(strings.TrimPrefix(command, "+")) + "GeneratedInput"
		if previous, duplicate := seenTypeNames[typeName]; duplicate {
			return nil, fmt.Errorf("typed input commands %q and %q generate duplicate type %s", previous, command, typeName)
		}
		seenTypeNames[typeName] = command
		fmt.Fprintf(&b, "\ntype %s struct {\n", typeName)
		businessFields := 0
		seenFlags := make(map[string]struct{})
		seenFields := make(map[string]string)
		for _, field := range definition.Flags {
			if field.Kind == "system" {
				continue
			}
			if field.Kind != "public" && field.Kind != "own" {
				return nil, fmt.Errorf("%s --%s has unsupported business flag kind %q", command, field.Name, field.Kind)
			}
			if _, duplicate := seenFlags[field.Name]; duplicate {
				return nil, fmt.Errorf("%s contains duplicate business flag --%s", command, field.Name)
			}
			seenFlags[field.Name] = struct{}{}
			goField := typedGoName(field.Name)
			if previous, duplicate := seenFields[goField]; duplicate {
				return nil, fmt.Errorf("%s flags --%s and --%s generate duplicate Go field %s", command, previous, field.Name, goField)
			}
			seenFields[goField] = field.Name
			businessFields++
			required := field.Required
			if override, exists := config.RequiredOverrides[field.Name]; exists {
				if required != override.From {
					return nil, fmt.Errorf("%s --%s required override expected %q, got %q", command, field.Name, override.From, required)
				}
				required = override.To
				usedOverrides[field.Name] = struct{}{}
			}
			line, err := renderTypedInputField(command, field, required, config.FieldOverrides[field.Name])
			if err != nil {
				return nil, err
			}
			b.WriteString(line)
		}
		if businessFields == 0 {
			return nil, fmt.Errorf("%s has no business input fields", command)
		}
		for name := range config.RequiredOverrides {
			if _, used := usedOverrides[name]; !used {
				return nil, fmt.Errorf("%s required override references unknown or system flag --%s", command, name)
			}
		}
		for name := range config.FieldOverrides {
			if _, seen := seenFlags[name]; !seen {
				return nil, fmt.Errorf("%s field override references unknown or system flag --%s", command, name)
			}
		}
		b.WriteString("}\n")
	}
	out, err := format.Source(b.Bytes())
	if err != nil {
		return nil, fmt.Errorf("format typed_inputs_gen.go: %w", err)
	}
	return out, nil
}

func renderTypedInputField(command string, field flagDef, required string, override typedFieldOverride) (string, error) {
	if field.Name == "" {
		return "", fmt.Errorf("%s has a business flag with no name", command)
	}
	goType, encoding, err := typedGoType(field.Type)
	if err != nil {
		return "", fmt.Errorf("%s --%s: %w", command, field.Name, err)
	}
	if override.GoType != "" {
		if field.Type != override.FromType {
			return "", fmt.Errorf("%s --%s field override expected type %q, got %q", command, field.Name, override.FromType, field.Type)
		}
		goType, encoding = override.GoType, override.Encoding
	}
	if required != "required" && required != "optional" && required != "xor" {
		return "", fmt.Errorf("%s --%s has unsupported required value %q", command, field.Name, required)
	}
	schemaRequired := required
	if schemaRequired == "xor" {
		// The canonical source does not identify XOR group membership. The
		// complete Relation and its presence/stage semantics remain handwritten.
		schemaRequired = "optional"
	}
	if field.Default != "" && schemaRequired == "required" {
		return "", fmt.Errorf("%s --%s is required and defaulted; add an explicit reviewed override", command, field.Name)
	}

	schemaTokens := []string{schemaRequired}
	if field.Default != "" {
		literal, err := typedDefaultLiteral(field.Type, field.Default)
		if err != nil {
			return "", fmt.Errorf("%s --%s default: %w", command, field.Name, err)
		}
		schemaTokens = append(schemaTokens, "default="+literal)
	}
	if len(field.Enum) > 0 {
		if strings.HasSuffix(field.Type, "_slice") || strings.HasSuffix(field.Type, "_array") {
			return "", fmt.Errorf("%s --%s has collection enum vocabulary; its completion-only legacy semantics need a reviewed Typed contract", command, field.Name)
		}
		for _, value := range field.Enum {
			if strings.ContainsAny(value, "|;=") {
				return "", fmt.Errorf("%s --%s enum value %q cannot be represented by the Typed schema tag", command, field.Name, value)
			}
		}
		schemaTokens = append(schemaTokens, "enum="+strings.Join(field.Enum, "|"))
	}

	cliTokens := make([]string, 0, 2)
	if len(field.Input) > 0 {
		sources := []string{"flag"}
		for _, source := range field.Input {
			if source != "file" && source != "stdin" {
				return "", fmt.Errorf("%s --%s has unsupported input source %q", command, field.Name, source)
			}
			sources = append(sources, source)
		}
		cliTokens = append(cliTokens, "sources="+strings.Join(sources, "|"))
	}
	if encoding != "" {
		cliTokens = append(cliTokens, "encoding="+encoding)
	}

	// Optional and xor fields retain explicit presence for future hooks that
	// replace RuntimeContext.Changed. Required fields are necessarily present
	// before Typed hooks run and do not need the wrapper.
	if schemaRequired != "required" {
		goType = "common.Provided[" + goType + "]"
	}
	tagParts := []string{
		"flag:" + strconv.Quote(field.Name),
		"schema:" + strconv.Quote(strings.Join(schemaTokens, ";")),
	}
	if len(cliTokens) > 0 {
		tagParts = append(tagParts, "cli:"+strconv.Quote(strings.Join(cliTokens, ";")))
	}
	tagParts = append(tagParts, "doc:"+strconv.Quote(field.Desc))
	return fmt.Sprintf("\t%s %s %s\n", typedGoName(field.Name), goType, strconv.Quote(strings.Join(tagParts, " "))), nil
}

func typedGoType(flagType string) (goType, encoding string, err error) {
	switch flagType {
	case "", "string":
		return "string", "", nil
	case "bool":
		return "bool", "", nil
	case "int":
		return "int", "", nil
	case "float64":
		return "float64", "", nil
	case "int_array":
		return "[]int", "comma_or_repeated", nil
	case "string_array":
		return "[]string", "repeated", nil
	case "string_slice":
		return "[]string", "comma_or_repeated", nil
	default:
		return "", "", fmt.Errorf("unsupported flag type %q", flagType)
	}
}

func typedDefaultLiteral(flagType, value string) (string, error) {
	var decoded any
	switch flagType {
	case "", "string":
		decoded = value
	case "bool":
		parsed, err := strconv.ParseBool(value)
		if err != nil {
			return "", err
		}
		decoded = parsed
	case "int":
		parsed, err := strconv.Atoi(value)
		if err != nil {
			return "", err
		}
		decoded = parsed
	case "float64":
		parsed, err := strconv.ParseFloat(value, 64)
		if err != nil {
			return "", err
		}
		decoded = parsed
	case "int_array", "string_array", "string_slice":
		if err := json.Unmarshal([]byte(value), &decoded); err != nil {
			return "", err
		}
	default:
		return "", fmt.Errorf("unsupported flag type %q", flagType)
	}
	encoded, err := json.Marshal(decoded)
	if err != nil {
		return "", err
	}
	return string(encoded), nil
}

var typedInitialisms = map[string]string{
	"csv": "CSV",
	"id":  "ID",
	"ids": "IDs",
	"uri": "URI",
	"url": "URL",
}

func typedGoName(name string) string {
	parts := strings.FieldsFunc(name, func(r rune) bool { return r == '-' || r == '_' })
	var result strings.Builder
	for _, part := range parts {
		if initialism, ok := typedInitialisms[strings.ToLower(part)]; ok {
			result.WriteString(initialism)
			continue
		}
		result.WriteString(strings.ToUpper(part[:1]))
		result.WriteString(part[1:])
	}
	return result.String()
}
