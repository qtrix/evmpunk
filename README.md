# evmpunk

Ethereum indexer code generator. Generates complete indexer structure from ABI files.

## Installation

```bash
go get github.com/taler/evmpunk
```

## Usage

### As CLI (in indexer project)

```bash
./indexer generate
```

### As a library

```go
package main

import "github.com/qtrix/evmpunk"

func main() {
    // Run interactive generator
    evmpunk.Generate("/path/to/project")
}
```

### Available functions

```go
// Parse events from ABI
events, err := evmpunk.ParseABIEvents("path/to/Contract.json")

// Generate files individually
cfg := evmpunk.GeneratorConfig{
    Name:         "myindexer",
    PackageName:  "myindexer",
    SchemaName:   "myindexer",
    IndexerID:    "myindexer",
    Description:  "My custom indexer",
    Dependencies: []string{"preprocessor"},
    Contracts:    contracts,
    Events:       events,
}

mainFile := evmpunk.GenerateMainFile(cfg)
executeFile := evmpunk.GenerateExecuteFile(cfg)
typesFile := evmpunk.GenerateTypesFile(cfg)
cacheFile := evmpunk.GenerateCacheFile(cfg.PackageName)
migration := evmpunk.GenerateMigrationForEvent(cfg.SchemaName, event)

// Helpers
snake := evmpunk.ToSnakeCase("MyEventName")  // "my_event_name"
sql := evmpunk.ABITypeToSQL("uint256")       // "NUMERIC"
```

## Generated output

```
indexers/myindexer/
├── myindexer.go    # Main struct, Init, Load, Rollback
├── execute.go      # Execute() and SaveToDatabase()
├── types.go        # Result struct with event slices
├── cache.go        # refreshCache() and isWatchedAddress()
└── README.md       # Documentation

db/migrations/myindexer/
├── 001_create_table_deposits.sql
├── 002_create_table_withdrawals.sql
└── ...
```

## Generated pattern

Execute uses the ethtypes pattern:

```go
log, err := ethgen.W3LogToLog(vLog)
if err != nil {
    continue
}

if ethtypes.Vault.IsDepositEvent(log) {
    event, err := ethtypes.Vault.DepositEvent(log)
    if err != nil {
        continue
    }
    i.result.DepositEvents = append(i.result.DepositEvents, event)
}
```

## SQL type mapping

| Solidity | PostgreSQL |
|----------|------------|
| uint*, int* | NUMERIC |
| address | TEXT |
| bool | BOOLEAN |
| string | TEXT |
| bytes* | BYTEA |

## Package structure

```
evmpunk/
├── generator.go   # Generate() - main interactive flow
├── templates.go   # Generation functions for each file
├── parser.go      # ParseABIEvents() - extracts events from ABI
├── helpers.go     # ToSnakeCase, ABITypeToSQL, WriteFile, CopyFile
└── types.go       # EventInfo, EventField, ContractInfo, GeneratorConfig
```

## License

MIT
