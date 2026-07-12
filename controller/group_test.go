package controller

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

type groupAPIResponse struct {
	Success bool                 `json:"success"`
	Message string               `json:"message"`
	Data    []string             `json:"data"`
	Groups  []model.GroupDisplay `json:"groups"`
}

type userGroupsAPIResponse struct {
	Success bool                              `json:"success"`
	Message string                            `json:"message"`
	Data    map[string]map[string]interface{} `json:"data"`
}

type groupRegistryAPIResponse struct {
	Success bool                      `json:"success"`
	Message string                    `json:"message"`
	Data    []model.GroupRegistryView `json:"data"`
}

type groupRegistryItemAPIResponse struct {
	Success bool                    `json:"success"`
	Message string                  `json:"message"`
	Data    model.GroupRegistryView `json:"data"`
}

func setupGroupControllerTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	initModelListColumnNames(t)

	gin.SetMode(gin.TestMode)
	common.SetMainDatabaseType(common.DatabaseTypeSQLite)
	common.SetLogDatabaseType(common.DatabaseTypeSQLite)
	common.RedisEnabled = false

	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	model.DB = db
	model.LOG_DB = db
	require.NoError(t, db.AutoMigrate(&model.User{}, &model.GroupRegistry{}))
	model.InvalidateGroupRegistryCache()

	t.Cleanup(func() {
		model.InvalidateGroupRegistryCache()
		sqlDB, err := db.DB()
		if err == nil {
			_ = sqlDB.Close()
		}
	})
	return db
}

func TestGetGroupsKeepsCodesAndAddsDisplayGroups(t *testing.T) {
	setupGroupControllerTestDB(t)
	originalGroupRatio := ratio_setting.GroupRatio2JSONString()
	t.Cleanup(func() {
		require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(originalGroupRatio))
	})
	require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(`{"vip":1}`))
	require.NoError(t, model.DB.Create(&model.GroupRegistry{Code: "vip", DisplayName: "VIP Display"}).Error)
	model.InvalidateGroupRegistryCache()

	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/group/", nil)

	GetGroups(ctx)

	require.Equal(t, http.StatusOK, rec.Code)
	var response groupAPIResponse
	require.NoError(t, common.Unmarshal(rec.Body.Bytes(), &response))
	require.True(t, response.Success)
	require.Equal(t, []string{"vip"}, response.Data)
	require.Equal(t, []model.GroupDisplay{{Code: "vip", DisplayName: "VIP Display"}}, response.Groups)
}

func TestGetUserGroupsAddsDisplayNameAndFallsBackToCode(t *testing.T) {
	db := setupGroupControllerTestDB(t)
	originalGroupRatio := ratio_setting.GroupRatio2JSONString()
	originalUserUsableGroups := setting.UserUsableGroups2JSONString()
	t.Cleanup(func() {
		require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(originalGroupRatio))
		require.NoError(t, setting.UpdateUserUsableGroupsByJSONString(originalUserUsableGroups))
	})
	require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(`{"vip":1,"unknown":1}`))
	require.NoError(t, setting.UpdateUserUsableGroupsByJSONString(`{"vip":"vip desc","unknown":"unknown desc"}`))
	require.NoError(t, db.Create(&model.User{Id: 1, Username: "group-user", Password: "password123", Group: "vip"}).Error)
	require.NoError(t, db.Create(&model.GroupRegistry{Code: "vip", DisplayName: "VIP Display"}).Error)
	model.InvalidateGroupRegistryCache()

	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/user/self/groups", nil)
	ctx.Set("id", 1)

	GetUserGroups(ctx)

	require.Equal(t, http.StatusOK, rec.Code)
	var response userGroupsAPIResponse
	require.NoError(t, common.Unmarshal(rec.Body.Bytes(), &response))
	require.True(t, response.Success)
	require.Equal(t, "VIP Display", response.Data["vip"]["display_name"])
	require.Equal(t, "unknown", response.Data["unknown"]["display_name"])
}

