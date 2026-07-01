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

type updateOptionAPIResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

func setupOptionControllerTestDB(t *testing.T) *gorm.DB {
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
	require.NoError(t, db.AutoMigrate(&model.Option{}))

	t.Cleanup(func() {
		sqlDB, err := db.DB()
		if err == nil {
			_ = sqlDB.Close()
		}
	})
	return db
}

func callUpdateOptionForTest(t *testing.T, body string) (updateOptionAPIResponse, int) {
	t.Helper()

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPut, "/api/option/", strings.NewReader(body))
	ctx.Request.Header.Set("Content-Type", "application/json")
	UpdateOption(ctx)

	var response updateOptionAPIResponse
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	return response, recorder.Code
}

func TestUpdateOptionRejectsInvalidQuotaPerCNYBeforePersisting(t *testing.T) {
	db := setupOptionControllerTestDB(t)

	response, status := callUpdateOptionForTest(t, `{"key":"QuotaPerCNY","value":"0"}`)

	require.Equal(t, http.StatusOK, status)
	require.False(t, response.Success)
	require.Contains(t, response.Message, "QuotaPerCNY")
	var count int64
	require.NoError(t, db.Model(&model.Option{}).Where("key = ?", "QuotaPerCNY").Count(&count).Error)
	require.Zero(t, count)
}

func TestUpdateOptionRejectsInvalidAsyncSpecPricingBeforePersisting(t *testing.T) {
	db := setupOptionControllerTestDB(t)

	response, status := callUpdateOptionForTest(t, `{"key":"AsyncSpecPricing","value":"{bad-json"}`)

	require.Equal(t, http.StatusOK, status)
	require.False(t, response.Success)
	require.Contains(t, response.Message, "规格定价设置失败")
	var count int64
	require.NoError(t, db.Model(&model.Option{}).Where("key = ?", "AsyncSpecPricing").Count(&count).Error)
	require.Zero(t, count)
}
