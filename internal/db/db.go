package db

import (
	"fmt"
	"strings"

	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// Copier handles database copy operations
type Copier struct {
	SourceDB      string
	DestDB        string
	TableName     string
	BatchSize     int
	sourceConn    *gorm.DB
	destConn      *gorm.DB
}

// NewCopier creates a new instance of Copier
func NewCopier(sourceDB, destDB, tableName string, batchSize int) *Copier {
	return &Copier{
		SourceDB:  sourceDB,
		DestDB:    destDB,
		TableName: tableName,
		BatchSize: batchSize,
	}
}

// Connect establishes connections to both source and destination databases
func (c *Copier) Connect() error {
	var err error
	
	// Connect to source database (SQLite)
	c.sourceConn, err = gorm.Open(sqlite.Open(c.SourceDB), &gorm.Config{})
	if err != nil {
		return fmt.Errorf("failed to connect to source database: %w", err)
	}

	// Connect to destination database (PostgreSQL)
	c.destConn, err = gorm.Open(postgres.Open(c.DestDB), &gorm.Config{})
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
			Type:       convertSQLiteTypeToPostgres(type_),
			IsNullable: notnull == 0,
			IsPrimary:  pk == 1,
		})
	}

	return columns, nil
}

// convertSQLiteTypeToPostgres converts SQLite data types to PostgreSQL equivalents
func convertSQLiteTypeToPostgres(sqliteType string) string {
	sqliteType = strings.ToUpper(sqliteType)
	
	switch {
	case strings.Contains(sqliteType, "INTEGER"):
		return "INTEGER"
	case strings.Contains(sqliteType, "REAL"):
		return "DOUBLE PRECISION"
	case strings.Contains(sqliteType, "TEXT"):
		return "TEXT"
	case strings.Contains(sqliteType, "BLOB"):
		return "BYTEA"
	case strings.Contains(sqliteType, "BOOLEAN"):
		return "BOOLEAN"
	case strings.Contains(sqliteType, "DATETIME"):
		return "TIMESTAMP"
	case strings.Contains(sqliteType, "NUMERIC"):
		return "NUMERIC"
	default:
		return "TEXT"
	}
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

	// Build CREATE TABLE statement
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
