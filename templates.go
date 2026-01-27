package evmpunk

import (
	"fmt"
	"strings"
)

// GenerateMainFile generates the main indexer file
func GenerateMainFile(cfg GeneratorConfig) string {
	depsStr := `nil`
	if len(cfg.Dependencies) > 0 {
		quoted := make([]string, len(cfg.Dependencies))
		for i, d := range cfg.Dependencies {
			quoted[i] = fmt.Sprintf("%q", d)
		}
		depsStr = fmt.Sprintf("[]string{%s}", strings.Join(quoted, ", "))
	}

	depsImport := ""
	for _, d := range cfg.Dependencies {
		if d == "preprocessor" || d == "preprocess" {
			depsImport = `	"github.com/taler/indexer/indexers/preprocess"`
		}
	}

	// Generate table names from events
	var tableNames []string
	for _, e := range cfg.Events {
		tableNames = append(tableNames, fmt.Sprintf("%q", ToSnakeCase(e.Name)+"s"))
	}
	tablesStr := "[]string{}"
	if len(tableNames) > 0 {
		tablesStr = fmt.Sprintf("[]string{%s}", strings.Join(tableNames, ", "))
	}

	return fmt.Sprintf(`package %s

import (
	"context"
	"sync"

	"github.com/ethereum/go-ethereum/common"
	"github.com/jackc/pgx/v5"
	"github.com/sirupsen/logrus"

%s
	"github.com/taler/indexer/state"
	"github.com/taler/indexer/telegram"
	"github.com/taler/indexer/types"
)

var log = logrus.WithField("module", "%s-indexer")

const ID = %q

// %s
type Indexer struct {
	logger   *logrus.Entry
	state    *state.Manager
	notifier *telegram.NotificationService

	raw   *types.RawData
	block *types.Block

	resultMu sync.Mutex
	result   *Result

	addressesToWatch []common.Address
}

func New() *Indexer {
	return &Indexer{
		logger: logrus.WithField("module", %q),
	}
}

func (i *Indexer) Init(state *state.Manager, notifier *telegram.NotificationService) {
	i.state = state
	i.notifier = notifier

	if err := i.refreshCache(); err != nil {
		log.WithError(err).Fatal("Failed to refresh cache")
	}
}

func (i *Indexer) Dependencies() []string {
	return %s
}

func (i *Indexer) Rollback(ctx context.Context, tx pgx.Tx, blockNumber int64) error {
	tables := %s

	for _, table := range tables {
		_, err := tx.Exec(ctx, "DELETE FROM %s."+table+" WHERE included_in_block = $1", blockNumber)
		if err != nil {
			return err
		}
	}
	return nil
}

func (i *Indexer) Result() interface{} {
	i.resultMu.Lock()
	defer i.resultMu.Unlock()
	return i.result
}

func (i *Indexer) ID() string {
	return ID
}

func (i *Indexer) Clear() {
	i.raw = nil
	i.block = nil
	i.result = nil
}

func (i *Indexer) Load(raw *types.RawData, deps map[string]interface{}) {
	i.raw = raw
	i.block = deps[preprocess.ID].(*types.Block)

	if err := i.refreshCache(); err != nil {
		log.WithError(err).Error("Failed to refresh cache")
	}
}
`, cfg.PackageName, depsImport, cfg.PackageName, cfg.IndexerID, cfg.Description, cfg.PackageName, depsStr, tablesStr, cfg.SchemaName)
}

