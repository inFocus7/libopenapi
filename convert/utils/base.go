package utils

import (
	"fmt"

	"github.com/pb33f/libopenapi"
	v3 "github.com/pb33f/libopenapi/datamodel/high/v3"
	"gopkg.in/yaml.v3"
)

// BaseConverter provides common functionality for all converters
type BaseConverter struct {
	source                *libopenapi.Document
	supportedSourcesRegex []string
}

// NewBaseConverter creates a new base converter
func NewBaseConverter(doc *libopenapi.Document, supportedSources []string) *BaseConverter {
	return &BaseConverter{
		source:                doc,
		supportedSourcesRegex: supportedSources,
	}
}

// SupportsSource checks if the source document version is supported
func (c *BaseConverter) SupportsSource() bool {
	version := (*c.source).GetVersion()
	docVersion := GetOpenAPIVersion(version)

	for _, supportedVersion := range c.supportedSourcesRegex {
		if docVersion.String() == supportedVersion {
			return true
		}
	}

	return false
}

// GetSource returns the OpenAPI document
func (c *BaseConverter) GetSource() *libopenapi.Document {
	return c.source
}

// DeepCopyDocument creates a deep copy of the OpenAPI document
func (c *BaseConverter) DeepCopyDocument() (*v3.Document, error) {
	v3Model, errs := (*c.source).BuildV3Model()
	if len(errs) > 0 {
		return nil, fmt.Errorf("failed to build V3 model: %v", errs)
	}

	if &v3Model.Model == nil {
		return nil, fmt.Errorf("document is nil")
	}

	bytes, err := yaml.Marshal(&v3Model.Model)
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

// CreateNewDocument creates a new document from the given model
func (c *BaseConverter) CreateNewDocument(doc *v3.Document) (*libopenapi.Document, error) {
	bytes, err := yaml.Marshal(doc)
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

// GetSourceVersion returns the OpenAPI version of the source document
func (c *BaseConverter) GetSourceVersion() OpenAPIVersion {
	version := (*c.source).GetVersion()
	return GetOpenAPIVersion(version)
}
