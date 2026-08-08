package model

import (
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

type legacyTextPricingModel struct {
	Id        int
	ModelName string
	Status    int
}

func TestTextPricingModelMetadataMigratesAcrossSupportedDatabases(t *testing.T) {
	testCases := []struct {
		name   string
		envs   []string
		openDB func(string) (*gorm.DB, error)
		dsn    string
	}{
		{
			name: "sqlite",
			dsn:  fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_")),
			openDB: func(dsn string) (*gorm.DB, error) {
				return gorm.Open(sqlite.Open(dsn), &gorm.Config{})
			},
		},
		{
			name: "mysql",
			envs: []string{
				"TEXT_PRICING_TEST_MYSQL_DSN",
				"CREDITS_TEST_MYSQL_DSN",
			},
			openDB: func(dsn string) (*gorm.DB, error) {
				return gorm.Open(mysql.Open(dsn), &gorm.Config{})
			},
		},
		{
			name: "postgres",
			envs: []string{
				"TEXT_PRICING_TEST_POSTGRES_DSN",
				"CREDITS_TEST_POSTGRES_DSN",
			},
			openDB: func(dsn string) (*gorm.DB, error) {
				return gorm.Open(postgres.Open(dsn), &gorm.Config{})
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			dsn := testCase.dsn
			if len(testCase.envs) > 0 {
				for _, env := range testCase.envs {
					dsn = os.Getenv(env)
					if dsn != "" {
						break
					}
				}
				if dsn == "" {
					t.Skip("no database DSN is configured for this migration test")
				}
			}
			db, err := testCase.openDB(dsn)
			require.NoError(t, err)
			sqlDB, err := db.DB()
			require.NoError(t, err)
			t.Cleanup(func() { _ = sqlDB.Close() })

			tableName := fmt.Sprintf("text_pricing_metadata_%d", time.Now().UnixNano())
			t.Cleanup(func() { _ = db.Migrator().DropTable(tableName) })
			require.NoError(t, db.Table(tableName).AutoMigrate(&legacyTextPricingModel{}))
			require.NoError(t, db.Table(tableName).Create(map[string]any{
				"model_name": "legacy-text-model",
				"status":     1,
			}).Error)

			require.NoError(t, db.Table(tableName).AutoMigrate(&Model{}))
			for _, column := range []string{
				"modal",
				"text_category",
				"official_price_key",
				"pricing_mode",
				"pricing_config",
				"pricing_updated_time",
			} {
				require.True(t, db.Table(tableName).Migrator().HasColumn(&Model{}, column), column)
			}

			var migrated Model
			require.NoError(t, db.Table(tableName).Where("model_name = ?", "legacy-text-model").First(&migrated).Error)
			require.Equal(t, "legacy-text-model", migrated.ModelName)
			require.Equal(t, 1, migrated.Status)
			require.Empty(t, migrated.TextCategory)
			require.Empty(t, migrated.OfficialPriceKey)
		})
	}
}
