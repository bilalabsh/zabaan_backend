package database

import (
	"log/slog"
	"os"
	"strings"
)

func migrateUsers() {
	createUsersTable()
	ensureAuthColumns()
}

func createUsersTable() {
	_, err := DB.Exec(`
		CREATE TABLE IF NOT EXISTS users (
			id INT AUTO_INCREMENT PRIMARY KEY,
			email VARCHAR(255) UNIQUE NOT NULL,
			username VARCHAR(255) UNIQUE NOT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
		)
	`)
	if err != nil {
		slog.Error("database create users table failed", "component", "database", "err", err)
		os.Exit(1)
	}
}

func ensureAuthColumns() {
	for _, q := range []string{
		"ALTER TABLE users ADD COLUMN first_name VARCHAR(255) DEFAULT ''",
		"ALTER TABLE users ADD COLUMN last_name VARCHAR(255) DEFAULT ''",
		"ALTER TABLE users ADD COLUMN password_hash VARCHAR(255) DEFAULT ''",
		"ALTER TABLE users ADD COLUMN token_valid_after DATETIME DEFAULT NULL",
	} {
		_, err := DB.Exec(q)
		if err != nil && !strings.Contains(err.Error(), "Duplicate column") {
			slog.Warn("database ensureAuthColumns", "component", "database", "query", q, "err", err)
		}
	}
}
