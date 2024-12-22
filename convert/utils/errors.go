package utils

import "fmt"

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
