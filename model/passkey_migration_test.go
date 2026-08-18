package model

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

type legacyPasskeyCredential struct {
	ID           int    `gorm:"primaryKey"`
	UserID       int    `gorm:"column:user_id;uniqueIndex:idx_passkey_credentials_user_id;not null"`
	RPID         string `gorm:"column:rp_id;type:varchar(255)"`
	Origin       string `gorm:"type:varchar(512)"`
	CredentialID string `gorm:"type:varchar(512);uniqueIndex;not null"`
	PublicKey    string `gorm:"type:text;not null"`
	CreatedAt    time.Time
	UpdatedAt    time.Time
	DeletedAt    gorm.DeletedAt `gorm:"index"`
}

func (legacyPasskeyCredential) TableName() string { return "passkey_credentials" }

func TestMigratePasskeyCredentialUserIndexIsSQLiteIdempotent(t *testing.T) {
	common.SetMainDatabaseType(common.DatabaseTypeSQLite)
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	originalDB := DB
	DB = db
	t.Cleanup(func() { DB = originalDB })

	require.NoError(t, db.AutoMigrate(&Option{}, &legacyPasskeyCredential{}))
	require.NoError(t, db.Create(&legacyPasskeyCredential{
		UserID:       7,
		CredentialID: "legacy-credential",
		PublicKey:    "legacy-key",
	}).Error)

	require.NoError(t, migratePasskeyCredentialUserIndex())
	require.NoError(t, migratePasskeyCredentialUserIndex())
	require.True(t, db.Migrator().HasIndex(&PasskeyCredential{}, "idx_passkey_credentials_user_id"))
	require.True(t, db.Migrator().HasIndex(&PasskeyCredential{}, "idx_passkey_user_rp"))

	var migrated PasskeyCredential
	require.NoError(t, db.First(&migrated, "credential_id = ?", "legacy-credential").Error)
	require.Equal(t, "geiliapi.com", migrated.RPID)
	require.Equal(t, "https://geiliapi.com", migrated.Origin)

	var marker Option
	require.NoError(t, db.Where("key = ?", "GeiliPasskeyMultiRPV1").First(&marker).Error)
	require.Equal(t, "true", marker.Value)
}

func TestUpsertPasskeyCredentialAllowsOneCredentialPerUserAndRP(t *testing.T) {
	common.SetMainDatabaseType(common.DatabaseTypeSQLite)
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	originalDB := DB
	DB = db
	t.Cleanup(func() { DB = originalDB })
	require.NoError(t, db.AutoMigrate(&Option{}, &PasskeyCredential{}))
	require.NoError(t, migratePasskeyCredentialUserIndex())

	makeCredential := func(rpID, origin, credentialID string) *PasskeyCredential {
		return &PasskeyCredential{
			UserID:       9,
			RPID:         rpID,
			Origin:       origin,
			CredentialID: credentialID,
			PublicKey:    "public-key",
		}
	}
	require.NoError(t, UpsertPasskeyCredential(makeCredential("geiliapi.com", "https://geiliapi.com", "old")))
	require.NoError(t, UpsertPasskeyCredential(makeCredential("auapi.ai", "https://auapi.ai", "new")))

	var credentials []PasskeyCredential
	require.NoError(t, db.Where("user_id = ?", 9).Order("rp_id asc").Find(&credentials).Error)
	require.Len(t, credentials, 2)
	require.Equal(t, "auapi.ai", credentials[0].RPID)
	require.Equal(t, "geiliapi.com", credentials[1].RPID)

	replacement := makeCredential("auapi.ai", "https://auapi.ai", "newer")
	replacement.SignCount = 4
	require.NoError(t, UpsertPasskeyCredential(replacement))
	var count int64
	require.NoError(t, db.Model(&PasskeyCredential{}).Where("user_id = ?", 9).Count(&count).Error)
	require.Equal(t, int64(2), count)

	var current PasskeyCredential
	require.NoError(t, db.Where("user_id = ? AND rp_id = ?", 9, "auapi.ai").First(&current).Error)
	require.Equal(t, "newer", current.CredentialID)
	require.Equal(t, uint32(4), current.SignCount)

	legacy, err := GetPasskeyByUserIDAndRPID(9, " GEILIAPI.COM ")
	require.NoError(t, err)
	require.Equal(t, "old", legacy.CredentialID)
	_, err = GetPasskeyByUserIDAndRPID(9, "missing.example")
	require.ErrorIs(t, err, ErrPasskeyNotFound)
}
