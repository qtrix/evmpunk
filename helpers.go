package evmpunk

import (
	"bufio"
	"os"
	"path/filepath"
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

// SchemaExists checks if a schema migration directory exists and has migration files
func SchemaExists(migrationDir string) bool {
	entries, err := os.ReadDir(migrationDir)
	if err != nil {
		return false
	}
	// Schema exists if directory has at least one .sql file
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".sql") {
			return true
		}
	}
	return false
}

// DetectModulePath reads go.mod from baseDir and returns the module path
func DetectModulePath(baseDir string) string {
	goModPath := filepath.Join(baseDir, "go.mod")
	file, err := os.Open(goModPath)
	if err != nil {
		return ""
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "module ") {
			return strings.TrimSpace(strings.TrimPrefix(line, "module "))
		}
	}
	return ""
}
