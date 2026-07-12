package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/require"
)

func TestInitChannelCacheIgnoresDisabledAbilities(t *testing.T) {
	truncateTables(t)

	originalMemoryCacheEnabled := common.MemoryCacheEnabled
	common.MemoryCacheEnabled = true
	t.Cleanup(func() {
		common.MemoryCacheEnabled = originalMemoryCacheEnabled
		InitChannelCache()
	})

	require.NoError(t, DB.Create(&Channel{
		Id:     1,
		Type:   1,
		Key:    "sk-channel",
		Status: common.ChannelStatusEnabled,
		Name:   "disabled-ability-channel",
		Models: "gemini-3-pro-image-preview",
		Group:  "media",
	}).Error)
	require.NoError(t, DB.Create(&Ability{
		Group:     "media",
		Model:     "gemini-3-pro-image-preview",
		ChannelId: 1,
		Enabled:   false,
	}).Error)

	InitChannelCache()

	channel, err := GetRandomSatisfiedChannel("media", "gemini-3-pro-image-preview", 0, "")
	require.NoError(t, err)
	require.Nil(t, channel)
}
