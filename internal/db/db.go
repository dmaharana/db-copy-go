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

	switch c.sourceDBType {
	case DBTypeSQLite:
		// SQLite schema query
		rows, err := c.sourceConn.Raw(fmt.Sprintf("PRAGMA table_info(%s)", c.TableName)).Rows()
		if err != nil {
			return nil, fmt.Errorf("failed to get table schema: %w", err)
		}
		defer rows.Close()

		for rows.Next() {
			var cid int
			var name, type_ string
			var notnull, pk int
			var dflt_value interface{}
			if err := rows.Scan(&cid, &name, &type_, &notnull, &dflt_value, &pk); err != nil {
				return nil, err
			}

			columns = append(columns, Column{
				Name:       name,
				Type:       c.convertDataType(type_, c.sourceDBType, c.destDBType),
				IsNullable: notnull == 0,
				IsPrimary:  pk == 1,
			})
		}

	case DBTypePostgres:
		// PostgreSQL schema query
		query := `
			SELECT column_name, data_type, 
				   CASE WHEN is_nullable = 'YES' THEN true ELSE false END as is_nullable,
				   CASE WHEN constraint_type = 'PRIMARY KEY' THEN true ELSE false END as is_primary
			FROM information_schema.columns c
			LEFT JOIN (
				SELECT kcu.column_name, tc.constraint_type
				FROM information_schema.table_constraints tc
				JOIN information_schema.key_column_usage kcu
					ON tc.constraint_name = kcu.constraint_name
				WHERE tc.table_name = ? AND tc.constraint_type = 'PRIMARY KEY'
			) pk ON c.column_name = pk.column_name
			WHERE c.table_name = ?
			ORDER BY ordinal_position;
		`
		rows, err := c.sourceConn.Raw(query, c.TableName, c.TableName).Rows()
		if err != nil {
			return nil, fmt.Errorf("failed to get table schema: %w", err)
		}
		defer rows.Close()

		for rows.Next() {
			var name, dataType string
			var isNullable, isPrimary bool
			if err := rows.Scan(&name, &dataType, &isNullable, &isPrimary); err != nil {
				return nil, err
			}

			columns = append(columns, Column{
				Name:       name,
				Type:       c.convertDataType(dataType, c.sourceDBType, c.destDBType),
				IsNullable: isNullable,
				IsPrimary:  isPrimary,
			})
		}
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

	// Build CREATE TABLE statement based on destination database type
	var columnDefs []string
	for _, col := range columns {
		var def string
		switch c.destDBType {
		case DBTypePostgres:
			def = fmt.Sprintf("%s %s", col.Name, col.Type)
			if col.IsPrimary {
				def += " PRIMARY KEY"
			}
			if !col.IsNullable {
				def += " NOT NULL"
			}
		case DBTypeSQLite:
			def = fmt.Sprintf("%s %s", col.Name, col.Type)
			if col.IsPrimary {
				def += " PRIMARY KEY"
			}
			if !col.IsNullable {
				def += " NOT NULL"
			}
		}
		columnDefs = append(columnDefs, def)
	}

	createTableSQL := fmt.Sprintf("CREATE TABLE %s (\n  %s\n);",
		c.TableName,
		strings.Join(columnDefs, ",\n  "),
	)

	// Create table
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

	// Get the data from source table
	var records []map[string]interface{}
	result := c.sourceConn.Table(c.TableName).Find(&records)
	if result.Error != nil {
		return fmt.Errorf("failed to read from source table: %w", result.Error)
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
