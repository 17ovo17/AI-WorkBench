package engine

import "fmt"

type FieldDef struct {
	Type     string      `json:"type"`
	Required bool        `json:"required"`
	Default  interface{} `json:"default,omitempty"`
}

type NodeSchema struct {
	Inputs  map[string]FieldDef `json:"inputs"`
	Outputs map[string]FieldDef `json:"outputs"`
}

var nodeSchemas = map[string]NodeSchema{}

func RegisterNodeSchema(nodeType string, schema NodeSchema) {
	nodeSchemas[nodeType] = schema
}

func GetNodeSchema(nodeType string) (NodeSchema, bool) {
	s, ok := nodeSchemas[nodeType]
	return s, ok
}

func ValidateNodeInputs(nodeType string, inputs map[string]interface{}) error {
	schema, ok := nodeSchemas[nodeType]
	if !ok {
		return nil
	}
	for field, def := range schema.Inputs {
		val, exists := inputs[field]
		if def.Required && !exists {
			return fmt.Errorf("node %s: required input '%s' missing", nodeType, field)
		}
		if exists {
			if err := validateType(val, def.Type); err != nil {
				return fmt.Errorf("node %s input '%s': %w", nodeType, field, err)
			}
		}
	}
	return nil
}

func validateType(val interface{}, expected string) error {
	switch expected {
	case "string":
		if _, ok := val.(string); !ok {
			return fmt.Errorf("expected string, got %T", val)
		}
	case "number":
		switch val.(type) {
		case float64, int, int64, float32:
		default:
			return fmt.Errorf("expected number, got %T", val)
		}
	case "boolean":
		if _, ok := val.(bool); !ok {
			return fmt.Errorf("expected boolean, got %T", val)
		}
	}
	return nil
}

func init() {
	RegisterNodeSchema("knowledge_retrieval", NodeSchema{
		Inputs: map[string]FieldDef{"query": {Type: "string", Required: true}},
	})
	RegisterNodeSchema("http_request", NodeSchema{
		Inputs: map[string]FieldDef{"url": {Type: "string", Required: true}},
	})
	RegisterNodeSchema("llm", NodeSchema{
		Inputs: map[string]FieldDef{"user_prompt": {Type: "string", Required: true}},
	})
}
