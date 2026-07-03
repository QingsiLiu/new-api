package model

import (
	"encoding/json"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/stretchr/testify/require"
)

func TestGetPricingIncludesAmountCNYForFixedPriceModels(t *testing.T) {
	truncateTables(t)
	previousModelPrice := ratio_setting.ModelPrice2JSONString()
	t.Cleanup(func() {
		require.NoError(t, ratio_setting.UpdateModelPriceByJSONString(previousModelPrice))
		InvalidatePricingCache()
	})
	require.NoError(t, ratio_setting.UpdateModelPriceByJSONString(`{"fixed-cny-model":0.11}`))
	require.NoError(t, DB.Create(&Channel{
		Id:     1,
		Name:   "fixed price channel",
		Key:    "sk-test",
		Status: common.ChannelStatusEnabled,
		Models: "fixed-cny-model",
		Group:  "default",
	}).Error)
	require.NoError(t, DB.Create(&Ability{
		Group:     "default",
		Model:     "fixed-cny-model",
		ChannelId: 1,
		Enabled:   true,
	}).Error)
	InvalidatePricingCache()

	pricings := GetPricing()
	require.Len(t, pricings, 1)
	body, err := json.Marshal(pricings[0])
	require.NoError(t, err)

	require.Contains(t, string(body), `"amount_cny":0.11`)
	require.NotContains(t, string(body), `"quota":`)
}
