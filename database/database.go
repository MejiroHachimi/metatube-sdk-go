package database

import (
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

const Sqlite = "sqlite"

type Config struct {
	// DSN the Data Source Name.
	DSN string

	// Disable automatic ping.
	DisableAutomaticPing bool

	// Prepared statement.
	PreparedStmt bool

	// Max DB open connections.
	MaxOpenConns int

	// Max DB idle connections.
	MaxIdleConns int

	LogLevel logger.LogLevel
}

func (cfg *Config) applyDefaults() {
	if cfg.DSN == "" {
		// use sqlite DB memory mode by default.
		cfg.DSN = "file::memory:?cache=shared"
	}

	if cfg.MaxIdleConns <= 0 {
		// golang's default.
		cfg.MaxIdleConns = 2
	}

	if cfg.LogLevel < logger.Silent ||
		cfg.LogLevel > logger.Info {
		// INFO by default.
		cfg.LogLevel = logger.Info
	}
}

func Open(cfg *Config) (*gorm.DB, error) {
	cfg.applyDefaults()

	// Reject old remote database configuration instead of creating a local file
	// whose name accidentally contains a connection string.
	if (strings.Contains(cfg.DSN, "://") && !strings.HasPrefix(cfg.DSN, "file:")) || strings.Contains(cfg.DSN, "host=") || strings.Contains(cfg.DSN, "dbname=") {
		return nil, fmt.Errorf("only SQLite databases are supported; use a local path or file: URI")
	}

	db, err := gorm.Open(sqlite.Open(cfg.DSN), &gorm.Config{
		Logger: logger.New(
			log.New(os.Stdout, "[GORM]\u0020", log.LstdFlags),
			logger.Config{
				SlowThreshold:             100 * time.Millisecond,
				LogLevel:                  cfg.LogLevel,
				IgnoreRecordNotFoundError: false,
				ParameterizedQueries:      false,
				Colorful:                  false,
			}),
		PrepareStmt:          cfg.PreparedStmt,
		DisableAutomaticPing: cfg.DisableAutomaticPing,
	})
	if err != nil {
		return nil, err
	}

	if sqlDB, err := db.DB(); err == nil /* ignore error */ {
		sqlDB.SetMaxIdleConns(cfg.MaxIdleConns)
		sqlDB.SetMaxOpenConns(cfg.MaxOpenConns)
	}
	return db, nil
}
