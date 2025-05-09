package schema

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"regexp"
	"strings"
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

// NewValidationError creates a new validation error with the specified field path
func NewValidationError(err error, path string) *ValidationResult {
	return &ValidationResult{
		Valid: false,
		Error: err,
		Path:  path,
	}
}

// SchemaValidator defines an interface for schema validation
type SchemaValidator interface {
	// Validate validates the data against the schema
	Validate(data interface{}) *ValidationResult
}

// JSONSchemaValidator implements schema validation using JSON Schema
type JSONSchemaValidator struct {
	schema       map[string]interface{}
	piiFields    []string
	piiDetection bool
}

// NewJSONSchemaValidator creates a new JSON Schema validator
func NewJSONSchemaValidator(schemaJSON string, piiFields []string, enablePIIDetection bool) (*JSONSchemaValidator, error) {
	var schema map[string]interface{}
	if err := json.Unmarshal([]byte(schemaJSON), &schema); err != nil {
		return nil, fmt.Errorf("invalid schema JSON: %w", err)
	}

	return &JSONSchemaValidator{
		schema:       schema,
		piiFields:    piiFields,
		piiDetection: enablePIIDetection,
	}, nil
}

// Validate validates the data against the schema
func (v *JSONSchemaValidator) Validate(data interface{}) *ValidationResult {
	return v.validateObject(data, v.schema, "")
}

// validateObject validates an object against the schema
func (v *JSONSchemaValidator) validateObject(data interface{}, schema map[string]interface{}, path string) *ValidationResult {
	// Convert data to map if it's not already
	dataMap, ok := data.(map[string]interface{})
	if !ok {
		// Try to convert from JSON
		jsonBytes, err := json.Marshal(data)
		if err != nil {
			return NewValidationError(ErrInvalidFieldType, path)
		}

		if err := json.Unmarshal(jsonBytes, &dataMap); err != nil {
			return NewValidationError(ErrInvalidFieldType, path)
		}
	}

	// Check required fields
	if required, ok := schema["required"].([]interface{}); ok {
		for _, req := range required {
			reqField, ok := req.(string)
			if !ok {
				continue
			}

			if _, exists := dataMap[reqField]; !exists {
				fieldPath := path
				if path != "" {
					fieldPath += "."
				}
				fieldPath += reqField
				return NewValidationError(fmt.Errorf("%w: %s", ErrMissingRequiredField, reqField), fieldPath)
			}
		}
	}

	// Check properties
	if properties, ok := schema["properties"].(map[string]interface{}); ok {
		for fieldName, fieldSchema := range properties {
			fieldSchemaMap, ok := fieldSchema.(map[string]interface{})
			if !ok {
				continue
			}

			fieldValue, exists := dataMap[fieldName]
			if !exists {
				continue
			}

			fieldPath := path
			if path != "" {
				fieldPath += "."
			}
			fieldPath += fieldName

			// Validate field type
			if err := v.validateType(fieldValue, fieldSchemaMap, fieldPath); err != nil {
				return err
			}

			// Validate field format
			if err := v.validateFormat(fieldValue, fieldSchemaMap, fieldPath); err != nil {
				return err
			}

			// Recursive validation for objects
			if fieldType, ok := fieldSchemaMap["type"].(string); ok && fieldType == "object" {
				if fieldObjectSchema, ok := fieldSchemaMap["properties"].(map[string]interface{}); ok {
					if result := v.validateObject(fieldValue, map[string]interface{}{
						"properties": fieldObjectSchema,
						"required":   fieldSchemaMap["required"],
					}, fieldPath); !result.Valid {
						return result
					}
				}
			}

			// Validate arrays
			if fieldType, ok := fieldSchemaMap["type"].(string); ok && fieldType == "array" {
				if items, ok := fieldSchemaMap["items"].(map[string]interface{}); ok {
					if result := v.validateArray(fieldValue, items, fieldPath); !result.Valid {
						return result
					}
				}
			}
		}
	}

	// Check for PII fields if enabled
	if v.piiDetection {
		for _, piiField := range v.piiFields {
			parts := strings.Split(piiField, ".")
			lastPart := parts[len(parts)-1]
			
			// Check if the field exists and is not hashed
			for field, value := range dataMap {
				// Direct match
				if field == lastPart {
					if !v.isHashedOrEncoded(value) {
						fieldPath := path
						if path != "" {
							fieldPath += "."
						}
						fieldPath += field
						return NewValidationError(fmt.Errorf("%w: %s", ErrPIIDetected, field), fieldPath)
					}
				}
			}
		}
	}

	return &ValidationResult{Valid: true}
}

