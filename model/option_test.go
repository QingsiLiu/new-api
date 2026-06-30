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
}

func TestUpdateOptionMapUpdatesAsyncTaskSpecPricingEnabled(t *testing.T) {
	truncateTables(t)
	operation_setting.AsyncTaskSpecPricingEnabled = false

	require.NoError(t, updateOptionMap("AsyncTaskSpecPricingEnabled", "true"))
	require.True(t, operation_setting.AsyncTaskSpecPricingEnabled)

	require.NoError(t, updateOptionMap("AsyncTaskSpecPricingEnabled", "false"))
	require.False(t, operation_setting.AsyncTaskSpecPricingEnabled)
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
