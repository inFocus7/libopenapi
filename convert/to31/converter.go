package to31

import (
	"github.com/pb33f/libopenapi"
	"github.com/pb33f/libopenapi/convert/utils"
)

// Converter handles conversion to OpenAPI 3.1.0
type Converter struct {
	*utils.BaseConverter
}

// NewTo31Converter creates a new Converter instance
func NewTo31Converter(doc *libopenapi.Document) *Converter {
	supportedSources := []string{utils.OpenAPIVersion3.String()}
	return &Converter{
		BaseConverter: utils.NewBaseConverter(doc, supportedSources),
	}
}

// Convert implements DocumentConverter
func (c *Converter) Convert() (*libopenapi.Document, error) {
	docCopy, err := c.DeepCopyDocument()
	if err != nil {
		return nil, &utils.ConversionError{
			Message: "failed to create document copy",
			Cause:   err,
		}
	}

	// Based on the source version, choose the appropriate converter (we only support 3.0)
	switch c.GetSourceVersion() {
	case utils.OpenAPIVersion3:
		if err := v3ToV31(docCopy); err != nil {
			return nil, err
		}
	default:
		return nil, &utils.ConversionError{
			Message: "unsupported source version for 3.1 conversion",
		}
	}

	return c.CreateNewDocument(docCopy)
}
