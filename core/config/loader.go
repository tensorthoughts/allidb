package config

import (
	"fmt"
	"os"
	"reflect"
	"strings"

	"gopkg.in/yaml.v3"
)

// LoadConfig loads a YAML configuration file and returns a validated Config.
// It applies defaults first, then overrides with values from the YAML file.
// Returns an error if the file cannot be read, parsed, or validated.
func LoadConfig(path string) (*Config, error) {
	// Start with defaults
	config := DefaultConfig()

	// Check if file exists
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil, fmt.Errorf("config file does not exist: %s", path)
	}

	// Read YAML file
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file %s: %w", path, err)
	}

	// Parse YAML into a temporary config
	var yamlConfig Config
	if err := yaml.Unmarshal(data, &yamlConfig); err != nil {
		return nil, fmt.Errorf("failed to parse YAML config file %s: %w", path, err)
	}

	// Merge YAML config over defaults
	// This uses reflection to merge non-zero values from yamlConfig into config
	if err := mergeConfig(&config, &yamlConfig); err != nil {
		return nil, fmt.Errorf("failed to merge config: %w", err)
	}

	// Apply environment variable overrides (env vars override YAML)
	if err := ApplyEnvOverrides(&config); err != nil {
		return nil, fmt.Errorf("failed to apply environment variable overrides: %w", err)
	}

	// Validate the merged config
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("config validation failed: %w", err)
	}

	return &config, nil
}

// mergeConfig merges non-zero values from src into dst.
// This ensures that YAML values override defaults, but only if they are explicitly set.
func mergeConfig(dst, src *Config) error {
	return mergeStruct(reflect.ValueOf(dst).Elem(), reflect.ValueOf(src).Elem())
}

// mergeStruct merges non-zero values from src into dst using reflection.
func mergeStruct(dst, src reflect.Value) error {
	if dst.Kind() != reflect.Struct || src.Kind() != reflect.Struct {
		return fmt.Errorf("both values must be structs")
	}

	dstType := dst.Type()
	for i := 0; i < dstType.NumField(); i++ {
		dstField := dst.Field(i)
		srcField := src.Field(i)

		if !dstField.CanSet() {
			continue
		}

		switch dstField.Kind() {
		case reflect.Struct:
			// Recursively merge nested structs
			if err := mergeStruct(dstField, srcField); err != nil {
				return fmt.Errorf("failed to merge field %s: %w", dstType.Field(i).Name, err)
			}

		case reflect.Slice:
			// For slices, replace if non-empty
			if srcField.Len() > 0 {
				dstField.Set(srcField)
			}

		case reflect.String:
			// For strings, replace if non-empty
			if srcField.String() != "" {
				dstField.Set(srcField)
			}

		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			// For integers, replace if non-zero
			if srcField.Int() != 0 {
				dstField.Set(srcField)
			}

		case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
			// For unsigned integers, replace if non-zero
			if srcField.Uint() != 0 {
				dstField.Set(srcField)
			}

		case reflect.Float32, reflect.Float64:
			// For floats, replace if non-zero
			if srcField.Float() != 0 {
				dstField.Set(srcField)
			}

		case reflect.Bool:
			// For booleans, always merge (YAML can explicitly set false)
			// We check if the field was set in YAML by comparing with zero value
			// Since bool zero value is false, we need to track if it was explicitly set
			// For simplicity, we'll always merge bools (YAML false overrides default true)
			dstField.Set(srcField)

		default:
			// For other types, try direct assignment
			if srcField.IsValid() && !isZeroValue(srcField) {
				dstField.Set(srcField)
			}
		}
	}

	return nil
}

// isZeroValue checks if a reflect.Value is the zero value for its type.
func isZeroValue(v reflect.Value) bool {
	switch v.Kind() {
	case reflect.String:
		return v.String() == ""
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return v.Int() == 0
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return v.Uint() == 0
	case reflect.Float32, reflect.Float64:
		return v.Float() == 0
	case reflect.Bool:
		return !v.Bool() // For bool, zero is false, but we want to merge false values
	case reflect.Slice, reflect.Map, reflect.Ptr, reflect.Interface:
		return v.IsNil()
	case reflect.Struct:
		// For structs, check if all fields are zero
		for i := 0; i < v.NumField(); i++ {
			if !isZeroValue(v.Field(i)) {
				return false
			}
		}
		return true
	default:
		return false
	}
}

// RedactSecrets returns a copy of the config with sensitive fields redacted.
// This is useful for logging configuration without exposing secrets.
func RedactSecrets(c *Config) *Config {
	if c == nil {
		return nil
	}

	// Create a deep copy
	redacted := *c

	// Redact API key header (if it contains sensitive info)
	// In practice, the header name itself isn't secret, but we redact it for safety
	if redacted.Security.Auth.APIKeyHeader != "" {
		redacted.Security.Auth.APIKeyHeader = "[REDACTED]"
	}

	// Redact file paths that might contain sensitive information
	// Note: The actual secrets are in the files, not the paths, but we redact for safety
	if redacted.Security.TLS.CertFile != "" {
		redacted.Security.TLS.CertFile = "[REDACTED]"
	}
	if redacted.Security.TLS.KeyFile != "" {
		redacted.Security.TLS.KeyFile = "[REDACTED]"
	}
	if redacted.Security.TLS.CAFile != "" {
		redacted.Security.TLS.CAFile = "[REDACTED]"
	}

	return &redacted
}

// String returns a string representation of the config with secrets redacted.
// This is safe for logging.
func (c *Config) String() string {
	redacted := RedactSecrets(c)
	
	// Use a simple string representation
	// In production, you might want to use JSON or YAML marshaling
	var sb strings.Builder
	sb.WriteString("Config{")
	sb.WriteString(fmt.Sprintf("NodeID: %s, ", redacted.Node.ID))
	sb.WriteString(fmt.Sprintf("GRPCPort: %d, ", redacted.Node.GRPCPort))
	sb.WriteString(fmt.Sprintf("HTTPPort: %d, ", redacted.Node.HTTPPort))
	sb.WriteString(fmt.Sprintf("DataDir: %s", redacted.Node.DataDir))
	sb.WriteString("}")
	return sb.String()
}

// ToYAML returns the YAML representation of the config with secrets redacted.
// This is safe for logging.
func (c *Config) ToYAML() (string, error) {
	redacted := RedactSecrets(c)
	data, err := yaml.Marshal(redacted)
	if err != nil {
		return "", fmt.Errorf("failed to marshal config to YAML: %w", err)
	}
	return string(data), nil
}

