package model

import (
	"fmt"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupFunnelTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	common.SetMainDatabaseType(common.DatabaseTypeSQLite)
	common.RedisEnabled = false
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	DB = db
	require.NoError(t, db.AutoMigrate(&Option{}, &User{}, &TopUp{}, &Task{}, &FunnelVisitor{}, &FunnelEvent{}, &FunnelActivityDay{}))
	t.Cleanup(func() {
		sqlDB, err := db.DB()
		if err == nil {
			_ = sqlDB.Close()
		}
	})
	return db
}

func TestFunnelSchemaHasOnlyBoundedColumnsAndUniqueKeys(t *testing.T) {
	db := setupFunnelTestDB(t)
	hash := strings.Repeat("a", 64)
	visitor := FunnelVisitor{Environment: FunnelEnvironmentProduction, VisitorHMAC: &hash, IdentityState: FunnelIdentityAnonymous, FirstSeenAt: 10, LastSeenAt: 10}
	require.NoError(t, db.Create(&visitor).Error)
	duplicateVisitor := FunnelVisitor{Environment: FunnelEnvironmentProduction, VisitorHMAC: &hash, IdentityState: FunnelIdentityAnonymous}
	require.Error(t, db.Create(&duplicateVisitor).Error)
	require.NoError(t, db.Create(&FunnelVisitor{Environment: FunnelEnvironmentProduction, IdentityState: FunnelIdentityAnonymous}).Error)
	require.NoError(t, db.Create(&FunnelVisitor{Environment: FunnelEnvironmentProduction, IdentityState: FunnelIdentityAnonymous}).Error)

	event := FunnelEvent{Environment: FunnelEnvironmentProduction, EventID: "7dfb2d2c-7f40-4f39-b8f4-5fb27db06041", VisitorID: visitor.ID, EventName: FunnelEventSLPView, EventVersion: 1, ReceivedAt: 10, Locale: "zh", ModelSlug: "gpt-image-2"}
	require.NoError(t, db.Create(&event).Error)
	duplicateEvent := event
	duplicateEvent.ID = 0
	require.Error(t, db.Create(&duplicateEvent).Error)

	day := FunnelActivityDay{Environment: FunnelEnvironmentProduction, UserID: 7, ActivityDate: 86400, FirstSeenAt: 90000, LastSeenAt: 90000}
	require.NoError(t, db.Create(&day).Error)
	duplicateDay := day
	duplicateDay.ID = 0
	require.Error(t, db.Create(&duplicateDay).Error)

	for _, forbidden := range []string{"ip", "user_agent", "referer", "email", "username", "prompt", "url", "error_message", "metadata"} {
		require.False(t, db.Migrator().HasColumn(&FunnelEvent{}, forbidden), forbidden)
	}
}
