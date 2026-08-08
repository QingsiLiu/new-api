package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/glebarez/sqlite"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

const checkinsTableName = "checkins"

type retireCheckinsResult struct {
	TableExists bool
	RowCount    int64
	Dropped     bool
}

func main() {
	driver := flag.String("driver", "", "database driver: sqlite, mysql, or postgres")
	dsn := flag.String("dsn", "", "database DSN")
	backup := flag.String("backup", "", "existing backup file required with --confirm")
	confirm := flag.Bool("confirm", false, "drop the checkins table after the dry-run checks pass")
	flag.Parse()

	result, err := runRetireCheckins(*driver, *dsn, *backup, *confirm)
	if err != nil {
		fmt.Fprintln(os.Stderr, "checkins retirement failed")
		os.Exit(1)
	}
	if !result.TableExists {
		fmt.Println("checkins table does not exist; no action taken")
		return
	}
	fmt.Printf("checkins table exists; rows=%d\n", result.RowCount)
	if !*confirm {
		fmt.Println("dry run complete; no schema changed. Re-run with --confirm and an existing --backup file to drop the table.")
		return
	}
	if result.Dropped {
		fmt.Println("checkins table dropped after confirming the supplied backup file exists")
	}
}

func runRetireCheckins(driver, dsn, backup string, confirm bool) (retireCheckinsResult, error) {
	if strings.TrimSpace(driver) == "" || strings.TrimSpace(dsn) == "" {
		return retireCheckinsResult{}, errors.New("driver and dsn are required")
	}
	db, err := openRetireCheckinsDB(driver, dsn)
	if err != nil {
		return retireCheckinsResult{}, errors.New("database connection failed")
	}
	return retireCheckinsTable(db, backup, confirm)
}

func openRetireCheckinsDB(driver, dsn string) (*gorm.DB, error) {
	switch strings.ToLower(strings.TrimSpace(driver)) {
	case "sqlite":
		return gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	case "mysql":
		return gorm.Open(mysql.Open(dsn), &gorm.Config{})
	case "postgres", "postgresql":
		return gorm.Open(postgres.New(postgres.Config{
			DSN:                  dsn,
			PreferSimpleProtocol: true,
		}), &gorm.Config{})
	default:
		return nil, errors.New("unsupported database driver")
	}
}

func retireCheckinsTable(db *gorm.DB, backup string, confirm bool) (retireCheckinsResult, error) {
	if db == nil {
		return retireCheckinsResult{}, errors.New("database is required")
	}
	result := retireCheckinsResult{TableExists: db.Migrator().HasTable(checkinsTableName)}
	if !result.TableExists {
		return result, nil
	}
	if err := db.Table(checkinsTableName).Count(&result.RowCount).Error; err != nil {
		return retireCheckinsResult{}, errors.New("could not count checkins rows")
	}
	if !confirm {
		return result, nil
	}
	if err := validateRetireCheckinsBackup(backup); err != nil {
		return retireCheckinsResult{}, err
	}
	if err := db.Migrator().DropTable(checkinsTableName); err != nil {
		return retireCheckinsResult{}, errors.New("could not drop checkins table")
	}
	if db.Migrator().HasTable(checkinsTableName) {
		return retireCheckinsResult{}, errors.New("checkins table still exists after drop")
	}
	result.Dropped = true
	return result, nil
}

func validateRetireCheckinsBackup(path string) error {
	if strings.TrimSpace(path) == "" {
		return errors.New("backup is required when confirm is set")
	}
	info, err := os.Stat(path)
	if err != nil || info.IsDir() {
		return errors.New("backup file is not available")
	}
	return nil
}
