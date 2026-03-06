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
	migrateUsers()
	slog.Info("database connected", "component", "database")
}

// Close closes the database connection
func Close() {
	if DB != nil {
		if err := DB.Close(); err != nil {
			slog.Warn("database close error", "component", "database", "err", err)
		}
	}
}
