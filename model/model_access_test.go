package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/require"
)

func TestUpdateModelChannelAccessRebuildsChannelAbilities(t *testing.T) {
	truncateTables(t)

	priority := int64(10)
	weight := uint(3)
	channelA := Channel{
		Type:     1,
		Key:      "a",
		Status:   common.ChannelStatusEnabled,
		Name:     "channel-a",
		Models:   "target,other",
		Group:    "default,vip",
		Priority: &priority,
		Weight:   &weight,
	}
	channelB := Channel{
		Type:   1,
		Key:    "b",
		Status: common.ChannelStatusEnabled,
		Name:   "channel-b",
		Models: "other",
		Group:  "default",
	}
	require.NoError(t, DB.Create(&channelA).Error)
	require.NoError(t, channelA.AddAbilities(nil))
	require.NoError(t, DB.Create(&channelB).Error)
	require.NoError(t, channelB.AddAbilities(nil))

	updated, err := UpdateModelChannelAccess("target", []int{channelB.Id})

	require.NoError(t, err)
	require.Equal(t, 2, updated)

	var refreshedA Channel
	require.NoError(t, DB.First(&refreshedA, channelA.Id).Error)
	require.Equal(t, "other", refreshedA.Models)

	var refreshedB Channel
	require.NoError(t, DB.First(&refreshedB, channelB.Id).Error)
	require.Equal(t, "other,target", refreshedB.Models)

	var disabledCount int64
	require.NoError(t, DB.Model(&Ability{}).Where("channel_id = ? AND model = ?", channelA.Id, "target").Count(&disabledCount).Error)
	require.Zero(t, disabledCount)

	var enabledAbilities []Ability
	require.NoError(t, DB.Where("channel_id = ? AND model = ?", channelB.Id, "target").Find(&enabledAbilities).Error)
	enabledGroups := make([]string, 0, len(enabledAbilities))
	for _, ability := range enabledAbilities {
		enabledGroups = append(enabledGroups, ability.Group)
	}
	require.ElementsMatch(t, []string{"default"}, enabledGroups)
}

func TestModelUpdateRenamesChannelAccessInSameTransaction(t *testing.T) {
	truncateTables(t)

	channel := Channel{
		Type:   1,
		Key:    "a",
		Status: common.ChannelStatusEnabled,
		Name:   "channel-a",
		Models: "old-name,other",
		Group:  "default",
	}
	require.NoError(t, DB.Create(&channel).Error)
	require.NoError(t, channel.AddAbilities(nil))

	modelMeta := Model{
		ModelName:    "old-name",
		Status:       1,
		SyncOfficial: 1,
		NameRule:     NameRuleExact,
	}
	require.NoError(t, modelMeta.Insert())

	modelMeta.ModelName = "new-name"
	require.NoError(t, modelMeta.Update())

	var refreshed Channel
	require.NoError(t, DB.First(&refreshed, channel.Id).Error)
	require.Equal(t, "new-name,other", refreshed.Models)

	var oldAbilityCount int64
	require.NoError(t, DB.Model(&Ability{}).Where("channel_id = ? AND model = ?", channel.Id, "old-name").Count(&oldAbilityCount).Error)
	require.Zero(t, oldAbilityCount)

	var newAbilityCount int64
	require.NoError(t, DB.Model(&Ability{}).Where("channel_id = ? AND model = ?", channel.Id, "new-name").Count(&newAbilityCount).Error)
	require.EqualValues(t, 1, newAbilityCount)
}
