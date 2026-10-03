package database

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"gorm.io/gorm/logger"
)

func TestSQLitePersistsAcrossReopen(t *testing.T) {
	path := filepath.Join(t.TempDir(), "metadata with spaces.db")
	db, err := Open(&Config{DSN: path, LogLevel: logger.Silent})
	require.NoError(t, err)
	require.Equal(t, Sqlite, db.Name())
	require.NoError(t, db.Exec("CREATE TABLE sample (value TEXT)").Error)
	require.NoError(t, db.Exec("INSERT INTO sample VALUES (?)", "persisted").Error)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	require.NoError(t, sqlDB.Close())
	db, err = Open(&Config{DSN: path, PreparedStmt: true, MaxOpenConns: 1})
	require.NoError(t, err)
	sqlDB, err = db.DB()
	require.NoError(t, err)
	defer sqlDB.Close()
	var value string
	require.NoError(t, db.Raw("SELECT value FROM sample").Scan(&value).Error)
	require.Equal(t, "persisted", value)
}

func TestOnlySQLiteConfigurationAccepted(t *testing.T) {
	for _, dsn := range []string{"postgres://localhost/db", "postgresql://localhost/db", "host=localhost dbname=test", "mysql://localhost/db"} {
		db, err := Open(&Config{DSN: dsn})
		require.ErrorContains(t, err, "only SQLite")
		require.Nil(t, db)
	}
	for _, dsn := range []string{"", "file::memory:?cache=shared", "file://" + filepath.Join(t.TempDir(), "uri.db")} {
		db, err := Open(&Config{DSN: dsn, LogLevel: -1})
		require.NoError(t, err)
		sqlDB, err := db.DB()
		require.NoError(t, err)
		require.NoError(t, sqlDB.Ping())
		require.NoError(t, sqlDB.Close())
	}
	_, err := Open(&Config{DSN: filepath.Join(t.TempDir(), "missing", "metadata.db")})
	require.Error(t, err)
}
