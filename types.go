package evmpunk

// EventInfo holds parsed event information from ABI
type EventInfo struct {
	Name      string
	Fields    []EventField
	Signature string
}

// EventField represents a single field in an event
type EventField struct {
	Name    string
	Type    string
	Indexed bool
}

// ContractInfo holds contract metadata and selected events
type ContractInfo struct {
	Name     string
	Events   []EventInfo
	ABIPath  string
	FileName string
}

// GeneratorConfig holds all configuration for code generation
type GeneratorConfig struct {
	Name         string
	PackageName  string
	SchemaName   string
	IndexerID    string
	Description  string
	Dependencies []string
	Contracts    []ContractInfo
	Events       []EventInfo
	OutputDir    string
	MigrationDir string
}
