package evmpunk

import (
	"os"
	"strings"
)

// ToSnakeCase converts CamelCase to snake_case
func ToSnakeCase(s string) string {
	var result []rune
	for i, r := range s {
		if i > 0 && 'A' <= r && r <= 'Z' {
			result = append(result, '_')
		}
		result = append(result, r)
	}
	return strings.ToLower(string(result))
}

// Capitalize returns string with first letter uppercase
func Capitalize(s string) string {
	if len(s) == 0 {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

// ABITypeToSQL converts Solidity types to PostgreSQL types
func ABITypeToSQL(abiType string) string {
	switch {
	case strings.HasPrefix(abiType, "uint"), strings.HasPrefix(abiType, "int"):
		return "NUMERIC"
	case abiType == "address":
		return "TEXT"
	case abiType == "bool":
		return "BOOLEAN"
	case abiType == "string":
		return "TEXT"
	case strings.HasPrefix(abiType, "bytes"):
		return "BYTEA"
	default:
		return "TEXT"
	}
}

// WriteFile writes content to a file
func WriteFile(path, content string) error {
	return os.WriteFile(path, []byte(content), 0644)
}

// CopyFile copies a file from src to dst
func CopyFile(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0644)
}