// GenerateExecuteFile generates the execute.go file
func GenerateExecuteFile(cfg GeneratorConfig) string {
	// Build result initialization with make() for each event slice
	var resultInits []string
	for _, c := range cfg.Contracts {
		for _, e := range c.Events {
			resultInits = append(resultInits, fmt.Sprintf("\t\t%sEvents: make([]ethtypes.%s%sEvent, 0),", e.Name, c.Name, e.Name))
		}
	}
	resultInitBlock := strings.Join(resultInits, "\n")

	// Parse logic - use IsEvent and Event functions from ethtypes
	var checks []string
	for _, c := range cfg.Contracts {
		for _, e := range c.Events {
			checks = append(checks, fmt.Sprintf(`				if ethtypes.%s.Is%sEvent(log) {
					event, err := ethtypes.%s.%sEvent(log)
					if err != nil {
						return errors.Wrap(err, "%s: could not decode %sEvent")
					}

					i.resultMu.Lock()
					i.result.%sEvents = append(i.result.%sEvents, event)
					i.resultMu.Unlock()
				}`,
				c.Name, e.Name, c.Name, e.Name, cfg.PackageName, e.Name, e.Name, e.Name))
		}
	}

	parseBlock := strings.Join(checks, " else ")

	// Save logic
	var saves []string
	for _, c := range cfg.Contracts {
		for _, e := range c.Events {
			tableName := ToSnakeCase(e.Name) + "s"

			var fields []string
			var pgxArgs []string

			fields = append(fields, "included_in_block", "tx_hash", "tx_index", "log_index", "vault_address", "chain_id")
			pgxArgs = append(pgxArgs,
				`"included_in_block": i.block.Number`,
				`"tx_hash": event.Raw.TxHash.Hex()`,
				`"tx_index": event.Raw.TxIndex`,
				`"log_index": event.Raw.Index`,
				`"vault_address": utils.NormalizeAddress(event.Raw.Address.Hex())`,
				`"chain_id": 1`)

			for _, f := range e.Fields {
				col := ToSnakeCase(f.Name)
				fields = append(fields, col)

				if f.Type == "address" {
					pgxArgs = append(pgxArgs, fmt.Sprintf(`"%s": utils.NormalizeAddress(event.%s.Hex())`, col, Capitalize(f.Name)))
				} else if strings.HasPrefix(f.Type, "uint") || strings.HasPrefix(f.Type, "int") {
					pgxArgs = append(pgxArgs, fmt.Sprintf(`"%s": event.%s.String()`, col, Capitalize(f.Name)))
				} else {
					pgxArgs = append(pgxArgs, fmt.Sprintf(`"%s": event.%s`, col, Capitalize(f.Name)))
				}
			}

			fieldList := "@" + strings.Join(fields, ", @")

			saves = append(saves, fmt.Sprintf(`	for _, event := range i.result.%sEvents {
		_, err := tx.Exec(ctx,
			`+"`INSERT INTO %s.%s (%s) VALUES (%s)`"+`,
			pgx.NamedArgs{
				%s,
			},
		)
		if err != nil {
			return err
		}
	}`, e.Name, cfg.SchemaName, tableName, strings.Join(fields, ", "), fieldList, strings.Join(pgxArgs, ",\n\t\t\t\t")))
		}
	}

	saveBlock := strings.Join(saves, "\n\n")

	return fmt.Sprintf(`package %s

import (
	"context"
	"slices"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/pkg/errors"

	"github.com/taler/indexer/ethtypes"
	"github.com/taler/indexer/utils"
)

func (i *Indexer) Execute(ctx context.Context) error {
	start := time.Now()
	i.logger.Debug("Executing indexer")
	defer func() {
		i.logger.WithField("duration", time.Since(start)).Debug("Indexer execution completed")
	}()

	if i.result != nil {
		return errors.New("%s: indexer already executed or not cleared")
	}

	i.result = &Result{
%s
	}

	for _, tx := range i.block.Txs {
		for _, log := range tx.LogEntries {
			if slices.Contains(i.addressesToWatch, log.Address) {
%s
			}
		}
	}

	return nil
}

func (i *Indexer) SaveToDatabase(ctx context.Context, tx pgx.Tx) error {
	if i.result == nil {
		return nil
	}

%s

	return nil
}
`, cfg.PackageName, cfg.PackageName, resultInitBlock, parseBlock, saveBlock)
}

// GenerateTypesFile generates the types.go file
func GenerateTypesFile(cfg GeneratorConfig) string {
	var fields []string
	for _, c := range cfg.Contracts {
		for _, e := range c.Events {
			fields = append(fields, fmt.Sprintf("\t%sEvents []ethtypes.%s%sEvent", e.Name, c.Name, e.Name))
		}
	}

	fieldsBlock := strings.Join(fields, "\n")
	if fieldsBlock == "" {
		fieldsBlock = "\t// Add event fields"
	}

	return fmt.Sprintf(`package %s

import "github.com/taler/indexer/ethtypes"

type Result struct {
%s
}
`, cfg.PackageName, fieldsBlock)
}

