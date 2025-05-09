package schema

import (
	"encoding/json"
	"errors"
	"testing"
)

// Common validation errors
var (
	ErrMissingRequiredField = errors.New("missing required field")
	ErrInvalidFieldType     = errors.New("invalid field type")
	ErrInvalidFieldValue    = errors.New("invalid field value")
	ErrPIIDetected          = errors.New("PII field detected without hashing")
)

// ValidationResult represents the result of a validation operation
type ValidationResult struct {
	Valid bool
	Error error
	Path  string
}

// SimpleValidator implements a simple schema validator for testing
type SimpleValidator struct {
	requiredFields []string
	piiFields      []string
	piiDetection   bool
}

// NewSimpleValidator creates a new simple validator
func NewSimpleValidator(requiredFields, piiFields []string, enablePIIDetection bool) *SimpleValidator {
	return &SimpleValidator{
		requiredFields: requiredFields,
		piiFields:      piiFields,
		piiDetection:   enablePIIDetection,
	}
}

// Validate validates the data against the schema
func (v *SimpleValidator) Validate(data interface{}) *ValidationResult {
	// Convert data to map if it's not already
	var dataMap map[string]interface{}
	
	switch d := data.(type) {
	case map[string]interface{}:
		dataMap = d
	default:
		// Try to convert from JSON
		jsonBytes, err := json.Marshal(data)
		if err != nil {
			return &ValidationResult{
				Valid: false,
				Error: ErrInvalidFieldType,
				Path:  "",
			}
		}

		if err := json.Unmarshal(jsonBytes, &dataMap); err != nil {
			return &ValidationResult{
				Valid: false,
				Error: ErrInvalidFieldType,
				Path:  "",
			}
		}
	}

	// Check required fields
	for _, field := range v.requiredFields {
		if _, exists := dataMap[field]; !exists {
			return &ValidationResult{
				Valid: false,
				Error: errors.New("missing required field: " + field),
				Path:  field,
			}
		}
	}

	// Check for PII fields if enabled
	if v.piiDetection {
		for _, piiField := range v.piiFields {
			if value, exists := dataMap[piiField]; exists {
				strValue, ok := value.(string)
				if !ok {
					continue
				}
				
				// Simple check for email format
				if isEmailFormat(strValue) && !isHashed(strValue) {
					return &ValidationResult{
						Valid: false,
						Error: ErrPIIDetected,
						Path:  piiField,
					}
				}
			}
		}
	}

	return &ValidationResult{Valid: true}
}

// isEmailFormat checks if a string looks like an email
func isEmailFormat(s string) bool {
	// Very simple check - contains @ and a dot after @
	atIndex := -1
	for i, c := range s {
		if c == '@' {
			atIndex = i
			break
		}
	}
	
	if atIndex == -1 || atIndex == 0 || atIndex == len(s)-1 {
		return false
	}
	
	hasDot := false
	for i := atIndex + 1; i < len(s); i++ {
		if s[i] == '.' {
			hasDot = true
			break
		}
	}
	
	return hasDot
}

// isHashed checks if a string looks like a hash
func isHashed(s string) bool {
	// Check if it's a hex string of appropriate length for a hash
	if len(s) == 64 {
		for _, c := range s {
			if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
				return false
			}
		}
		return true
	}
	return false
}

// TestSchemaValidator tests the schema validator implementation
func TestSchemaValidator(t *testing.T) {
	// Create validator that requires "resource_metrics" and checks for PII in "user.email"
	validator := NewSimpleValidator(
		[]string{"resource_metrics"},
		[]string{"user.email"},
		true,
	)
	
	// Test with missing required field
	invalidData := map[string]interface{}{
		"wrong_field": []interface{}{},
	}
	
	result := validator.Validate(invalidData)
	if result.Valid {
		t.Error("Validation should fail for missing required field")
	}
	
	// Test with valid data
	validData := map[string]interface{}{
		"resource_metrics": []interface{}{
			map[string]interface{}{
				"resource": map[string]interface{}{
					"attributes": map[string]interface{}{
						"service.name": "test-service",
					},
				},
			},
		},
	}
	
	result = validator.Validate(validData)
	if !result.Valid {
		t.Errorf("Validation should succeed for valid data, got error: %v", result.Error)
	}
	
	// Test with unhashed PII
	dataWithPII := map[string]interface{}{
		"resource_metrics": []interface{}{},
		"user.email": "test@example.com",
	}
	
	result = validator.Validate(dataWithPII)
	if result.Valid {
		t.Error("Validation should fail for unhashed PII")
	}
	if result.Error != ErrPIIDetected {
		t.Errorf("Expected PII error, got: %v", result.Error)
	}
	
	// Test with hashed PII
	dataWithHashedPII := map[string]interface{}{
		"resource_metrics": []interface{}{},
		"user.email": "a94a8fe5ccb19ba61c4c0873d391e987982fbbd3a94a8fe5ccb19ba61c4c0873",
	}
	
	result = validator.Validate(dataWithHashedPII)
	if !result.Valid {
		t.Errorf("Validation should succeed for hashed PII, got error: %v", result.Error)
	}
}
