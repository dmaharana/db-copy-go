package cmd

import (
	"db-copy/internal/db"
	"github.com/spf13/cobra"
)

var (
	sourceDB     string
	destDB       string
	tableName    string
	batchSize    int
	recordCount  int
	sampleDBPath string
)

// RootCmd represents the base command when called without any subcommands
var RootCmd = &cobra.Command{
	Use:   "db-copy",
	Short: "A CLI tool to copy tables between different databases",
	Long: `db-copy allows you to copy tables between different database types.
Currently supports copying from SQLite to PostgreSQL.`,
}

// copyCmd represents the copy command
var copyCmd = &cobra.Command{
	Use:   "copy",
	Short: "Copy a table from source to destination database",
	RunE:  runCopy,
}

// sampleCmd represents the sample command
var sampleCmd = &cobra.Command{
	Use:   "sample",
	Short: "Create a sample SQLite database with test data",
	Long: `Creates a new SQLite database with a sample 'sample_users' table.
The table includes fields like ID, Name, Email, Age, Active status, and timestamps.`,
	RunE: runSample,
}

func init() {
	// Copy command flags
	copyCmd.Flags().StringVarP(&sourceDB, "source", "s", "", "Source database connection string (SQLite)")
	copyCmd.Flags().StringVarP(&destDB, "dest", "d", "", "Destination database connection string (PostgreSQL)")
	copyCmd.Flags().StringVarP(&tableName, "table", "t", "", "Table name to copy")
	copyCmd.Flags().IntVarP(&batchSize, "batch-size", "b", 1000, "Batch size for copying records")

	copyCmd.MarkFlagRequired("source")
	copyCmd.MarkFlagRequired("dest")
	copyCmd.MarkFlagRequired("table")

	// Sample command flags
	sampleCmd.Flags().StringVarP(&sampleDBPath, "db", "d", "sample.db", "Path to create the sample SQLite database")
	sampleCmd.Flags().IntVarP(&recordCount, "count", "c", 1000, "Number of sample records to create")

	RootCmd.AddCommand(copyCmd)
	RootCmd.AddCommand(sampleCmd)
}

func runCopy(cmd *cobra.Command, args []string) error {
	copier := db.NewCopier(sourceDB, destDB, tableName, batchSize)

	if err := copier.Connect(); err != nil {
		return err
	}

	if err := copier.Copy(); err != nil {
		return err
	}

	return nil
}

func runSample(cmd *cobra.Command, args []string) error {
	return db.CreateSampleData(sampleDBPath, recordCount)
}
