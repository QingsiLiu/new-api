package model

import (
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	miniredis "github.com/alicebob/miniredis/v2"
	"github.com/glebarez/sqlite"
	"github.com/go-redis/redis/v8"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupUserAffTransferDB(t *testing.T) *gorm.DB {
	t.Helper()
	previousDB := DB
	previousType := common.MainDatabaseType()
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared&_busy_timeout=5000", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	DB = db
	common.SetMainDatabaseType(common.DatabaseTypeSQLite)
	require.NoError(t, db.AutoMigrate(&User{}))
	t.Cleanup(func() {
		DB = previousDB
		common.SetMainDatabaseType(previousType)
		if sqlDB, err := db.DB(); err == nil {
			_ = sqlDB.Close()
		}
	})
	return db
}

func TestTransferAllAffQuotaToQuotaMovesExactBalance(t *testing.T) {
	db := setupUserAffTransferDB(t)
	user := User{Username: "aff-all", Password: "hash", Status: common.UserStatusEnabled, Quota: 275000, AffQuota: 333333}
	require.NoError(t, db.Create(&user).Error)

	result, err := TransferAllAffQuotaToQuota(user.Id)
	require.NoError(t, err)
	require.Equal(t, 333333, result.TransferredQuota)
	require.Equal(t, 608333, result.BalanceQuota)
	require.Zero(t, result.AffBalanceQuota)

	var stored User
	require.NoError(t, db.First(&stored, user.Id).Error)
	require.Equal(t, 608333, stored.Quota)
	require.Zero(t, stored.AffQuota)

	_, err = TransferAllAffQuotaToQuota(user.Id)
	require.ErrorIs(t, err, ErrAffQuotaBelowMinimum)
	require.NoError(t, db.First(&stored, user.Id).Error)
	require.Equal(t, 608333, stored.Quota)
	require.Zero(t, stored.AffQuota)
}

func TestTransferAllAffQuotaToQuotaRejectsBelowOneCNY(t *testing.T) {
	db := setupUserAffTransferDB(t)
	user := User{Username: "aff-small", Password: "hash", Status: common.UserStatusEnabled, Quota: 7, AffQuota: common.CNYToQuota(1) - 1}
	require.NoError(t, db.Create(&user).Error)

	_, err := TransferAllAffQuotaToQuota(user.Id)
	require.ErrorIs(t, err, ErrAffQuotaBelowMinimum)

	var stored User
	require.NoError(t, db.First(&stored, user.Id).Error)
	require.Equal(t, 7, stored.Quota)
	require.Equal(t, common.CNYToQuota(1)-1, stored.AffQuota)
}

func TestTransferAllAffQuotaToQuotaConcurrentSingleWinner(t *testing.T) {
	db := setupUserAffTransferDB(t)
	const initialQuota = 125000
	const reward = 350000
	user := User{Username: "aff-race", Password: "hash", Status: common.UserStatusEnabled, Quota: initialQuota, AffQuota: reward}
	require.NoError(t, db.Create(&user).Error)

	const callers = 5
	var wg sync.WaitGroup
	results := make(chan AffTransferResult, callers)
	for range callers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if result, err := TransferAllAffQuotaToQuota(user.Id); err == nil {
				results <- result
			}
		}()
	}
	wg.Wait()
	close(results)

	positive := 0
	for result := range results {
		if result.TransferredQuota > 0 {
			positive++
		}
	}
	require.Equal(t, 1, positive)

	var stored User
	require.NoError(t, db.First(&stored, user.Id).Error)
	require.Equal(t, initialQuota+reward, stored.Quota)
	require.Zero(t, stored.AffQuota)
	require.Equal(t, initialQuota+reward, stored.Quota+stored.AffQuota)
}

func TestTransferAllAffQuotaToQuotaInvalidatesCachedBalance(t *testing.T) {
	db := setupUserAffTransferDB(t)
	user := User{Username: "aff-cache", Password: "hash", Status: common.UserStatusEnabled, Quota: 125000, AffQuota: 250000}
	require.NoError(t, db.Create(&user).Error)

	server := miniredis.RunT(t)
	previousRDB := common.RDB
	previousRedisEnabled := common.RedisEnabled
	common.RDB = redis.NewClient(&redis.Options{Addr: server.Addr()})
	common.RedisEnabled = true
	t.Cleanup(func() {
		_ = common.RDB.Close()
		common.RDB = previousRDB
		common.RedisEnabled = previousRedisEnabled
	})
	require.NoError(t, populateUserCache(user))

	cachedBefore, err := GetUserQuota(user.Id, false)
	require.NoError(t, err)
	require.Equal(t, 125000, cachedBefore)

	_, err = TransferAllAffQuotaToQuota(user.Id)
	require.NoError(t, err)
	cachedAfter, err := GetUserQuota(user.Id, false)
	require.NoError(t, err)
	require.Equal(t, 375000, cachedAfter)
	require.Eventually(t, func() bool {
		cached, err := getUserQuotaCache(user.Id)
		return err == nil && cached == 375000
	}, time.Second, 10*time.Millisecond)
}