// GenerateCacheFile generates the cache.go file
func GenerateCacheFile(pkg string) string {
	return fmt.Sprintf(`package %s

import (
	"context"

	"github.com/ethereum/go-ethereum/common"
)

func (i *Indexer) refreshCache() error {
	ctx := context.Background()

	rows, err := i.state.DB.Query(ctx, "SELECT address FROM public.vaults WHERE active = true")
	if err != nil {
		return err
	}
	defer rows.Close()

	addresses := []common.Address{}

	for rows.Next() {
		var addr string
		if err := rows.Scan(&addr); err != nil {
			return err
		}
		addresses = append(addresses, common.HexToAddress(addr))
	}

	i.addressesToWatch = addresses
	i.logger.Infof("Loaded %%d addresses", len(addresses))
	return nil
}
`, pkg)
}

// GenerateMigrationForEvent generates a migration file for a single event
func GenerateMigrationForEvent(schema string, event EventInfo) string {
	tableName := ToSnakeCase(event.Name) + "s"

	var cols []string
	cols = append(cols, "    id BIGSERIAL PRIMARY KEY")
	cols = append(cols, "    included_in_block BIGINT NOT NULL")
	cols = append(cols, "    tx_hash TEXT NOT NULL")
	cols = append(cols, "    tx_index INTEGER NOT NULL")
	cols = append(cols, "    log_index INTEGER NOT NULL")
	cols = append(cols, "    vault_address TEXT NOT NULL")
	cols = append(cols, "    chain_id INTEGER NOT NULL")

	for _, f := range event.Fields {
		cols = append(cols, fmt.Sprintf("    %s %s", ToSnakeCase(f.Name), ABITypeToSQL(f.Type)))
	}

	cols = append(cols, "    created_at TIMESTAMP DEFAULT NOW()")

	var indices []string
	indices = append(indices, fmt.Sprintf("CREATE INDEX idx_%s_%s_block ON %s.%s(included_in_block);", schema, tableName, schema, tableName))
	indices = append(indices, fmt.Sprintf("CREATE INDEX idx_%s_%s_tx ON %s.%s(included_in_block, tx_index, log_index);", schema, tableName, schema, tableName))
	indices = append(indices, fmt.Sprintf("CREATE INDEX idx_%s_%s_vault ON %s.%s(vault_address);", schema, tableName, schema, tableName))

	return fmt.Sprintf(`-- +migrate Up
CREATE TABLE IF NOT EXISTS %s.%s (
%s
);

%s

-- +migrate Down
DROP TABLE IF EXISTS %s.%s;
`, schema, tableName, strings.Join(cols, ",\n"), strings.Join(indices, "\n"), schema, tableName)
}

// GenerateReadme generates a README.md file
func GenerateReadme(name, desc string, events []EventInfo) string {
	eventsSection := ""
	if len(events) > 0 {
		var list []string
		for _, e := range events {
			var fields []string
			for _, f := range e.Fields {
				idx := ""
				if f.Indexed {
					idx = " indexed"
				}
				fields = append(fields, fmt.Sprintf("%s %s%s", f.Type, f.Name, idx))
			}
			list = append(list, fmt.Sprintf("- **%s**(%s)", e.Name, strings.Join(fields, ", ")))
		}
		eventsSection = fmt.Sprintf("\n## Events\n\n%s\n", strings.Join(list, "\n"))
	}

	return fmt.Sprintf(`# %s Indexer

%s
%s
## Files

- `+"`%s.go`"+` - Main structure
- `+"`execute.go`"+` - Event parsing & save
- `+"`types.go`"+` - Result types
- `+"`cache.go`"+` - Address cache

## Usage

1. Register in `+"`indexers/register.go`"+`
2. `+"`./indexer migrate`"+`
3. `+"`./indexer scrape queue`"+`
`, strings.Title(name), desc, eventsSection, name)
}
