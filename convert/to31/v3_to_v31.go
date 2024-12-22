package to31

import (
	"fmt"

	"github.com/pb33f/libopenapi/convert/utils"
	v3base "github.com/pb33f/libopenapi/datamodel/high/base"
	v3 "github.com/pb33f/libopenapi/datamodel/high/v3"
	"github.com/pb33f/libopenapi/orderedmap"
	"gopkg.in/yaml.v3"
)

// v3ToV31 converts from OpenAPI 3.0.x to 3.1.0
func v3ToV31(doc *v3.Document) error {
	// 1. Update OpenAPI version
	doc.Version = "3.1.0"

	// 2. Add JSON Schema dialect
	doc.JsonSchemaDialect = "https://spec.openapis.org/oas/3.1/dialect/base"

	// 3. Convert schemas
	if err := convertSchemas(doc); err != nil {
		return &utils.ConversionError{
			Message: "failed to convert schemas",
			Cause:   err,
		}
	}

	// 4. Handle webhooks
	doc.Webhooks = orderedmap.New[string, *v3.PathItem]()

	return nil
}

// convertSchemas converts all schemas in the document from 3.0 to 3.1 format
func convertSchemas(doc *v3.Document) error {
	// Convert component schemas
	if doc.Components != nil && doc.Components.Schemas != nil {
		for key, schemaProxy := range doc.Components.Schemas.FromOldest() {
			if err := convertSchema(schemaProxy); err != nil {
				return fmt.Errorf("failed to convert schema %s: %v", key, err)
			}
		}
	}

	// Convert path operation schemas
	if doc.Paths != nil {
		for _, pathItem := range doc.Paths.PathItems.FromOldest() {
			if err := convertPathItemSchemas(pathItem); err != nil {
				return err
			}
		}
	}

	return nil
}

// convertSchema converts a single schema from 3.0 to 3.1 format
func convertSchema(schemaProxy *v3base.SchemaProxy) error {
	if schemaProxy == nil {
		return nil
	}

	schema := schemaProxy.Schema()

	// Handle nullable property
	if schema.Nullable != nil && *schema.Nullable {
		schema.Type = append(schema.Type, "null")
		schema.Nullable = nil
	}

	// Convert example to examples array
	if schema.Example != nil {
		schema.Examples = []*yaml.Node{schema.Example}
		schema.Example = nil
	}

	// Convert exclusiveMinimum/Maximum
	if schema.ExclusiveMinimum != nil && schema.ExclusiveMinimum.IsA() {
		schema.ExclusiveMinimum.N = 1
		if schema.Minimum != nil {
			schema.ExclusiveMinimum.B = *schema.Minimum
		}
	}
	if schema.ExclusiveMaximum != nil && schema.ExclusiveMaximum.IsA() {
		schema.ExclusiveMaximum.N = 1
		if schema.Maximum != nil {
			schema.ExclusiveMaximum.B = *schema.Maximum
		}
	}

	// Handle file upload formats
	if len(schema.Type) == 1 && schema.Type[0] == "string" {
		switch schema.Format {
		case "base64", "byte":
			schema.ContentEncoding = "base64"
			schema.Format = ""
		case "binary":
			schema.ContentMediaType = "application/octet-stream"
			schema.Format = ""
		}
	}

	// Convert sub-schemas
	if schema.Properties != nil {
		for _, prop := range schema.Properties.FromOldest() {
			if err := convertSchema(prop); err != nil {
				return err
			}
		}
	}

	// Handle items
	if schema.Items != nil && schema.Items.IsA() {
		if err := convertSchema(schema.Items.A); err != nil {
			return err
		}
	}

	// Handle additional properties
	if schema.AdditionalProperties != nil && schema.AdditionalProperties.IsA() {
		if err := convertSchema(schema.AdditionalProperties.A); err != nil {
			return err
		}
	}

	return nil
}

// convertMediaType converts a media type object from 3.0 to 3.1 format
func convertMediaType(mediaType *v3.MediaType) error {
	if mediaType == nil {
		return nil
	}

	// For binary file upload in request body, remove schema entirely
	if mediaType.Schema != nil && mediaType.Schema.Schema().Format == "binary" {
		mediaType.Schema = nil
		return nil
	}

	// For other cases, convert the schema
	if mediaType.Schema != nil {
		if err := convertSchema(mediaType.Schema); err != nil {
			return err
		}
	}

	return nil
}

// convertPathItemSchemas converts schemas in a path item
func convertPathItemSchemas(pathItem *v3.PathItem) error {
	if pathItem == nil {
		return nil
	}

	operations := []*v3.Operation{
		pathItem.Get,
		pathItem.Put,
		pathItem.Post,
		pathItem.Delete,
		pathItem.Options,
		pathItem.Head,
		pathItem.Patch,
		pathItem.Trace,
	}

	for _, op := range operations {
		if op == nil {
			continue
		}

		// Convert request body schema
		if op.RequestBody != nil && op.RequestBody.Content != nil {
			for _, mediaType := range op.RequestBody.Content.FromOldest() {
				if err := convertMediaType(mediaType); err != nil {
					return err
				}
			}
		}

		// Convert response schemas
		if op.Responses != nil {
			// Convert default response
			if op.Responses.Default != nil && op.Responses.Default.Content != nil {
				for _, mediaType := range op.Responses.Default.Content.FromOldest() {
					if err := convertMediaType(mediaType); err != nil {
						return err
					}
				}
			}

			// Convert response codes
			for pair := op.Responses.Codes.First(); pair != nil; pair = pair.Next() {
				response := pair.Value()
				if response != nil && response.Content != nil {
					for _, mediaType := range response.Content.FromOldest() {
						if err := convertMediaType(mediaType); err != nil {
							return err
						}
					}
				}
			}
		}

		// Convert parameters
		if op.Parameters != nil {
			for _, param := range op.Parameters {
				if param.Schema != nil {
					if err := convertSchema(param.Schema); err != nil {
						return err
					}
				}
			}
		}
	}

	return nil
}
