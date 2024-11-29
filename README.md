# DB Copy Tool

A command-line tool written in Go that helps you copy tables between different database systems. Currently supports copying from SQLite to PostgreSQL.

## Features

- Copy tables from SQLite to PostgreSQL
- Automatic table creation in destination database
- Schema conversion from SQLite to PostgreSQL
- Batch processing for efficient data transfer
- Create sample SQLite database with test data
- Progress tracking during operations
- Transaction support for data consistency

## Installation

1. Make sure you have Go 1.22 or later installed
2. Clone the repository:
```bash
git clone <repository-url>
cd db-copy
```

3. Build the project using make:
```bash
# Show available make commands
make help

# Build the binary
make build  # or simply: make b

# Clean the build directory
make clean  # or simply: make c

# The binary will be created in the build directory as 'dbcopy'
```

Alternatively, you can install it to your GOPATH:
```bash
make install
```

## Usage

### Creating a Sample Database

To create a sample SQLite database with test data:

```bash
./dbcopy sample [-d sample.db] [-c 1000]
```

Options:
- `-d, --db`: Path to create the SQLite database (default: "sample.db")
- `-c, --count`: Number of sample records to create (default: 1000)

The sample database will contain a `sample_users` table with the following schema:
- `id`: Primary key
- `name`: User's name (varchar)
- `email`: Unique email address (varchar)
- `age`: Age between 20 and 59 (integer)
- `active`: Boolean status
- `created_at`: Timestamp
- `updated_at`: Timestamp

### Copying Tables

To copy a table from SQLite to PostgreSQL:

```bash
./dbcopy copy -s source.db -d "postgres://user:password@localhost:5432/dbname" -t table_name [-b batch_size]
```

Required flags:
- `-s, --source`: Source SQLite database file path
- `-d, --dest`: Destination PostgreSQL connection string
- `-t, --table`: Name of the table to copy

Optional flags:
- `-b, --batch-size`: Number of records to copy in each batch (default: 1000)

Example PostgreSQL connection strings:
- Basic: `postgres://username:password@localhost:5432/dbname`
- With SSL mode: `postgres://username:password@localhost:5432/dbname?sslmode=disable`

## Example Workflow

1. Create a sample SQLite database with 500 records:
```bash
./dbcopy sample -d test.db -c 500
```

2. Copy the sample table to PostgreSQL:
```bash
./dbcopy copy -s test.db -d "postgres://user:password@localhost:5432/dbname" -t "sample_users"
```

## Dependencies

- [GORM](https://gorm.io/): Modern ORM library for Go
- [SQLite Driver](https://github.com/gorm-io/sqlite): GORM SQLite driver
- [PostgreSQL Driver](https://github.com/gorm-io/postgres): GORM PostgreSQL driver
- [Cobra](https://github.com/spf13/cobra): Modern CLI application framework

## Development

The project follows a standard Go project layout:
```
db-copy/
├── cmd/
│   └── dbcopy.go           # dbcopy application entry point
├── internal/
│   ├── cmd/
│   │   └── root.go       # CLI command definitions
│   └── db/
│       ├── db.go         # Database copy functionality
│       └── sample.go     # Sample data generation
└── README.md
```

## Contributing

1. Fork the repository
2. Create your feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add some amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

## License

This project is licensed under the MIT License - see the LICENSE file for details.
