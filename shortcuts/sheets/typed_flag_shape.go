// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

//nolint:forbidigo // Schema adaptation failures are registration-time programmer errors.
package sheets

import (
	"encoding/json"
	"fmt"
	"sort"

	"github.com/larksuite/cli/shortcuts/common"
)

type typedFlagJSONSchema struct {
	Type        string                          `json:"type"`
	Description string                          `json:"description"`
	Enum        []common.JSONValue              `json:"enum"`
	Required    []string                        `json:"required"`
	Properties  map[string]*typedFlagJSONSchema `json:"properties"`
	Items       *typedFlagJSONSchema            `json:"items"`
	OneOf       []*typedFlagJSONSchema          `json:"oneOf"`
	Minimum     *float64                        `json:"minimum"`
	Maximum     *float64                        `json:"maximum"`
	MinLength   *int                            `json:"minLength"`
	MaxLength   *int                            `json:"maxLength"`
	MinItems    *int                            `json:"minItems"`
	MaxItems    *int                            `json:"maxItems"`
}

// batchOperationsInputShape projects the canonical Sheets flag schema into the
// Typed contract, then adds the legacy envelope accepted by +batch-update.
// Keeping the canonical array as the source prevents the Typed help/schema from
// drifting from --print-schema as batchable shortcuts evolve.
func batchOperationsInputShape() common.ValueShape {
	index, err := loadFlagSchemas()
	if err != nil {
		panic(fmt.Sprintf("load Sheets flag schema for +batch-update --operations: %v", err))
	}
	raw := index.Flags["+batch-update"]["operations"]
	if len(raw) == 0 {
		panic("missing Sheets flag schema for +batch-update --operations")
	}
	var schema typedFlagJSONSchema
	if err := json.Unmarshal(raw, &schema); err != nil {
		panic(fmt.Sprintf("decode Sheets flag schema for +batch-update --operations: %v", err))
	}
	operations, err := commonShapeFromFlagSchema(&schema)
	if err != nil {
		panic(fmt.Sprintf("compile Sheets flag schema for +batch-update --operations: %v", err))
	}
	if _, ok := operations.(common.ArrayShape); !ok {
		panic(fmt.Sprintf("Sheets flag schema for +batch-update --operations is %T, want array", operations))
	}
	envelope := common.ObjectShape{AdditionalProperties: true, Fields: []common.ValueField{
		{Name: "continue_on_error", Description: "continue after failed sub-operations", Shape: common.BooleanShape{}},
		{Name: "operations", Description: schema.Description, Required: true, Shape: operations},
	}}

	// The domain translator intentionally owns semantic validation so it can
	// aggregate multiple bad sub-operations and return prescriptive errors. Keep
	// permissive fallbacks after the canonical variants: the internal schema
	// still projects the authoritative shape first, while malformed JSON values
	// reach the existing translator instead of being replaced by a generic
	// framework oneOf error.
	looseScalar := []common.ValueShape{common.StringShape{}, common.NumberShape{}, common.BooleanShape{}, common.NullShape{}, common.ObjectShape{AdditionalProperties: true}}
	looseItem := common.OneOfShape{Variants: append(append([]common.ValueShape(nil), looseScalar...), common.ArrayShape{Items: common.OneOfShape{Variants: looseScalar}})}
	looseArray := common.ArrayShape{Items: looseItem}
	variants := []common.ValueShape{operations, envelope, looseArray, common.ObjectShape{AdditionalProperties: true}}
	variants = append(variants, looseScalar[:4]...)
	return common.OneOfShape{Variants: variants}
}

func commonShapeFromFlagSchema(schema *typedFlagJSONSchema) (common.ValueShape, error) {
	if schema == nil {
		return nil, fmt.Errorf("schema is nil")
	}
	if len(schema.OneOf) > 0 {
		variants := make([]common.ValueShape, 0, len(schema.OneOf))
		for index, variant := range schema.OneOf {
			shape, err := commonShapeFromFlagSchema(variant)
			if err != nil {
				return nil, fmt.Errorf("oneOf[%d]: %w", index, err)
			}
			variants = append(variants, shape)
		}
		return common.OneOfShape{Variants: variants}, nil
	}
	switch schema.Type {
	case "string":
		shape := common.StringShape{MinLength: schema.MinLength, MaxLength: schema.MaxLength}
		for index, value := range schema.Enum {
			item, ok := value.(string)
			if !ok {
				return nil, fmt.Errorf("string enum[%d] has type %T", index, value)
			}
			shape.Enum = append(shape.Enum, item)
		}
		return shape, nil
	case "boolean":
		return common.BooleanShape{}, nil
	case "integer":
		shape := common.IntegerShape{}
		if schema.Minimum != nil {
			minimum := int64(*schema.Minimum)
			if float64(minimum) != *schema.Minimum {
				return nil, fmt.Errorf("integer minimum %v is not integral", *schema.Minimum)
			}
			shape.Minimum = &minimum
		}
		if schema.Maximum != nil {
			maximum := int64(*schema.Maximum)
			if float64(maximum) != *schema.Maximum {
				return nil, fmt.Errorf("integer maximum %v is not integral", *schema.Maximum)
			}
			shape.Maximum = &maximum
		}
		return shape, nil
	case "number":
		return common.NumberShape{Minimum: schema.Minimum, Maximum: schema.Maximum}, nil
	case "array":
		item, err := commonShapeFromFlagSchema(schema.Items)
		if err != nil {
			return nil, fmt.Errorf("items: %w", err)
		}
		return common.ArrayShape{Items: item, MinItems: schema.MinItems, MaxItems: schema.MaxItems}, nil
	case "object":
		required := make(map[string]struct{}, len(schema.Required))
		for _, name := range schema.Required {
			required[name] = struct{}{}
		}
		names := make([]string, 0, len(schema.Properties))
		for name := range schema.Properties {
			names = append(names, name)
		}
		sort.Strings(names)
		shape := common.ObjectShape{AdditionalProperties: true}
		for _, name := range names {
			property := schema.Properties[name]
			child, err := commonShapeFromFlagSchema(property)
			if err != nil {
				return nil, fmt.Errorf("property %q: %w", name, err)
			}
			_, isRequired := required[name]
			shape.Fields = append(shape.Fields, common.ValueField{Name: name, Description: property.Description, Required: isRequired, Shape: child})
		}
		return shape, nil
	case "null":
		return common.NullShape{}, nil
	default:
		return nil, fmt.Errorf("unsupported JSON Schema type %q", schema.Type)
	}
}
