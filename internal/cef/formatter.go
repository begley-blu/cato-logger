package cef

import (
	"fmt"
	"sort"
	"strings"
)

// Formatter handles CEF message formatting
type Formatter struct {
	vendor        string
	product       string
	version       string
	fieldMappings map[string]string
	orderedFields []string
}

// NewFormatter creates a new CEF formatter
func NewFormatter(vendor, product, version string, fieldMappings map[string]string, orderedFields []string) *Formatter {
	return &Formatter{
		vendor:        vendor,
		product:       product,
		version:       version,
		fieldMappings: fieldMappings,
		orderedFields: orderedFields,
	}
}

// Format converts an event to CEF format
func (f *Formatter) Format(fieldsMap map[string]string) string {
	signature := getMapValue(fieldsMap, "event_type", "Unknown")
	name := fmt.Sprintf("%s - %s",
		signature,
		getMapValue(fieldsMap, "event_sub_type", "Unknown"))

	severity := mapEventTypeToSeverity(signature)

	header := fmt.Sprintf("CEF:0|%s|%s|%s|%s|%s|%d|",
		f.vendor, f.product, f.version,
		signature, name, severity)

	extensions := make(map[string]string)

	// Apply field mappings
	for sourceKey, targetKey := range f.fieldMappings {
		if value, exists := fieldsMap[sourceKey]; exists && value != "" {
			extensions[targetKey] = sanitizeValue(value)
		}
	}

	// Add unmapped fields
	for k, v := range fieldsMap {
		if !isMappedField(k, f.fieldMappings) && v != "" {
			extensions[k] = sanitizeValue(v)
		}
	}

	// Format extensions in order
	var parts []string

	// Ordered fields first
	for _, field := range f.orderedFields {
		if value, exists := extensions[field]; exists {
			parts = append(parts, fmt.Sprintf("%s=%s", field, value))
			delete(extensions, field)
		}
	}

	// Remaining fields alphabetically
	var remaining []string
	for k := range extensions {
		remaining = append(remaining, k)
	}
	sort.Strings(remaining)

	for _, field := range remaining {
		parts = append(parts, fmt.Sprintf("%s=%s", field, extensions[field]))
	}

	return header + strings.Join(parts, " ")
}

// sanitizeValue escapes special CEF characters
func sanitizeValue(value string) string {
	value = strings.ReplaceAll(value, "\\", "\\\\")
	value = strings.ReplaceAll(value, "=", "\\=")
	value = strings.ReplaceAll(value, "|", "\\|")
	value = strings.ReplaceAll(value, "\n", "\\n")
	value = strings.ReplaceAll(value, "\r", "\\r")
	return value
}

// isMappedField checks if a field name exists in the mapping
func isMappedField(fieldName string, fieldMappings map[string]string) bool {
	_, exists := fieldMappings[fieldName]
	return exists
}

// mapEventTypeToSeverity converts event types to CEF severity levels
func mapEventTypeToSeverity(eventType string) int {
	severityMap := map[string]int{
		"Threat":           10,
		"Malware":          10,
		"Attack":           9,
		"Intrusion":        9,
		"Security":         8,
		"Policy Violation": 7,
		"Warning":          6,
		"Alert":            6,
		"Connectivity":     5,
		"Network":          4,
		"Traffic":          3,
		"Info":             2,
		"Debug":            1,
	}

	if severity, exists := severityMap[eventType]; exists {
		return severity
	}
	return 5 // Default severity
}

// getMapValue safely retrieves a value from a map with a default
func getMapValue(m map[string]string, key, defaultVal string) string {
	if val, ok := m[key]; ok && val != "" {
		return val
	}
	return defaultVal
}
