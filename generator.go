package evmpunk

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// Generate runs the interactive generator
func Generate(baseDir string) {
	reader := bufio.NewReader(os.Stdin)

	fmt.Println("═══════════════════════════════════════════════════════")
	fmt.Println("    🚀 evmpunk - Ethereum Indexer Generator")
	fmt.Println("═══════════════════════════════════════════════════════")
	fmt.Println()
	fmt.Print("Indexer name (e.g., fusion, mytoken): ")

	name, _ := reader.ReadString('\n')
	name = strings.TrimSpace(name)

	packageName := strings.ReplaceAll(name, "-", "")
	fmt.Printf("Package name [%s]: ", packageName)
	pkgInput, _ := reader.ReadString('\n')
	if pkgInput = strings.TrimSpace(pkgInput); pkgInput != "" {
		packageName = pkgInput
	}

	schemaName := packageName
	fmt.Printf("Database schema name [%s]: ", schemaName)
	schemaInput, _ := reader.ReadString('\n')
	if schemaInput = strings.TrimSpace(schemaInput); schemaInput != "" {
		schemaName = schemaInput
	}

	indexerID := name
	fmt.Printf("Indexer ID [%s]: ", indexerID)
	idInput, _ := reader.ReadString('\n')
	if idInput = strings.TrimSpace(idInput); idInput != "" {
		indexerID = idInput
	}

	fmt.Print("Description: ")
	description, _ := reader.ReadString('\n')
	description = strings.TrimSpace(description)

	fmt.Print("Dependencies (comma-separated, or Enter for 'preprocessor'): ")
	depsInput, _ := reader.ReadString('\n')
	depsInput = strings.TrimSpace(depsInput)
	var dependencies []string
	if depsInput == "" {
		dependencies = []string{"preprocessor"}
	} else {
		for _, dep := range strings.Split(depsInput, ",") {
			dependencies = append(dependencies, strings.TrimSpace(dep))
		}
	}

	// ABI Integration
	fmt.Println()
	fmt.Println("──────────────────────────────────────────────────────")
	fmt.Println("    📋 ABI Integration")
	fmt.Println("──────────────────────────────────────────────────────")
	fmt.Print("Do you have ABI files to add? [y/N]: ")
	hasABI, _ := reader.ReadString('\n')
	hasABI = strings.TrimSpace(strings.ToLower(hasABI))

	var contracts []ContractInfo
	var selectedEvents []EventInfo
	var needsEthTypesGen bool

	abiSourceDir := filepath.Join(baseDir, "ethtypes/_source")

	if hasABI == "y" || hasABI == "yes" {
		for {
			fmt.Print("\nABI file name (e.g., MyContract.json) or Enter to finish: ")
			abiFile, _ := reader.ReadString('\n')
			if abiFile = strings.TrimSpace(abiFile); abiFile == "" {
				break
			}

			abiPath := filepath.Join(abiSourceDir, abiFile)
			if _, err := os.Stat(abiPath); os.IsNotExist(err) {
				fmt.Printf("⚠️  File not found: %s\n", abiPath)
				continue
			}

			contractName := strings.TrimSuffix(abiFile, ".json")
			events, err := ParseABIEvents(abiPath)
			if err != nil {
				fmt.Printf("⚠️  Error parsing ABI: %v\n", err)
				continue
			}

			if len(events) == 0 {
				fmt.Println("⚠️  No events found in this ABI")
				continue
			}

			fmt.Printf("\n✅ Found %d events in %s:\n", len(events), abiFile)
			for i, event := range events {
				fmt.Printf("  %d. %s(", i+1, event.Name)
				var fieldTypes []string
				for _, field := range event.Fields {
					indexed := ""
					if field.Indexed {
						indexed = " indexed"
					}
					fieldTypes = append(fieldTypes, fmt.Sprintf("%s %s%s", field.Type, field.Name, indexed))
				}
				fmt.Printf("%s)\n", strings.Join(fieldTypes, ", "))
			}

			fmt.Print("\nWhich events to index? (comma-separated numbers, or 'all'): ")
			selection, _ := reader.ReadString('\n')
			selection = strings.TrimSpace(selection)

			var contractEvents []EventInfo
			if selection == "all" {
				contractEvents = events
				selectedEvents = append(selectedEvents, events...)
			} else {
				for _, idxStr := range strings.Split(selection, ",") {
					var idx int
					fmt.Sscanf(strings.TrimSpace(idxStr), "%d", &idx)
					if idx > 0 && idx <= len(events) {
						contractEvents = append(contractEvents, events[idx-1])
						selectedEvents = append(selectedEvents, events[idx-1])
					}
				}
			}

			contracts = append(contracts, ContractInfo{
				Name:     contractName,
				Events:   contractEvents,
				ABIPath:  abiPath,
				FileName: abiFile,
			})
			needsEthTypesGen = true
		}
	}

	// Build config
	cfg := GeneratorConfig{
		Name:         name,
		PackageName:  packageName,
		SchemaName:   schemaName,
		IndexerID:    indexerID,
		Description:  description,
		Dependencies: dependencies,
		Contracts:    contracts,
		Events:       selectedEvents,
		OutputDir:    filepath.Join(baseDir, "indexers", packageName),
		MigrationDir: filepath.Join(baseDir, "db/migrations", schemaName),
	}

	// Generate files
	fmt.Println()
	fmt.Printf("Generating indexer in: %s\n", cfg.OutputDir)

	os.MkdirAll(cfg.OutputDir, 0755)

	// Main file
	WriteFile(filepath.Join(cfg.OutputDir, packageName+".go"), GenerateMainFile(cfg))

	// Execute file
	WriteFile(filepath.Join(cfg.OutputDir, "execute.go"), GenerateExecuteFile(cfg))

	// Types file
	WriteFile(filepath.Join(cfg.OutputDir, "types.go"), GenerateTypesFile(cfg))

	// Cache file
	WriteFile(filepath.Join(cfg.OutputDir, "cache.go"), GenerateCacheFile(packageName))

	// README
	WriteFile(filepath.Join(cfg.OutputDir, "README.md"), GenerateReadme(name, description, selectedEvents))

	// Migrations - ONE FILE PER EVENT
	var createdMigrations []string
	if len(selectedEvents) > 0 {
		os.MkdirAll(cfg.MigrationDir, 0755)

		for i, event := range selectedEvents {
			tableName := ToSnakeCase(event.Name) + "s"
			migrationFile := filepath.Join(cfg.MigrationDir,
				fmt.Sprintf("%03d_create_table_%s.sql", i+1, tableName))

			WriteFile(migrationFile, GenerateMigrationForEvent(schemaName, event))
			createdMigrations = append(createdMigrations, migrationFile)
		}
	}

	// Create schema migration in public
	publicMigrationDir := filepath.Join(baseDir, "db/migrations/public")
	schemaMigrationFile := CreateSchemaMigration(publicMigrationDir, schemaName)

	// Update cmd/reset.go
	AddSchemaToResetCommand(filepath.Join(baseDir, "cmd/reset.go"), schemaName)

	// Print summary
	fmt.Println()
	fmt.Println("✅ Indexer generated successfully!")
	fmt.Println()
	fmt.Println("Files created:")
	fmt.Printf("  - %s/%s.go\n", cfg.OutputDir, packageName)
	fmt.Printf("  - %s/execute.go\n", cfg.OutputDir)
	fmt.Printf("  - %s/types.go\n", cfg.OutputDir)
	fmt.Printf("  - %s/cache.go\n", cfg.OutputDir)
	fmt.Printf("  - %s/README.md\n", cfg.OutputDir)

	if schemaMigrationFile != "" {
		fmt.Printf("  - %s\n", schemaMigrationFile)
	}

	if len(createdMigrations) > 0 {
		fmt.Println("  - Migrations:")
		for _, mig := range createdMigrations {
			fmt.Printf("    - %s\n", mig)
		}
	}

	fmt.Println()
	fmt.Println("Updated:")
	fmt.Println("  - cmd/reset.go")

	fmt.Println()
	fmt.Println("Next steps:")
	fmt.Printf("  1. Register in indexers/register.go: %s.New()\n", packageName)

	if needsEthTypesGen {
		fmt.Println("  2. Run: ./indexer generate-eth-types")
		fmt.Println("  3. Run: ./indexer migrate")

		fmt.Print("\nRun generate-eth-types now? [Y/n]: ")
		runGen, _ := reader.ReadString('\n')
		if runGen = strings.TrimSpace(strings.ToLower(runGen)); runGen == "" || runGen == "y" {
			fmt.Println("\nPreparing selected ABIs...")

			// Create temporary folder for selected ABIs
			tmpABIFolder := filepath.Join(baseDir, "ethtypes", "_source_tmp")
			os.RemoveAll(tmpABIFolder)
			os.MkdirAll(tmpABIFolder, 0755)

			// Copy only selected ABI files
			for _, contract := range contracts {
				destPath := filepath.Join(tmpABIFolder, contract.FileName)
				if err := CopyFile(contract.ABIPath, destPath); err != nil {
					fmt.Printf("⚠️  Error copying %s: %v\n", contract.FileName, err)
				} else {
					fmt.Printf("  ✓ Copied %s\n", contract.FileName)
				}
			}

			fmt.Println("\nGenerating ETH types...")
			cmd := exec.Command("go", "run", ".", "generate-eth-types",
				"--ethtypes.abi-folder", tmpABIFolder,
				"--ethtypes.package-path", "ethtypes")
			cmd.Dir = baseDir
			cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
			if err := cmd.Run(); err != nil {
				fmt.Printf("⚠️  Error: %v\n", err)
			} else {
				fmt.Println("✅ Done!")
			}

			// Cleanup temp folder
			os.RemoveAll(tmpABIFolder)
		}
	} else {
		fmt.Println("  2. Run: ./indexer migrate")
	}
	fmt.Println()
}

