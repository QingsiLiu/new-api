package controller

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

type manageUserAPIResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

func setupUserControllerTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	gin.SetMode(gin.TestMode)
	common.UsingSQLite = true
	common.UsingMySQL = false
	common.UsingPostgreSQL = false
	common.RedisEnabled = false

	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	model.DB = db
	model.LOG_DB = db
	require.NoError(t, db.AutoMigrate(&model.User{}, &model.UserQuotaChangeRecord{}))

	t.Cleanup(func() {
		sqlDB, err := db.DB()
		if err == nil {
			_ = sqlDB.Close()
		}
	})
	return db
}

func TestManageUserQuotaIsIdempotentWithRequestID(t *testing.T) {
	db := setupUserControllerTestDB(t)

	target := &model.User{
		Username:  "bridge-user",
		Password:  "hash",
		DisplayName: "Bridge User",
		Role:      common.RoleCommonUser,
		Status:    common.UserStatusEnabled,
		Quota:     10,
		Group:     "default",
	}
	require.NoError(t, db.Create(target).Error)

	reqBody := `{"id":%d,"action":"add_quota","mode":"add","value":7,"request_id":"req-123"}`
	call := func() *httptest.ResponseRecorder {
		rec := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(rec)
		ctx.Request = httptest.NewRequest(http.MethodPost, "/api/user/manage", strings.NewReader(fmt.Sprintf(reqBody, target.Id)))
		ctx.Request.Header.Set("Content-Type", "application/json")
		ctx.Set("id", 1)
		ctx.Set("role", common.RoleRootUser)
		ManageUser(ctx)
		return rec
	}

	first := call()
	require.Equal(t, http.StatusOK, first.Code, first.Body.String())
	second := call()
	require.Equal(t, http.StatusOK, second.Code, second.Body.String())

	got, err := model.GetUserById(target.Id, false)
	require.NoError(t, err)
	require.Equal(t, 17, got.Quota)

	var records []model.UserQuotaChangeRecord
	require.NoError(t, db.Order("id asc").Find(&records).Error)
	require.Len(t, records, 1)
	require.Equal(t, "req-123", records[0].RequestId)
	require.Equal(t, 10, records[0].BeforeQuota)
	require.Equal(t, 17, records[0].AfterQuota)
}