func TestCreateGroupRegistryEndpointCreatesHiddenCode(t *testing.T) {
	setupGroupControllerTestDB(t)
	originalGroupRatio := ratio_setting.GroupRatio2JSONString()
	t.Cleanup(func() {
		require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(originalGroupRatio))
	})

	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/api/group/registry", strings.NewReader(`{"display_name":"Business","ratio":1.5,"user_usable":true,"description":"Business desc"}`))
	ctx.Request.Header.Set("Content-Type", "application/json")

	CreateGroupRegistry(ctx)

	require.Equal(t, http.StatusOK, rec.Code)
	var response groupRegistryItemAPIResponse
	require.NoError(t, common.Unmarshal(rec.Body.Bytes(), &response))
	require.True(t, response.Success, response.Message)
	require.NotEmpty(t, response.Data.Code)
	require.NotEqual(t, "Business", response.Data.Code)
	require.Equal(t, "Business", response.Data.DisplayName)
	require.Equal(t, 1.5, response.Data.Ratio)
	require.True(t, response.Data.UserUsable)
}

func TestUpdateGroupRegistryEndpointDoesNotAcceptCodeChange(t *testing.T) {
	setupGroupControllerTestDB(t)
	require.NoError(t, model.DB.Create(&model.GroupRegistry{Code: "legacy", DisplayName: "Legacy"}).Error)
	model.InvalidateGroupRegistryCache()

	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	ctx.Params = gin.Params{{Key: "code", Value: "legacy"}}
	ctx.Request = httptest.NewRequest(http.MethodPut, "/api/group/registry/legacy", strings.NewReader(`{"code":"other","display_name":"Renamed"}`))
	ctx.Request.Header.Set("Content-Type", "application/json")

	UpdateGroupRegistry(ctx)

	require.Equal(t, http.StatusOK, rec.Code)
	var response groupRegistryItemAPIResponse
	require.NoError(t, common.Unmarshal(rec.Body.Bytes(), &response))
	require.True(t, response.Success, response.Message)
	require.Equal(t, "legacy", response.Data.Code)
	require.Equal(t, "Renamed", response.Data.DisplayName)
}

func TestDeleteGroupRegistryEndpointReturnsReferences(t *testing.T) {
	setupGroupControllerTestDB(t)
	originalGroupRatio := ratio_setting.GroupRatio2JSONString()
	t.Cleanup(func() {
		require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(originalGroupRatio))
	})
	require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(`{"busy":1}`))
	require.NoError(t, model.DB.Create(&model.GroupRegistry{Code: "busy", DisplayName: "Busy"}).Error)
	model.InvalidateGroupRegistryCache()

	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	ctx.Params = gin.Params{{Key: "code", Value: "busy"}}
	ctx.Request = httptest.NewRequest(http.MethodDelete, "/api/group/registry/busy", nil)

	DeleteGroupRegistry(ctx)

	require.Equal(t, http.StatusOK, rec.Code)
	var response struct {
		Success bool           `json:"success"`
		Message string         `json:"message"`
		Data    map[string]int `json:"data"`
	}
	require.NoError(t, common.Unmarshal(rec.Body.Bytes(), &response))
	require.False(t, response.Success)
	require.Equal(t, 1, response.Data["group_ratio"])
}

