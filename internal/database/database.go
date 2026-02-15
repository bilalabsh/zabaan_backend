package database

import (
	"database/sql"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/bilalabsh/zabaan_backend/internal/config"
	_ "github.com/go-sql-driver/mysql"
)

// DB is the MySQL connection pool
var DB *sql.DB


func Init(cfg *config.Config) {
	if cfg.DatabaseURL == "" {
		slog.Info("database skipped", "component", "database", "reason", "DATABASE_URL not set")
		return
	}
	dsn := cfg.DatabaseURL
	if strings.Contains(dsn, "?") {
		dsn += "&parseTime=true"
	} else {
		dsn += "?parseTime=true"
	}
	var err error
	DB, err = sql.Open("mysql", dsn)
	if err != nil {
		slog.Error("database open failed", "component", "database", "err", err)
		os.Exit(1)
	}
	if err := DB.Ping(); err != nil {
		slog.Error("database ping failed", "component", "database", "err", err)
		os.Exit(1)
	}
	DB.SetConnMaxLifetime(5 * time.Minute)
	DB.SetMaxIdleConns(2)
	createUsersTable()
	ensureAuthColumns()
	slog.Info("database connected", "component", "database")
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

// Close closes the database connection
func Close() {
	if DB != nil {
		if err := DB.Close(); err != nil {
			slog.Warn("database close error", "component", "database", "err", err)
		}
	}
}

