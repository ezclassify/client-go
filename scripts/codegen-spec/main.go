// Command codegen-spec creates an OpenAPI 3.0 codegen view of the canonical
// OpenAPI 3.1 document. The checked-in spec and its digest are never changed.
package main

import (
	"fmt"
	"os"

	"go.yaml.in/yaml/v3"
)

func main() {
	if len(os.Args) != 3 {
		fmt.Fprintln(os.Stderr, "usage: codegen-spec INPUT OUTPUT")
		os.Exit(2)
	}
	input, err := os.ReadFile(os.Args[1])
	if err != nil {
		fatal(err)
	}
	var document yaml.Node
	if err := yaml.Unmarshal(input, &document); err != nil {
		fatal(err)
	}
	if err := normalizeDocument(&document); err != nil {
		fatal(err)
	}
	output, err := os.Create(os.Args[2])
	if err != nil {
		fatal(err)
	}
	encoder := yaml.NewEncoder(output)
	encoder.SetIndent(2)
	if err := encoder.Encode(&document); err != nil {
		fatal(err)
	}
	if err := encoder.Close(); err != nil {
		fatal(err)
	}
	if err := output.Close(); err != nil {
		fatal(err)
	}
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "codegen-spec:", err)
	os.Exit(1)
}

func normalizeDocument(document *yaml.Node) error {
	if document.Kind != yaml.DocumentNode || len(document.Content) != 1 {
		return fmt.Errorf("expected one OpenAPI document")
	}
	root := document.Content[0]
	version := field(root, "openapi")
	if version == nil || version.Value != "3.1.0" {
		return fmt.Errorf("expected canonical OpenAPI 3.1.0 document")
	}
	version.Value = "3.0.3"
	schemas := field(field(root, "components"), "schemas")
	if schemas == nil || schemas.Kind != yaml.MappingNode {
		return fmt.Errorf("expected components.schemas")
	}
	for i := 0; i < len(schemas.Content); i += 2 {
		name := schemas.Content[i].Value
		if err := normalizeSchemaTree(schemas.Content[i+1], "components/schemas/"+name); err != nil {
			return err
		}
	}
	return nil
}

// Descend only through Schema Object members. Literal examples, defaults,
// consts, enum values and extensions can themselves contain a "type" key;
// those are data and must never be interpreted as schema declarations.
func normalizeSchemaTree(node *yaml.Node, path string) error {
	if node.Kind != yaml.MappingNode {
		return fmt.Errorf("%s: expected Schema Object", path)
	}
	if err := normalizeSchema(node, path); err != nil {
		return err
	}
	for _, name := range []string{"properties", "patternProperties", "$defs", "definitions", "dependentSchemas"} {
		children := field(node, name)
		if children == nil {
			continue
		}
		if children.Kind != yaml.MappingNode {
			return fmt.Errorf("%s/%s: expected schema map", path, name)
		}
		for i := 0; i < len(children.Content); i += 2 {
			key := children.Content[i].Value
			if err := normalizeSchemaTree(children.Content[i+1], path+"/"+name+"/"+key); err != nil {
				return err
			}
		}
	}
	for _, name := range []string{"items", "additionalProperties", "not", "if", "then", "else", "contains", "propertyNames", "unevaluatedProperties"} {
		child := field(node, name)
		if child != nil && child.Kind == yaml.MappingNode {
			if err := normalizeSchemaTree(child, path+"/"+name); err != nil {
				return err
			}
		}
	}
	for _, name := range []string{"allOf", "anyOf", "oneOf", "prefixItems"} {
		children := field(node, name)
		if children == nil {
			continue
		}
		if children.Kind != yaml.SequenceNode {
			return fmt.Errorf("%s/%s: expected schema list", path, name)
		}
		for i, child := range children.Content {
			if err := normalizeSchemaTree(child, fmt.Sprintf("%s/%s/%d", path, name, i)); err != nil {
				return err
			}
		}
	}
	return nil
}

