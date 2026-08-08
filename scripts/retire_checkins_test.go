package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestRetireCheckinsTableDefaultsToDryRun(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.Exec("CREATE TABLE checkins (id integer primary key)").Error)
	require.NoError(t, db.Exec("INSERT INTO checkins (id) VALUES (1)").Error)

	result, err := retireCheckinsTable(db, "", false)
	require.NoError(t, err)
	require.True(t, result.TableExists)
	require.Equal(t, int64(1), result.RowCount)
	require.False(t, result.Dropped)
	require.True(t, db.Migrator().HasTable(checkinsTableName))
}

func TestRetireCheckinsTableRequiresBackupBeforeDrop(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.Exec("CREATE TABLE checkins (id integer primary key)").Error)

	_, err = retireCheckinsTable(db, "", true)
	require.Error(t, err)
	require.True(t, db.Migrator().HasTable(checkinsTableName))

	backupPath := filepath.Join(t.TempDir(), "before-checkins-drop.sql")
	require.NoError(t, os.WriteFile(backupPath, []byte("backup complete"), 0o600))
	result, err := retireCheckinsTable(db, backupPath, true)
	require.NoError(t, err)
	require.True(t, result.Dropped)
	require.False(t, db.Migrator().HasTable(checkinsTableName))
}
