// Copyright 2022 Princess B33f Heavy Industries / Dave Shanley
// SPDX-License-Identifier: MIT

package base

import (
	"github.com/pb33f/libopenapi/datamodel/low"
)

// Discriminator is only used by OpenAPI 3+ documents, it represents a polymorphic discriminator used for schemas
//
// When request bodies or response payloads may be one of a number of different schemas, a discriminator object can be
// used to aid in serialization, deserialization, and validation. The discriminator is a specific object in a schema
// which is used to inform the consumer of the document of an alternative schema based on the value associated with it.
//
// When using the discriminator, inline schemas will not be considered.
//  v3 - https://spec.openapis.org/oas/v3.1.0#discriminator-object
type Discriminator struct {
	PropertyName low.NodeReference[string]
	Mapping      map[low.KeyReference[string]]low.ValueReference[string]
}

// FindMappingValue will return a ValueReference containing the string mapping value
func (d *Discriminator) FindMappingValue(key string) *low.ValueReference[string] {
	for k, v := range d.Mapping {
		if k.Value == key {
			return &v
		}
	}
	return nil
}