func normalizeSchema(node *yaml.Node, path string) error {
	if node.Kind != yaml.MappingNode {
		return nil
	}
	if typ := field(node, "type"); typ != nil && typ.Kind == yaml.SequenceNode {
		if len(typ.Content) != 2 {
			return fmt.Errorf("%s: unsupported type union", path)
		}
		first, second := typ.Content[0], typ.Content[1]
		if first.Kind != yaml.ScalarNode || second.Kind != yaml.ScalarNode {
			return fmt.Errorf("%s: unsupported type union", path)
		}
		if first.Value == "null" || second.Value == "null" {
			if field(node, "nullable") != nil {
				return fmt.Errorf("%s: duplicate nullable declaration", path)
			}
			other := first
			if first.Value == "null" {
				other = second
			}
			if other.Value == "null" {
				return fmt.Errorf("%s: null-only type union", path)
			}
			typ.Kind, typ.Tag, typ.Value, typ.Content = yaml.ScalarNode, "!!str", other.Value, nil
			appendField(node, "nullable", &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!bool", Value: "true"})
		} else if (first.Value == "string" && second.Value == "number") || (first.Value == "number" && second.Value == "string") {
			if field(node, "oneOf") != nil || field(node, "anyOf") != nil || field(node, "allOf") != nil {
				return fmt.Errorf("%s: primitive type union conflicts with composition", path)
			}
			// JSON string and number are disjoint, so oneOf is equivalent to the
			// OAS 3.1 primitive type union and supported by pinned oapi-codegen.
			removeField(node, "type")
			appendField(node, "oneOf", &yaml.Node{Kind: yaml.SequenceNode, Content: []*yaml.Node{
				primitive(first.Value), primitive(second.Value),
			}})
		} else {
			return fmt.Errorf("%s: unsupported type union", path)
		}
	}
	union := field(node, "anyOf")
	if union == nil || union.Kind != yaml.SequenceNode {
		return nil
	}
	var nonNull *yaml.Node
	nullCount := 0
	for _, arm := range union.Content {
		if arm.Kind == yaml.MappingNode && field(arm, "type") != nil && field(arm, "type").Value == "null" {
			if len(arm.Content) != 2 {
				return fmt.Errorf("%s: null arm has extra constraints", path)
			}
			nullCount++
		} else {
			nonNull = arm
		}
	}
	if nullCount == 0 {
		return nil
	}
	if nullCount != 1 || len(union.Content) != 2 || nonNull == nil || nonNull.Kind != yaml.MappingNode || field(node, "nullable") != nil || field(node, "type") != nil {
		return fmt.Errorf("%s: unsupported nullable anyOf", path)
	}
	removeField(node, "anyOf")
	if field(nonNull, "$ref") != nil {
		if len(nonNull.Content) != 2 || field(node, "allOf") != nil {
			return fmt.Errorf("%s: unsupported nullable reference", path)
		}
		appendField(node, "allOf", &yaml.Node{Kind: yaml.SequenceNode, Content: []*yaml.Node{nonNull}})
	} else {
		for i := 0; i < len(nonNull.Content); i += 2 {
			key := nonNull.Content[i].Value
			if field(node, key) != nil {
				return fmt.Errorf("%s: duplicate nullable schema field %q", path, key)
			}
			appendField(node, key, nonNull.Content[i+1])
		}
	}
	appendField(node, "nullable", &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!bool", Value: "true"})
	return nil
}

func primitive(typ string) *yaml.Node {
	node := &yaml.Node{Kind: yaml.MappingNode}
	appendField(node, "type", &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: typ})
	return node
}

func field(node *yaml.Node, name string) *yaml.Node {
	if node == nil || node.Kind != yaml.MappingNode {
		return nil
	}
	for i := 0; i < len(node.Content); i += 2 {
		if node.Content[i].Value == name {
			return node.Content[i+1]
		}
	}
	return nil
}

func appendField(node *yaml.Node, name string, value *yaml.Node) {
	node.Content = append(node.Content, &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: name}, value)
}

func removeField(node *yaml.Node, name string) {
	for i := 0; i < len(node.Content); i += 2 {
		if node.Content[i].Value == name {
			node.Content = append(node.Content[:i], node.Content[i+2:]...)
			return
		}
	}
}