func TestUserTokenChannelLogAndPricingResponsesIncludeGroupDisplay(t *testing.T) {
	db := setupGroupControllerTestDB(t)
	require.NoError(t, db.AutoMigrate(&model.Token{}, &model.Channel{}, &model.Log{}))
	model.LOG_DB = db

	originalGroupRatio := ratio_setting.GroupRatio2JSONString()
	originalUserUsableGroups := setting.UserUsableGroups2JSONString()
	t.Cleanup(func() {
		require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(originalGroupRatio))
		require.NoError(t, setting.UpdateUserUsableGroupsByJSONString(originalUserUsableGroups))
	})
	require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(`{"vip":1,"fallback":1}`))
	require.NoError(t, setting.UpdateUserUsableGroupsByJSONString(`{"vip":"vip desc","fallback":"fallback desc"}`))
	require.NoError(t, db.Create(&model.GroupRegistry{Code: "vip", DisplayName: "VIP Display"}).Error)
	model.InvalidateGroupRegistryCache()
	require.NoError(t, db.Create(&model.User{Id: 1, Username: "display-user", Password: "password123", Group: "vip"}).Error)
	require.NoError(t, db.Create(&model.Token{Id: 1, UserId: 1, Name: "display-token", Key: "sk-display", Group: "vip"}).Error)
	require.NoError(t, db.Create(&model.Channel{Id: 1, Name: "display-channel", Key: "sk-channel", Group: "vip,fallback"}).Error)
	require.NoError(t, db.Create(&model.Log{Id: 1, UserId: 1, Username: "display-user", CreatedAt: 100, Type: model.LogTypeConsume, Group: "vip"}).Error)

	userRec := httptest.NewRecorder()
	userCtx, _ := gin.CreateTestContext(userRec)
	userCtx.Request = httptest.NewRequest(http.MethodGet, "/api/user/?p=1&page_size=10", nil)
	GetAllUsers(userCtx)
	require.Equal(t, http.StatusOK, userRec.Code)
	var userResp struct {
		Success bool `json:"success"`
		Data    struct {
			Items []model.User `json:"items"`
		} `json:"data"`
	}
	require.NoError(t, common.Unmarshal(userRec.Body.Bytes(), &userResp))
	require.True(t, userResp.Success)
	require.Equal(t, "VIP Display", userResp.Data.Items[0].GroupDisplay)

	tokenRec := httptest.NewRecorder()
	tokenCtx, _ := gin.CreateTestContext(tokenRec)
	tokenCtx.Request = httptest.NewRequest(http.MethodGet, "/api/token/?p=1&page_size=10", nil)
	tokenCtx.Set("id", 1)
	GetAllTokens(tokenCtx)
	require.Equal(t, http.StatusOK, tokenRec.Code)
	var tokenResp struct {
		Success bool `json:"success"`
		Data    struct {
			Items []model.Token `json:"items"`
		} `json:"data"`
	}
	require.NoError(t, common.Unmarshal(tokenRec.Body.Bytes(), &tokenResp))
	require.True(t, tokenResp.Success)
	require.Equal(t, "VIP Display", tokenResp.Data.Items[0].GroupDisplay)

	channelRec := httptest.NewRecorder()
	channelCtx, _ := gin.CreateTestContext(channelRec)
	channelCtx.Request = httptest.NewRequest(http.MethodGet, "/api/channel/?p=1&page_size=10", nil)
	GetAllChannels(channelCtx)
	require.Equal(t, http.StatusOK, channelRec.Code)
	var channelResp struct {
		Success bool `json:"success"`
		Data    struct {
			Items []model.Channel `json:"items"`
		} `json:"data"`
	}
	require.NoError(t, common.Unmarshal(channelRec.Body.Bytes(), &channelResp))
	require.True(t, channelResp.Success)
	require.Equal(t, "VIP Display, fallback", channelResp.Data.Items[0].GroupDisplay)
	require.Equal(t, []model.GroupDisplay{
		{Code: "vip", DisplayName: "VIP Display"},
		{Code: "fallback", DisplayName: "fallback"},
	}, channelResp.Data.Items[0].GroupsDisplay)

	logRec := httptest.NewRecorder()
	logCtx, _ := gin.CreateTestContext(logRec)
	logCtx.Request = httptest.NewRequest(http.MethodGet, "/api/log/?p=1&page_size=10", nil)
	GetAllLogs(logCtx)
	require.Equal(t, http.StatusOK, logRec.Code)
	var logResp struct {
		Success bool `json:"success"`
		Data    struct {
			Items []model.Log `json:"items"`
		} `json:"data"`
	}
	require.NoError(t, common.Unmarshal(logRec.Body.Bytes(), &logResp))
	require.True(t, logResp.Success)
	require.Equal(t, "VIP Display", logResp.Data.Items[0].GroupDisplay)

	pricingRec := httptest.NewRecorder()
	pricingCtx, _ := gin.CreateTestContext(pricingRec)
	pricingCtx.Request = httptest.NewRequest(http.MethodGet, "/api/pricing", nil)
	pricingCtx.Set("id", 1)
	GetPricing(pricingCtx)
	require.Equal(t, http.StatusOK, pricingRec.Code)
	var pricingResp struct {
		Success            bool                 `json:"success"`
		UsableGroupDisplay map[string]string    `json:"usable_group_display"`
		GroupDisplay       map[string]string    `json:"group_display"`
		AutoGroupsDisplay  []model.GroupDisplay `json:"auto_groups_display"`
	}
	require.NoError(t, common.Unmarshal(pricingRec.Body.Bytes(), &pricingResp))
	require.True(t, pricingResp.Success)
	require.Equal(t, "VIP Display", pricingResp.UsableGroupDisplay["vip"])
	require.Equal(t, "fallback", pricingResp.GroupDisplay["fallback"])
}