// validateType validates that the field value matches the expected type
func (v *JSONSchemaValidator) validateType(value interface{}, schema map[string]interface{}, path string) *ValidationResult {
	expectedType, ok := schema["type"].(string)
	if !ok {
		return &ValidationResult{Valid: true}
	}

	switch expectedType {
	case "string":
		if _, ok := value.(string); !ok {
			return NewValidationError(fmt.Errorf("%w: expected string", ErrInvalidFieldType), path)
		}
	case "number":
		switch value.(type) {
		case float64, float32, int, int64, int32:
			// Valid numeric types
		default:
			return NewValidationError(fmt.Errorf("%w: expected number", ErrInvalidFieldType), path)
		}
	case "integer":
		switch value.(type) {
		case int, int64, int32:
			// Valid integer types
		default:
			return NewValidationError(fmt.Errorf("%w: expected integer", ErrInvalidFieldType), path)
		}
	case "boolean":
		if _, ok := value.(bool); !ok {
			return NewValidationError(fmt.Errorf("%w: expected boolean", ErrInvalidFieldType), path)
		}
	case "object":
		if _, ok := value.(map[string]interface{}); !ok {
			return NewValidationError(fmt.Errorf("%w: expected object", ErrInvalidFieldType), path)
		}
	case "array":
		if _, ok := value.([]interface{}); !ok {
			return NewValidationError(fmt.Errorf("%w: expected array", ErrInvalidFieldType), path)
		}
	}

	return &ValidationResult{Valid: true}
}

// validateFormat validates that the field value matches the expected format
func (v *JSONSchemaValidator) validateFormat(value interface{}, schema map[string]interface{}, path string) *ValidationResult {
	format, ok := schema["format"].(string)
	if !ok {
		return &ValidationResult{Valid: true}
	}

	strValue, ok := value.(string)
	if !ok {
		return NewValidationError(fmt.Errorf("%w: format validation requires string", ErrInvalidFieldType), path)
	}

	switch format {
	case "date-time":
		// Basic date-time format validation (ISO 8601)
		if !regexp.MustCompile(`^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(?:\.\d+)?(?:Z|[+-]\d{2}:\d{2})$`).MatchString(strValue) {
			return NewValidationError(fmt.Errorf("%w: invalid date-time format", ErrInvalidFieldValue), path)
		}
	case "email":
		// Basic email format validation
		if !regexp.MustCompile(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`).MatchString(strValue) {
			return NewValidationError(fmt.Errorf("%w: invalid email format", ErrInvalidFieldValue), path)
		}
	case "uri":
		// Basic URI format validation
		if !regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9+.-]*:\/\/`).MatchString(strValue) {
			return NewValidationError(fmt.Errorf("%w: invalid URI format", ErrInvalidFieldValue), path)
		}
	}

	return &ValidationResult{Valid: true}
}

// validateArray validates that the array items match the expected schema
func (v *JSONSchemaValidator) validateArray(value interface{}, itemSchema map[string]interface{}, path string) *ValidationResult {
	arr, ok := value.([]interface{})
	if !ok {
		return NewValidationError(fmt.Errorf("%w: expected array", ErrInvalidFieldType), path)
	}

	for i, item := range arr {
		itemPath := fmt.Sprintf("%s[%d]", path, i)

		// Validate item type
		if err := v.validateType(item, itemSchema, itemPath); err != nil {
			return err
		}

		// Validate item format
		if err := v.validateFormat(item, itemSchema, itemPath); err != nil {
			return err
		}

		// Recursive validation for objects
		if itemType, ok := itemSchema["type"].(string); ok && itemType == "object" {
			if itemObjectSchema, ok := itemSchema["properties"].(map[string]interface{}); ok {
				if result := v.validateObject(item, map[string]interface{}{
					"properties": itemObjectSchema,
					"required":   itemSchema["required"],
				}, itemPath); !result.Valid {
					return result
				}
			}
		}
	}

	return &ValidationResult{Valid: true}
}

