package db

import (
	"fmt"
	"strings"

	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// DBType represents the type of database
type DBType int

const (
	DBTypeSQLite DBType = iota
	DBTypePostgres
)

// Copier handles database copy operations
type Copier struct {
	SourceDB      string
	DestDB        string
	TableName     string
	BatchSize     int
	sourceConn    *gorm.DB
	destConn      *gorm.DB
	sourceDBType  DBType
	destDBType    DBType
}

// NewCopier creates a new instance of Copier
func NewCopier(sourceDB, destDB, tableName string, batchSize int) *Copier {
	c := &Copier{
		SourceDB:  sourceDB,
		DestDB:    destDB,
		TableName: tableName,
		BatchSize: batchSize,
	}

	// Determine source database type
	if strings.HasPrefix(sourceDB, "postgres://") {
		c.sourceDBType = DBTypePostgres
	} else {
		c.sourceDBType = DBTypeSQLite
	}

	// Determine destination database type
	if strings.HasPrefix(destDB, "postgres://") {
		c.destDBType = DBTypePostgres
	} else {
		c.destDBType = DBTypeSQLite
	}

	return c
}

// Connect establishes connections to both source and destination databases
func (c *Copier) Connect() error {
	var err error

	// Connect to source database
	switch c.sourceDBType {
	case DBTypePostgres:
		c.sourceConn, err = gorm.Open(postgres.Open(c.SourceDB), &gorm.Config{})
	case DBTypeSQLite:
		c.sourceConn, err = gorm.Open(sqlite.Open(c.SourceDB), &gorm.Config{})
	}
	if err != nil {
		return fmt.Errorf("failed to connect to source database: %w", err)
	}

	// Connect to destination database
	switch c.destDBType {
	case DBTypePostgres:
		c.destConn, err = gorm.Open(postgres.Open(c.DestDB), &gorm.Config{})
	case DBTypeSQLite:
		c.destConn, err = gorm.Open(sqlite.Open(c.DestDB), &gorm.Config{})
	}
	if err != nil {
		return fmt.Errorf("failed to connect to destination database: %w", err)
	}

	return nil
}

// Column represents a database column with its properties
type Column struct {
	Name       string
	Type       string
	IsNullable bool
	IsPrimary  bool
}

// getSourceSchema retrieves the table schema from the source database
func (c *Copier) getSourceSchema() ([]Column, error) {
	var columns []Column

	// Get table schema using GORM's Migrator
	columnTypes, err := c.sourceConn.Migrator().ColumnTypes(c.TableName)
	if err != nil {
		return nil, fmt.Errorf("failed to get column types: %w", err)
	}

	// Get primary key information using raw SQL based on database type
	var primaryKeys []string
	switch c.sourceDBType {
	case DBTypeSQLite:
		var pks []struct {
			Name string
			Pk   int
		}
		if err := c.sourceConn.Raw("SELECT name, pk FROM pragma_table_info(?) WHERE pk > 0", c.TableName).Scan(&pks).Error; err != nil {
			return nil, fmt.Errorf("failed to get primary keys: %w", err)
		}
		for _, pk := range pks {
			primaryKeys = append(primaryKeys, pk.Name)
		}
	case DBTypePostgres:
		if err := c.sourceConn.Raw(`
			SELECT a.attname
			FROM pg_index i
			JOIN pg_attribute a ON a.attrelid = i.indrelid AND a.attnum = ANY(i.indkey)
			WHERE i.indrelid = ?::regclass AND i.indisprimary
		`, c.TableName).Scan(&primaryKeys).Error; err != nil {
			return nil, fmt.Errorf("failed to get primary keys: %w", err)
		}
	}

	// Create a map of primary key columns for easier lookup
	pkMap := make(map[string]bool)
	for _, pk := range primaryKeys {
		pkMap[pk] = true
	}

	// Convert column information to our Column type
	for _, col := range columnTypes {
		nullable, ok := col.Nullable()
		if !ok {
			// If we can't determine nullability, assume it's nullable
			nullable = true
		}

		// Get the database type name
		dbTypeName := col.DatabaseTypeName()

		columns = append(columns, Column{
			Name:       col.Name(),
			Type:       c.convertDataType(dbTypeName, c.sourceDBType, c.destDBType),
			IsNullable: nullable,
			IsPrimary:  pkMap[col.Name()],
		})
	}

	return columns, nil
}

// convertDataType converts data types between different databases
func (c *Copier) convertDataType(sourceType string, fromDB, toDB DBType) string {
	sourceType = strings.ToUpper(sourceType)

	// If source and destination are the same type, no conversion needed
	if fromDB == toDB {
		return sourceType
	}

	switch fromDB {
	case DBTypeSQLite:
		// Convert SQLite to PostgreSQL
		switch {
		case strings.Contains(sourceType, "INTEGER"):
			return "INTEGER"
		case strings.Contains(sourceType, "REAL"):
			return "DOUBLE PRECISION"
		case strings.Contains(sourceType, "TEXT"):
			return "TEXT"
		case strings.Contains(sourceType, "BLOB"):
			return "BYTEA"
		case strings.Contains(sourceType, "BOOLEAN"):
			return "BOOLEAN"
		case strings.Contains(sourceType, "DATETIME"):
			return "TIMESTAMP"
		case strings.Contains(sourceType, "NUMERIC"):
			return "NUMERIC"
		default:
			return "TEXT"
		}

	case DBTypePostgres:
		// Convert PostgreSQL to SQLite
		switch sourceType {
		case "BIGINT", "INTEGER", "SMALLINT":
			return "INTEGER"
		case "DOUBLE PRECISION", "REAL", "NUMERIC", "DECIMAL":
			return "REAL"
		case "TEXT", "VARCHAR", "CHAR", "CHARACTER VARYING":
			return "TEXT"
		case "BYTEA":
			return "BLOB"
		case "BOOLEAN":
			return "BOOLEAN"
		case "TIMESTAMP", "TIMESTAMP WITHOUT TIME ZONE", "TIMESTAMP WITH TIME ZONE":
			return "DATETIME"
		default:
			return "TEXT"
		}
	}

	return "TEXT" // Default fallback
}

// ensureTableExists creates the table in the destination database if it doesn't exist
func (c *Copier) ensureTableExists() error {
	// Check if table exists using GORM's migrator
	if c.destConn.Migrator().HasTable(c.TableName) {
		return nil
	}

	// Get schema from source
	columns, err := c.getSourceSchema()
	if err != nil {
		return fmt.Errorf("failed to get source table schema: %w", err)
	}

	// Create table definition
	var columnDefs []string
	for _, col := range columns {
		def := fmt.Sprintf("%s %s", col.Name, col.Type)
		if col.IsPrimary {
			def += " PRIMARY KEY"
		}
		if !col.IsNullable {
			def += " NOT NULL"
		}
		columnDefs = append(columnDefs, def)
	}

	// Create table using SQL
	createTableSQL := fmt.Sprintf("CREATE TABLE %s (\n  %s\n);",
		c.TableName,
		strings.Join(columnDefs, ",\n  "),
	)

	if err := c.destConn.Exec(createTableSQL).Error; err != nil {
		return fmt.Errorf("failed to create table: %w", err)
	}

	fmt.Printf("Created table '%s' in destination database\n", c.TableName)
	return nil
}

// Copy performs the actual data copy operation
func (c *Copier) Copy() error {
	// Ensure destination table exists with correct schema
	if err := c.ensureTableExists(); err != nil {
		return err
	}

	// Get the data from source table using GORM
	var records []map[string]interface{}
	if err := c.sourceConn.Table(c.TableName).Find(&records).Error; err != nil {
		return fmt.Errorf("failed to read from source table: %w", err)
	}

	// Begin transaction in destination database
	tx := c.destConn.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// Copy data in batches
	totalRecords := len(records)
	for i := 0; i < totalRecords; i += c.BatchSize {
		end := i + c.BatchSize
		if end > totalRecords {
			end = totalRecords
		}

		batch := records[i:end]
		if err := tx.Table(c.TableName).Create(&batch).Error; err != nil {
			tx.Rollback()
			return fmt.Errorf("failed to insert batch into destination table: %w", err)
		}

		fmt.Printf("Copied %d/%d records\n", end, totalRecords)
	}

	if err := tx.Commit().Error; err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	fmt.Printf("Successfully copied %d records from %s to destination database\n", totalRecords, c.TableName)
	return nil
}
