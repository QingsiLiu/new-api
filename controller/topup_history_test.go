package controller

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestGetUserTopUpsReturnsContractDTO(t *testing.T) {
	db := setupModelListControllerTestDB(t)
	require.NoError(t, db.AutoMigrate(&model.TopUp{}))

	user := &model.User{
		Username: "topup-history-user",
		Status:   common.UserStatusEnabled,
	}
	require.NoError(t, db.Create(user).Error)

	creditOrder := &model.TopUp{
		UserId:             user.Id,
		Amount:             1,
		Money:              500,
		TradeNo:            "credits-history-1",
		PaymentProvider:    model.PaymentProviderWaffoPancake,
		CreateTime:         common.GetTimestamp(),
		Status:             common.TopUpStatusSuccess,
		PackageId:          "credits-usd-105000",
		Credits:            1,
		QuotaAmount:        378_000_000,
		Currency:           "USD",
		PriceMinor:         50_000,
		BaseCredits:        100_000,
		BonusCredits:       5_000,
		PaymentStoreId:     "private-store",
		PaymentEnvironment: "sandbox",
	}
	require.NoError(t, db.Create(creditOrder).Error)
	require.NoError(t, db.Create(&model.TopUp{
		UserId:        user.Id,
		Amount:        36,
		Money:         36,
		TradeNo:       "legacy-history-1",
		PaymentMethod: "alipay",
		CreateTime:    common.GetTimestamp(),
		Status:        common.TopUpStatusSuccess,
	}).Error)
	require.NoError(t, db.Create(&model.TopUp{
		UserId:        user.Id + 1,
		Amount:        10,
		Money:         10,
		TradeNo:       "other-user-order",
		PaymentMethod: "alipay",
		CreateTime:    common.GetTimestamp(),
		Status:        common.TopUpStatusSuccess,
	}).Error)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/user/topup/self?p=1&page_size=50", nil)
	ctx.Set("id", user.Id)
	GetUserTopUps(ctx)

	require.Equal(t, http.StatusOK, recorder.Code)
	var response struct {
		Success bool `json:"success"`
		Data    struct {
			Items []map[string]any `json:"items"`
			Total int              `json:"total"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &response))
	require.True(t, response.Success)
	require.Equal(t, 2, response.Data.Total)
	require.Len(t, response.Data.Items, 2)

	var record map[string]any
	var legacy map[string]any
	for _, item := range response.Data.Items {
		switch item["trade_no"] {
		case "credits-history-1":
			record = item
		case "legacy-history-1":
			legacy = item
		}
	}
	require.NotNil(t, record)
	require.NotNil(t, legacy)
	require.Equal(t, "credits-history-1", record["trade_no"])
	require.Equal(t, "credits-usd-105000", record["package_id"])
	require.Equal(t, "105000", record["credits"])
	require.EqualValues(t, 378_000_000, record["quota"])
	require.Equal(t, "USD", record["currency"])
	require.EqualValues(t, 50_000, record["price_minor"])
	require.Equal(t, model.PaymentProviderWaffoPancake, record["payment_method"])

	for _, forbidden := range []string{
		"user_id",
		"payment_provider",
		"payment_store_id",
		"payment_environment",
		"base_credits",
		"bonus_credits",
		"complete_time",
	} {
		require.NotContains(t, record, forbidden)
	}

	require.EqualValues(t, 36, legacy["amount"])
	require.EqualValues(t, 36, legacy["money"])
	require.Equal(t, "alipay", legacy["payment_method"])
	for _, creditsOnly := range []string{"package_id", "credits", "quota", "currency", "price_minor"} {
		require.NotContains(t, legacy, creditsOnly)
	}
}

func TestGetTopUpInfoExposesCanonicalCreditsState(t *testing.T) {
	for _, enabled := range []bool{false, true} {
		t.Run(map[bool]string{false: "disabled", true: "enabled"}[enabled], func(t *testing.T) {
			if enabled {
				t.Setenv(common.CreditsFeatureFlagEnv, "true")
			} else {
				t.Setenv(common.CreditsFeatureFlagEnv, "false")
			}

			recorder := httptest.NewRecorder()
			ctx, _ := gin.CreateTestContext(recorder)
			ctx.Request = httptest.NewRequest(http.MethodGet, "/api/user/topup/info", nil)
			GetTopUpInfo(ctx)

			require.Equal(t, http.StatusOK, recorder.Code)
			var response struct {
				Success bool `json:"success"`
				Data    struct {
					CreditsEnabled   bool                  `json:"credits_enabled"`
					CreditsV1Enabled bool                  `json:"credits_v1_enabled"`
					QuotaPerCredit   int                   `json:"quota_per_credit"`
					Packages         []model.CreditPackage `json:"packages"`
				} `json:"data"`
			}
			require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &response))
			require.True(t, response.Success)
			require.Equal(t, enabled, response.Data.CreditsEnabled)
			require.Equal(t, enabled, response.Data.CreditsV1Enabled)
			require.Equal(t, common.CreditsQuotaUnit, response.Data.QuotaPerCredit)
			if enabled {
				require.Len(t, response.Data.Packages, 10)
			} else {
				require.Empty(t, response.Data.Packages)
			}
		})
	}
}
