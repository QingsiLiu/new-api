package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/stretchr/testify/require"
)

func TestInitOptionMapIncludesAsyncTaskSpecPricingEnabled(t *testing.T) {
	truncateTables(t)
	operation_setting.AsyncTaskSpecPricingEnabled = false

	InitOptionMap()

	require.Equal(t, "false", common.OptionMap["AsyncTaskSpecPricingEnabled"])
	require.Contains(t, common.OptionMap["AsyncSpecPricing"], "gpt-image-2")
	require.Contains(t, common.OptionMap["AsyncSpecPricing"], "gemini-3-pro-image-preview")
	imageResult := operation_setting.ResolveImageSpecQuota("gpt-image-2", "2048x2048", "", "", 1)
	require.True(t, imageResult.Matched)
	require.Equal(t, "2k", imageResult.SpecKey)
	require.NotEmpty(t, common.OptionMap["QuotaPerCNY"])
}

func TestUpdateOptionMapUpdatesAsyncTaskSpecPricingEnabled(t *testing.T) {
	truncateTables(t)
	operation_setting.AsyncTaskSpecPricingEnabled = false
	previousPricing := operation_setting.AsyncSpecPricing2JSONString()
	previousQuotaPerCNY := operation_setting.QuotaPerCNY
	t.Cleanup(func() {
		require.NoError(t, operation_setting.UpdateAsyncSpecPricingByJSONString(previousPricing))
		operation_setting.QuotaPerCNY = previousQuotaPerCNY
	})

	require.NoError(t, updateOptionMap("AsyncTaskSpecPricingEnabled", "true"))
	require.True(t, operation_setting.AsyncTaskSpecPricingEnabled)

	require.NoError(t, updateOptionMap("AsyncTaskSpecPricingEnabled", "false"))
	require.False(t, operation_setting.AsyncTaskSpecPricingEnabled)

	require.NoError(t, updateOptionMap("QuotaPerCNY", "1000"))
	require.Equal(t, 1000.0, operation_setting.QuotaPerCNY)
	require.Error(t, updateOptionMap("QuotaPerCNY", "0"))
	require.Equal(t, 1000.0, operation_setting.QuotaPerCNY)
	require.Error(t, updateOptionMap("QuotaPerCNY", "not-a-number"))
	require.Equal(t, 1000.0, operation_setting.QuotaPerCNY)

	specJSON := `{"video":{"seedance-2.0-fast":{"default_cny_per_second":0.25}}}`
	require.NoError(t, updateOptionMap("AsyncSpecPricing", specJSON))
	result := operation_setting.ResolveVideoSpecQuota("seedance-2.0-fast", "720p", 4)
	require.True(t, result.Matched)
	require.Equal(t, 1000, result.Quota)
}

func TestUpdateOptionPersistsAsyncTaskSpecPricingEnabled(t *testing.T) {
	truncateTables(t)
	operation_setting.AsyncTaskSpecPricingEnabled = false

	require.NoError(t, UpdateOption("AsyncTaskSpecPricingEnabled", "true"))
	require.True(t, operation_setting.AsyncTaskSpecPricingEnabled)

	var option Option
	require.NoError(t, DB.First(&option, "key = ?", "AsyncTaskSpecPricingEnabled").Error)
	require.Equal(t, "true", option.Value)
}

func TestUpdateOptionRejectsInvalidAsyncSpecPricingOptionsBeforePersisting(t *testing.T) {
	truncateTables(t)

	require.Error(t, UpdateOption("QuotaPerCNY", "0"))
	require.Error(t, UpdateOption("AsyncSpecPricing", "{bad-json"))

	var count int64
	require.NoError(t, DB.Model(&Option{}).Where("key IN ?", []string{"QuotaPerCNY", "AsyncSpecPricing"}).Count(&count).Error)
	require.Zero(t, count)
}

func TestInitOptionMapIncludesAsyncTaskProductRoutesEnabled(t *testing.T) {
	truncateTables(t)
	operation_setting.AsyncTaskProductRoutesEnabled = false

	InitOptionMap()

	require.Equal(t, "false", common.OptionMap["AsyncTaskProductRoutesEnabled"])
}

func TestUpdateOptionMapUpdatesAsyncTaskProductRoutesEnabled(t *testing.T) {
	truncateTables(t)
	operation_setting.AsyncTaskProductRoutesEnabled = false

	require.NoError(t, updateOptionMap("AsyncTaskProductRoutesEnabled", "true"))
	require.True(t, operation_setting.AsyncTaskProductRoutesEnabled)

	require.NoError(t, updateOptionMap("AsyncTaskProductRoutesEnabled", "false"))
	require.False(t, operation_setting.AsyncTaskProductRoutesEnabled)
}

func TestUpdateOptionPersistsAsyncTaskProductRoutesEnabled(t *testing.T) {
	truncateTables(t)
	operation_setting.AsyncTaskProductRoutesEnabled = false

	require.NoError(t, UpdateOption("AsyncTaskProductRoutesEnabled", "true"))
	require.True(t, operation_setting.AsyncTaskProductRoutesEnabled)

	var option Option
	require.NoError(t, DB.First(&option, "key = ?", "AsyncTaskProductRoutesEnabled").Error)
	require.Equal(t, "true", option.Value)
}

func TestInitOptionMapIncludesAsyncTaskServiceUserProxyEnabled(t *testing.T) {
	truncateTables(t)
	operation_setting.AsyncTaskServiceUserProxyEnabled = false

	InitOptionMap()

	require.Equal(t, "false", common.OptionMap["AsyncTaskServiceUserProxyEnabled"])
}

func TestUpdateOptionMapUpdatesAsyncTaskServiceUserProxyEnabled(t *testing.T) {
	truncateTables(t)
	operation_setting.AsyncTaskServiceUserProxyEnabled = false

	require.NoError(t, updateOptionMap("AsyncTaskServiceUserProxyEnabled", "true"))
	require.True(t, operation_setting.AsyncTaskServiceUserProxyEnabled)

	require.NoError(t, updateOptionMap("AsyncTaskServiceUserProxyEnabled", "false"))
	require.False(t, operation_setting.AsyncTaskServiceUserProxyEnabled)
}

func TestUpdateOptionPersistsAsyncTaskServiceUserProxyEnabled(t *testing.T) {
	truncateTables(t)
	operation_setting.AsyncTaskServiceUserProxyEnabled = false

	require.NoError(t, UpdateOption("AsyncTaskServiceUserProxyEnabled", "true"))
	require.True(t, operation_setting.AsyncTaskServiceUserProxyEnabled)

	var option Option
	require.NoError(t, DB.First(&option, "key = ?", "AsyncTaskServiceUserProxyEnabled").Error)
	require.Equal(t, "true", option.Value)
}
