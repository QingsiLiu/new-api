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

type legacyCreditsTopUp struct {
	Id         int
	UserId     int
	TradeNo    string
	CreateTime int64
	Status     string
}

func TestCreditsSchemaMigratesAcrossSupportedDatabases(t *testing.T) {
	testCases := []struct {
		name   string
		env    string
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
			env:  "CREDITS_TEST_MYSQL_DSN",
			openDB: func(dsn string) (*gorm.DB, error) {
				return gorm.Open(mysql.Open(dsn), &gorm.Config{})
			},
		},
		{
			name: "postgres",
			env:  "CREDITS_TEST_POSTGRES_DSN",
			openDB: func(dsn string) (*gorm.DB, error) {
				return gorm.Open(postgres.Open(dsn), &gorm.Config{})
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			dsn := testCase.dsn
			if testCase.env != "" {
				dsn = os.Getenv(testCase.env)
				if dsn == "" {
					t.Skipf("%s is not configured", testCase.env)
				}
			}
			db, err := testCase.openDB(dsn)
			require.NoError(t, err)
			sqlDB, err := db.DB()
			require.NoError(t, err)
			t.Cleanup(func() { _ = sqlDB.Close() })

			prefix := fmt.Sprintf("credits_v1_migration_%d_", time.Now().UnixNano())
			topUpsTable := prefix + "top_ups"
			grantsTable := prefix + "credit_grants"
			eventsTable := prefix + "payment_events"
			t.Cleanup(func() {
				_ = db.Migrator().DropTable(eventsTable, grantsTable, topUpsTable)
			})

			require.NoError(t, db.Table(topUpsTable).AutoMigrate(&legacyCreditsTopUp{}))
			require.NoError(t, db.Table(topUpsTable).Create(map[string]any{
				"user_id":     101,
				"trade_no":    "legacy-order",
				"create_time": 1_700_000_000,
				"status":      "success",
			}).Error)

			require.NoError(t, db.Table(topUpsTable).AutoMigrate(&TopUp{}))
			require.NoError(t, db.Table(grantsTable).AutoMigrate(&CreditGrant{}))
			require.NoError(t, db.Table(eventsTable).AutoMigrate(&PaymentEvent{}))

			for _, column := range []string{
				"package_id",
				"credits",
				"quota_amount",
				"currency",
				"price_minor",
				"base_credits",
				"bonus_credits",
				"payment_store_id",
				"payment_environment",
			} {
				require.True(t, db.Table(topUpsTable).Migrator().HasColumn(&TopUp{}, column), column)
			}

			var legacy TopUp
			require.NoError(t, db.Table(topUpsTable).Where("trade_no = ?", "legacy-order").First(&legacy).Error)
			require.Equal(t, 101, legacy.UserId)
			require.Equal(t, "success", legacy.Status)
			require.Empty(t, legacy.PackageId)
			require.Zero(t, legacy.QuotaAmount)

			grant := CreditGrant{
				UserId:       101,
				GrantType:    SignupCreditGrantType,
				Source:       "email_verified",
				IdentityHash: signupCreditIdentityHash("migration@example.com"),
				Credits:      20,
				Quota:        72_000,
				CreatedAt:    time.Now().Unix(),
			}
			require.NoError(t, db.Table(grantsTable).Create(&grant).Error)
			duplicateGrant := grant
			duplicateGrant.Id = 0
			require.Error(t, db.Table(grantsTable).Create(&duplicateGrant).Error)

			event := PaymentEvent{
				Provider:  PaymentProviderWaffoPancake,
				EventId:   "evt-migration",
				TradeNo:   "legacy-order",
				CreatedAt: time.Now().Unix(),
			}
			require.NoError(t, db.Table(eventsTable).Create(&event).Error)
			duplicateEvent := event
			duplicateEvent.Id = 0
			require.Error(t, db.Table(eventsTable).Create(&duplicateEvent).Error)
		})
	}
}