// GetNextMigrationNumber finds the highest migration number in a directory and returns next
func GetNextMigrationNumber(migrationDir string) int {
	entries, err := os.ReadDir(migrationDir)
	if err != nil {
		return 1
	}

	re := regexp.MustCompile(`^(\d+)_`)
	var numbers []int

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		matches := re.FindStringSubmatch(entry.Name())
		if len(matches) > 1 {
			if num, err := strconv.Atoi(matches[1]); err == nil {
				numbers = append(numbers, num)
			}
		}
	}

	if len(numbers) == 0 {
		return 1
	}

	sort.Ints(numbers)
	return numbers[len(numbers)-1] + 1
}

// CreateSchemaMigration creates a new migration file for the schema
func CreateSchemaMigration(migrationDir, schemaName string) string {
	// Check if schema migration already exists
	entries, _ := os.ReadDir(migrationDir)
	for _, entry := range entries {
		if strings.Contains(entry.Name(), fmt.Sprintf("create_schema_%s.sql", schemaName)) {
			return "" // Already exists
		}
	}

	nextNum := GetNextMigrationNumber(migrationDir)
	fileName := fmt.Sprintf("%03d_create_schema_%s.sql", nextNum, schemaName)
	filePath := filepath.Join(migrationDir, fileName)

	content := fmt.Sprintf(`-- +migrate Up
CREATE SCHEMA IF NOT EXISTS %s;
`, schemaName)

	WriteFile(filePath, content)
	return filePath
}

// AddSchemaToResetCommand adds a schema drop line to the reset command
func AddSchemaToResetCommand(filePath, schemaName string) error {
	content, _ := os.ReadFile(filePath)
	contentStr := string(content)
	dropLine := fmt.Sprintf("\t\t\tdrop schema %s cascade;", schemaName)

	if strings.Contains(contentStr, dropLine) {
		return nil
	}

	contentStr = strings.Replace(contentStr,
		"\t\t\tdrop schema telegram cascade;",
		fmt.Sprintf("\t\t\tdrop schema telegram cascade;\n%s", dropLine), 1)

	return os.WriteFile(filePath, []byte(contentStr), 0644)
}
