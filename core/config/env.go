package config

import (
	"fmt"
	"os"
	"reflect"
	"strconv"
	"strings"
)

const envPrefix = "ALLIDB_"

// ApplyEnvOverrides applies environment variable overrides to the config.
// Environment variables must have the ALLIDB_ prefix and use underscores
// to separate nested fields (e.g., ALLIDB_NODE_ID, ALLIDB_AI_LLM_PROVIDER).
// Returns an error if a type mismatch occurs.
func ApplyEnvOverrides(config *Config) error {
	// Build a map of environment variable names to field paths
	fieldMap := buildFieldMap(reflect.TypeOf(*config))

	// Get all environment variables
	envVars := os.Environ()

	for _, envVar := range envVars {
		parts := strings.SplitN(envVar, "=", 2)
		if len(parts) != 2 {
			continue
		}

		key := parts[0]
		value := parts[1]

		// Only process variables with the prefix
		if !strings.HasPrefix(key, envPrefix) {
			continue
		}

		// Remove prefix and convert to field path
		fieldPath := strings.TrimPrefix(key, envPrefix)
		if fieldPath == "" {
			continue
		}

		// Find the field path in our map
		fieldInfo, ok := fieldMap[strings.ToLower(fieldPath)]
		if !ok {
			// Ignore unknown keys
			continue
		}

		// Set the value using reflection
		if err := setFieldValue(config, fieldInfo.path, value, fieldInfo.fieldType); err != nil {
			return fmt.Errorf("failed to set %s from %s: %w", fieldPath, key, err)
		}
	}

	return nil
}

// fieldInfo holds information about a config field.
type fieldInfo struct {
	path      []string
	fieldType reflect.Type
}

// buildFieldMap builds a map of lowercase environment variable names to field paths.
// Example: "node_id" -> {path: ["Node", "ID"], fieldType: string}
func buildFieldMap(t reflect.Type) map[string]fieldInfo {
	fieldMap := make(map[string]fieldInfo)
	buildFieldMapRecursive(t, []string{}, []string{}, fieldMap)
	return fieldMap
}

// buildFieldMapRecursive recursively builds the field map.
// path is the Go field names for reflection, yamlPath is the YAML tag names for env keys.
func buildFieldMapRecursive(t reflect.Type, path []string, yamlPath []string, fieldMap map[string]fieldInfo) {
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}

	if t.Kind() != reflect.Struct {
		return
	}

	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		
		// Skip unexported fields
		if !field.IsExported() {
			continue
		}

		// Get the YAML tag name, or use the field name converted to snake_case
		yamlTag := field.Tag.Get("yaml")
		var yamlFieldName string
		if yamlTag != "" && yamlTag != "-" {
			// Remove options from YAML tag (e.g., "name,omitempty" -> "name")
			yamlFieldName = strings.Split(yamlTag, ",")[0]
		} else {
			// Convert Go field name to snake_case for YAML
			yamlFieldName = toSnakeCase(field.Name)
		}

		// Path for reflection (Go field names)
		newPath := append(path, field.Name)
		// Path for env key (YAML tag names)
		newYamlPath := append(yamlPath, yamlFieldName)
		envKey := strings.ToLower(strings.Join(newYamlPath, "_"))

		// If it's a nested struct, recurse
		if field.Type.Kind() == reflect.Struct {
			buildFieldMapRecursive(field.Type, newPath, newYamlPath, fieldMap)
		} else {
			// Leaf field - add to map
			fieldMap[envKey] = fieldInfo{
				path:      newPath,
				fieldType: field.Type,
			}
		}
	}
}


// toSnakeCase converts a CamelCase string to snake_case.
// Example: "GRPCPort" -> "grpc_port", "ListenAddress" -> "listen_address", "ID" -> "id"
func toSnakeCase(s string) string {
	if s == "" {
		return s
	}

	var result strings.Builder
	result.Grow(len(s) + 5) // Pre-allocate with some extra space

	runes := []rune(s)
	for i, r := range runes {
		if r >= 'A' && r <= 'Z' {
			// Add underscore before uppercase letter if:
			// 1. Not the first character, AND
			// 2. Either:
			//    - Previous character was lowercase, OR
			//    - Previous character was uppercase AND next character is lowercase (acronym boundary)
			if i > 0 {
				prev := runes[i-1]
				nextIsLower := i+1 < len(runes) && runes[i+1] >= 'a' && runes[i+1] <= 'z'
				prevIsLower := prev >= 'a' && prev <= 'z'
				prevIsUpper := prev >= 'A' && prev <= 'Z'
				
				if prevIsLower || (prevIsUpper && nextIsLower) {
					result.WriteByte('_')
				}
			}
		}
		result.WriteRune(r)
	}

	return strings.ToLower(result.String())
}

// setFieldValue sets a field value using reflection, converting the string value
// to the appropriate type.
func setFieldValue(config *Config, path []string, value string, fieldType reflect.Type) error {
	// Navigate to the field
	field := reflect.ValueOf(config).Elem()
	for _, p := range path {
		field = field.FieldByName(p)
		if !field.IsValid() {
			return fmt.Errorf("field %s not found", strings.Join(path, "."))
		}
		if field.Kind() == reflect.Ptr {
			if field.IsNil() {
				field.Set(reflect.New(field.Type().Elem()))
			}
			field = field.Elem()
		}
	}

	if !field.CanSet() {
		return fmt.Errorf("field %s cannot be set", strings.Join(path, "."))
	}

	// Convert value based on field type
	switch fieldType.Kind() {
	case reflect.String:
		field.SetString(value)

	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		intValue, err := strconv.ParseInt(value, 10, 64)
		if err != nil {
			return fmt.Errorf("invalid int value '%s': %w", value, err)
		}
		if field.OverflowInt(intValue) {
			return fmt.Errorf("int value '%s' overflows field type", value)
		}
		field.SetInt(intValue)

	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		uintValue, err := strconv.ParseUint(value, 10, 64)
		if err != nil {
			return fmt.Errorf("invalid uint value '%s': %w", value, err)
		}
		if field.OverflowUint(uintValue) {
			return fmt.Errorf("uint value '%s' overflows field type", value)
		}
		field.SetUint(uintValue)

	case reflect.Float32, reflect.Float64:
		floatValue, err := strconv.ParseFloat(value, 64)
		if err != nil {
			return fmt.Errorf("invalid float value '%s': %w", value, err)
		}
		if field.OverflowFloat(floatValue) {
			return fmt.Errorf("float value '%s' overflows field type", value)
		}
		field.SetFloat(floatValue)

	case reflect.Bool:
		boolValue, err := strconv.ParseBool(value)
		if err != nil {
			return fmt.Errorf("invalid bool value '%s': %w", value, err)
		}
		field.SetBool(boolValue)

	case reflect.Slice:
		// For slices, split by comma
		if fieldType.Elem().Kind() == reflect.String {
			values := strings.Split(value, ",")
			// Trim whitespace from each value
			for i, v := range values {
				values[i] = strings.TrimSpace(v)
			}
			field.Set(reflect.ValueOf(values))
		} else {
			return fmt.Errorf("unsupported slice element type: %s", fieldType.Elem().Kind())
		}

	default:
		return fmt.Errorf("unsupported field type: %s", fieldType.Kind())
	}

	return nil
}

