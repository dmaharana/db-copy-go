package db

import (
	"fmt"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// SampleUser represents a user for the sample table
type SampleUser struct {
	ID        uint      `gorm:"primarykey"`
	Name      string    `gorm:"size:255;not null"`
	Email     string    `gorm:"size:255;not null;unique"`
	Age       int       `gorm:"not null"`
	Active    bool      `gorm:"not null;default:true"`
	CreatedAt time.Time `gorm:"not null"`
	UpdatedAt time.Time `gorm:"not null"`
}

// CreateSampleData creates a sample users table with test data
func CreateSampleData(dbPath string, recordCount int) error {
	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}

	// Create the table
	err = db.AutoMigrate(&SampleUser{})
	if err != nil {
		return fmt.Errorf("failed to create table: %w", err)
	}

	// Generate sample users
	users := make([]SampleUser, recordCount)
	for i := 0; i < recordCount; i++ {
		users[i] = SampleUser{
			Name:      fmt.Sprintf("User %d", i+1),
			Email:     fmt.Sprintf("user%d@example.com", i+1),
			Age:       20 + (i % 40), // Ages between 20 and 59
			Active:    i%2 == 0,      // Alternating active status
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}
	}

	// Insert the users in batches
	batchSize := 100
	for i := 0; i < len(users); i += batchSize {
		end := i + batchSize
		if end > len(users) {
			end = len(users)
		}

		batch := users[i:end]
		if err := db.Create(&batch).Error; err != nil {
			return fmt.Errorf("failed to insert batch: %w", err)
		}

		fmt.Printf("Inserted records %d-%d\n", i+1, end)
	}

	fmt.Printf("Successfully created sample table 'sample_users' with %d records\n", recordCount)
	return nil
}
