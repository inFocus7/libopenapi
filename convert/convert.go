package convert

import (
	"fmt"
	"regexp"

	"github.com/pb33f/libopenapi"
	v3base "github.com/pb33f/libopenapi/datamodel/high/base"
	v3 "github.com/pb33f/libopenapi/datamodel/high/v3"
	"github.com/pb33f/libopenapi/orderedmap"
	"gopkg.in/yaml.v3"
)

// ConversionError represents an error that occurred during conversion
type ConversionError struct {
	Message string
	Cause   error
}

func (e *ConversionError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("%s: %v", e.Message, e.Cause)
	}
	return e.Message
}

// Converter handles OpenAPI document conversions
type Converter struct {
	document *libopenapi.Document
}

// NewConverter creates a new converter instance
func NewConverter(doc *libopenapi.Document) *Converter {
	return &Converter{document: doc}
}

// ConvertV3ToV31 converts an OpenAPI 3.0.x document to 3.1.0
// Based on https://www.openapis.org/blog/2021/02/16/migrating-from-openapi-3-0-to-3-1-0
func (c *Converter) ConvertV3ToV31() (*libopenapi.Document, error) {
	// Check if document is nil
	if c.document == nil {
		return nil, &ConversionError{Message: "document is nil"}
	}

	// Verify source document is 3.0.x
	version := (*c.document).GetVersion()
	if !isOpenAPI30x(version) {
		return nil, &ConversionError{
			Message: fmt.Sprintf("document version %s is not OpenAPI 3.0.x", version),
		}
	}

	// Build the V3 model
	v3Model, errs := (*c.document).BuildV3Model()
	if len(errs) > 0 {
		return nil, &ConversionError{
			Message: fmt.Sprintf("failed to build V3 model: %v", errs),
		}
	}

	// Create a deep copy of the document to avoid modifying the original
	docCopy, err := deepCopyDocument(&v3Model.Model)
	if err != nil {
		return nil, &ConversionError{
			Message: "failed to create document copy",
			Cause:   err,
		}
	}

	// Perform the conversion steps
	if err := convertToV31(docCopy); err != nil {
		return nil, err
	}

	bytes, err := yaml.Marshal(docCopy)
	if err != nil {
		return nil, &ConversionError{
			Message: "failed to marshal converted document",
			Cause:   err,
		}
	}

	newDoc, err := libopenapi.NewDocument(bytes)
	if err != nil {
		return nil, &ConversionError{
			Message: "failed to create new document",
			Cause:   err,
		}
	}

	return &newDoc, nil
}

// isOpenAPI30x checks if the version string matches 3.0.x pattern
func isOpenAPI30x(version string) bool {
	pattern := `^3\.0\.\d+`
	matched, _ := regexp.MatchString(pattern, version)
	return matched
}

// deepCopyDocument creates a deep copy of the OpenAPI document
func deepCopyDocument(doc *v3.Document) (*v3.Document, error) {
	if doc == nil {
		return nil, fmt.Errorf("document is nil")
	}

	bytes, err := yaml.Marshal(doc)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal document to bytes: %v", err)
	}

	newDoc, err := libopenapi.NewDocument(bytes)
	if err != nil {
		return nil, fmt.Errorf("failed to create new document: %v", err)
	}

	model, errs := newDoc.BuildV3Model()
	if len(errs) > 0 {
		return nil, fmt.Errorf("failed to build V3 model: %v", errs)
	}

	return &model.Model, nil
}

// convertToV31 performs the actual conversion from 3.0 to 3.1
func convertToV31(doc *v3.Document) error {
	// 1. Update OpenAPI version
	doc.Version = "3.1.0"

	// 2. Add JSON Schema dialect (required in 3.1)
	doc.JsonSchemaDialect = "https://spec.openapis.org/oas/3.1/dialect/base"

	// 3. Convert schemas (handle nullable, etc.)
	if err := convertSchemas(doc); err != nil {
		return &ConversionError{
			Message: "failed to convert schemas",
			Cause:   err,
		}
	}

	// 4. Handle webhooks (new in 3.1)
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

	// get the underlying schema
	schema := schemaProxy.Schema()

	// Handle nullable property (convert to type array with "null")
	if schema.Nullable != nil && *schema.Nullable == true {
		// TODO: Should we check if the schema already has "null" in the type array?
		schema.Type = append(schema.Type, "null")
		schema.Nullable = nil
	}

	// Convert example to examples array (3.1 change)
	if schema.Example != nil {
		schema.Examples = []*yaml.Node{schema.Example}
		schema.Example = nil
	}

	// Convert exclusiveMinimum/Maximum from boolean to numeric value
	if schema.ExclusiveMinimum != nil && schema.ExclusiveMinimum.IsA() {
		// change the N-bit to 1, which represents the 3.1 version
		schema.ExclusiveMinimum.N = 1
		if schema.Minimum != nil {
			schema.ExclusiveMinimum.B = *schema.Minimum
		}
	}
	if schema.ExclusiveMaximum != nil && schema.ExclusiveMaximum.IsA() {
		// change the N-bit to 1, which represents the 3.1 version
		schema.ExclusiveMaximum.N = 1
		if schema.Maximum != nil {
			schema.ExclusiveMaximum.B = *schema.Maximum
		}
	}

	// Handle file upload formats
	if len(schema.Type) == 1 && schema.Type[0] == "string" {
		switch schema.Format {
		case "base64", "byte":
			// Convert base64/byte format to contentEncoding
			schema.ContentEncoding = "base64"
			schema.Format = ""
		case "binary":
			// Only convert binary format to contentMediaType if it's a property
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

	// Handle items from the A value (3.0.x version)
	if schema.Items != nil && schema.Items.IsA() {
		if err := convertSchema(schema.Items.A); err != nil {
			return err
		}
	}

	// Handle additional properties from the A value (3.0.x version)
	if schema.AdditionalProperties != nil && schema.AdditionalProperties.IsA() {
		// Get the A value from the, which is the 3.0.x version
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

	// For any binary file upload in request body, remove schema entirely
	// This applies to both application/octet-stream and other media types
	// like image/png when they use format: binary
	if mediaType.Schema != nil && mediaType.Schema.Schema().Format == "binary" {
		mediaType.Schema = nil
		return nil
	}

	// For other cases (like base64 encoding or properties), convert the schema
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
			// Convert default response if present
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
