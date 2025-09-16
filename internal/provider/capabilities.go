package provider

import (
	"fmt"
)

// VariableCategory represents a category for grouping template variables
type VariableCategory struct {
	Name        string
	DisplayName string
	Description string
	Variables   []TemplateVariable
}

// ValidateCapabilities checks if provider capabilities are valid and consistent
func ValidateCapabilities(caps ProviderCapabilities) error {
	// Check for required fields
	if len(caps.MediaTypes) == 0 {
		return fmt.Errorf("provider must support at least one media type")
	}

	return nil
}