// isHashedOrEncoded checks if a value appears to be hashed or encoded
func (v *JSONSchemaValidator) isHashedOrEncoded(value interface{}) bool {
	// Check if it's a string first
	strValue, ok := value.(string)
	if !ok {
		return false
	}

	// Check if it's a SHA-256 hash (64 hex characters)
	if regexp.MustCompile(`^[0-9a-f]{64}$`).MatchString(strValue) {
		return true
	}

	// Check if it's base64 encoded
	if regexp.MustCompile(`^[A-Za-z0-9+/]+={0,2}$`).MatchString(strValue) && len(strValue) % 4 == 0 {
		return true
	}

	// Check if the field name indicates it's a hash
	// This will need reflection to check the actual field name, which is not 
	// available in this context, but would be implemented in a real validator

	return false
}

// SimpleValidator implements a simple schema validator
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
			return NewValidationError(ErrInvalidFieldType, "")
		}

		if err := json.Unmarshal(jsonBytes, &dataMap); err != nil {
			return NewValidationError(ErrInvalidFieldType, "")
		}
	}

	// Check required fields
	for _, field := range v.requiredFields {
		parts := strings.Split(field, ".")
		current := dataMap
		
		for i, part := range parts {
			if i == len(parts)-1 {
				if _, exists := current[part]; !exists {
					return NewValidationError(fmt.Errorf("%w: %s", ErrMissingRequiredField, field), field)
				}
			} else {
				next, exists := current[part]
				if !exists {
					return NewValidationError(fmt.Errorf("%w: %s", ErrMissingRequiredField, field), field)
				}
				
				nextMap, ok := next.(map[string]interface{})
				if !ok {
					return NewValidationError(fmt.Errorf("%w: %s is not an object", ErrInvalidFieldType, part), field)
				}
				
				current = nextMap
			}
		}
	}

	// Check for PII fields if enabled
	if v.piiDetection {
		for _, piiField := range v.piiFields {
			parts := strings.Split(piiField, ".")
			current := dataMap
			
			for i, part := range parts {
				if i == len(parts)-1 {
					if value, exists := current[part]; exists {
						if !v.isHashedOrEncoded(value) {
							return NewValidationError(fmt.Errorf("%w: %s", ErrPIIDetected, piiField), piiField)
						}
					}
				} else {
					next, exists := current[part]
					if !exists {
						break
					}
					
					nextMap, ok := next.(map[string]interface{})
					if !ok {
						break
					}
					
					current = nextMap
				}
			}
		}
	}

	return &ValidationResult{Valid: true}
}

// isHashedOrEncoded checks if a value appears to be hashed or encoded
func (v *SimpleValidator) isHashedOrEncoded(value interface{}) bool {
	// Check if it's a string first
	strValue, ok := value.(string)
	if !ok {
		return false
	}

	// Check if it's a SHA-256 hash (64 hex characters)
	if regexp.MustCompile(`^[0-9a-f]{64}$`).MatchString(strValue) {
		return true
	}

	// Check if it's base64 encoded
	if regexp.MustCompile(`^[A-Za-z0-9+/]+={0,2}$`).MatchString(strValue) && len(strValue) % 4 == 0 {
		return true
	}

	// Check if the field name contains "_hash" or "_encoded"
	valueType := reflect.TypeOf(value)
	if valueType.Kind() == reflect.Struct {
		for i := 0; i < valueType.NumField(); i++ {
			field := valueType.Field(i)
			if strings.Contains(field.Name, "_hash") || strings.Contains(field.Name, "_encoded") {
				return true
			}
		}
	}

	return false
}
