package utils

import "regexp"

// OpenAPIVersion represents different OpenAPI specification versions
type OpenAPIVersion int

const (
	OpenAPIVersionUnknown OpenAPIVersion = iota
	OpenAPIVersion2                      // 2.0.x
	OpenAPIVersion3                      // 3.0.x
	OpenAPIVersion31                     // 3.1.x
)

var (
	// Version pattern matchers
	v2Pattern  = `^2\.0\.\d+$`
	v3Pattern  = `^3\.0\.\d+$`
	v31Pattern = `^3\.1\.\d+$`
)

// GetOpenAPIVersion determines the OpenAPI version from a version string
func GetOpenAPIVersion(version string) OpenAPIVersion {
	if matched, _ := regexp.MatchString(v2Pattern, version); matched {
		return OpenAPIVersion2
	}
	if matched, _ := regexp.MatchString(v3Pattern, version); matched {
		return OpenAPIVersion3
	}
	if matched, _ := regexp.MatchString(v31Pattern, version); matched {
		return OpenAPIVersion31
	}
	return OpenAPIVersionUnknown
}

// String returns the string representation of the OpenAPI version
func (v OpenAPIVersion) String() string {
	switch v {
	case OpenAPIVersion2:
		return "2.0.x"
	case OpenAPIVersion3:
		return "3.0.x"
	case OpenAPIVersion31:
		return "3.1.x"
	default:
		return "unknown"
	}
}
