package convert

import (
	"fmt"

	convert "github.com/pb33f/libopenapi/convert/to31"
	"github.com/pb33f/libopenapi/convert/utils"

	"github.com/pb33f/libopenapi"
)

// DocumentConverter defines the interface for OpenAPI document converters
type DocumentConverter interface {
	// Convert performs the conversion and returns the converted document
	Convert() (*libopenapi.Document, error)
	// SupportsSource checks if this converter supports the source document
	SupportsSource() bool
}

// Converter handles OpenAPI document conversions
type Converter struct {
	document *libopenapi.Document
}

// NewConverter creates a new converter instance
func NewConverter(doc *libopenapi.Document) *Converter {
	return &Converter{document: doc}
}

// To31 converts the document to OpenAPI 3.1.0
func (c *Converter) To31() (*libopenapi.Document, error) {
	if c.document == nil {
		return nil, &utils.ConversionError{Message: "document is nil"}
	}

	converter := convert.NewTo31Converter(c.document)
	if !converter.SupportsSource() {
		version := (*c.document).GetVersion()
		return nil, &utils.ConversionError{
			Message: fmt.Sprintf("conversion from version %s to 3.1 not supported", version),
		}
	}

	return converter.Convert()
}
