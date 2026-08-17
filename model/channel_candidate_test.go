package model

import (
	"fmt"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/stretchr/testify/require"
)

func TestGetSatisfiedChannelCandidatesSupportsCacheModesAndPriorityOrdering(t *testing.T) {
	for _, memoryCacheEnabled := range []bool{false, true} {
		t.Run(fmt.Sprintf("memory_cache_%t", memoryCacheEnabled), func(t *testing.T) {
			setChannelCandidateCacheMode(t, memoryCacheEnabled)
			truncateTables(t)

			seedCandidateChannel(t, 5101, "high-a", "default", "candidate-model", common.ChannelStatusEnabled, 100, 1, "", "")
			seedCandidateChannel(t, 5102, "high-b", "default", "candidate-model", common.ChannelStatusEnabled, 100, 5, "", "")
			seedCandidateChannel(t, 5103, "low", "default", "candidate-model", common.ChannelStatusEnabled, 10, 1, "", "")
			seedCandidateChannel(t, 5104, "disabled", "default", "candidate-model", common.ChannelStatusManuallyDisabled, 1000, 100, "", "")
			// Guard against stale/inconsistent ability state: channel status remains
			// authoritative for the async candidate API in both cache modes.
			require.NoError(t, UpdateAbilityStatus(5104, true))
			refreshChannelCandidateCache(memoryCacheEnabled)

			candidates, err := GetSatisfiedChannelCandidatesWithSelectionFilter("default", "candidate-model", "", nil)
			require.NoError(t, err)
			require.Len(t, candidates, 3)
			require.Equal(t, int64(100), candidates[0].Priority)
			require.Equal(t, int64(100), candidates[1].Priority)
			require.Equal(t, int64(10), candidates[2].Priority)
			require.ElementsMatch(t, []int{5101, 5102}, []int{candidates[0].Channel.Id, candidates[1].Channel.Id})
			require.Equal(t, 5103, candidates[2].Channel.Id)

			// A caller receives a snapshot, never the shared cached Channel pointer.
			if memoryCacheEnabled {
				candidates[0].Channel.Name = "mutated-snapshot"
				cached, cacheErr := CacheGetChannel(candidates[0].Channel.Id)
				require.NoError(t, cacheErr)
				require.NotEqual(t, "mutated-snapshot", cached.Name)
			}
		})
	}
}

func TestGetSatisfiedChannelCandidatesPrefersExactThenFallsBackToNormalizedModel(t *testing.T) {
	for _, memoryCacheEnabled := range []bool{false, true} {
		t.Run(fmt.Sprintf("memory_cache_%t", memoryCacheEnabled), func(t *testing.T) {
			setChannelCandidateCacheMode(t, memoryCacheEnabled)
			truncateTables(t)

			requestedModel := "gpt-4-gizmo-candidate"
			normalizedModel := "gpt-4-gizmo-*"
			seedCandidateChannel(t, 5201, "exact", "default", requestedModel, common.ChannelStatusEnabled, 10, 1, "", "")
			seedCandidateChannel(t, 5202, "normalized", "default", normalizedModel, common.ChannelStatusEnabled, 100, 1, "", "")
			refreshChannelCandidateCache(memoryCacheEnabled)

			candidates, err := GetSatisfiedChannelCandidatesWithSelectionFilter("default", requestedModel, "", nil)
			require.NoError(t, err)
			require.Len(t, candidates, 1)
			require.Equal(t, 5201, candidates[0].Channel.Id)

			require.NoError(t, DB.Model(&Channel{}).Where("id = ?", 5201).Update("status", common.ChannelStatusManuallyDisabled).Error)
			require.NoError(t, UpdateAbilityStatus(5201, false))
			refreshChannelCandidateCache(memoryCacheEnabled)

			candidates, err = GetSatisfiedChannelCandidatesWithSelectionFilter("default", requestedModel, "", nil)
			require.NoError(t, err)
			require.Len(t, candidates, 1)
			require.Equal(t, 5202, candidates[0].Channel.Id)
		})
	}
}

func TestGetSatisfiedChannelCandidatesAppliesPathBeforeNormalizedFallback(t *testing.T) {
	for _, memoryCacheEnabled := range []bool{false, true} {
		t.Run(fmt.Sprintf("memory_cache_%t", memoryCacheEnabled), func(t *testing.T) {
			setChannelCandidateCacheMode(t, memoryCacheEnabled)
			truncateTables(t)

			requestedModel := "gpt-4-gizmo-route-candidate"
			normalizedModel := "gpt-4-gizmo-*"
			otherSettings := `{"advanced_custom":{"advanced_routes":[{"incoming_path":"/v1/responses","models":["gpt-4-gizmo-route-candidate"]}]}}`
			seedCandidateChannel(t, 5301, "exact-wrong-path", "default", requestedModel, common.ChannelStatusEnabled, 100, 1, "", otherSettings)
			require.NoError(t, DB.Model(&Channel{}).Where("id = ?", 5301).Update("type", constant.ChannelTypeAdvancedCustom).Error)
			seedCandidateChannel(t, 5302, "normalized-compatible", "default", normalizedModel, common.ChannelStatusEnabled, 10, 1, "", "")
			refreshChannelCandidateCache(memoryCacheEnabled)

			candidates, err := GetSatisfiedChannelCandidatesWithSelectionFilter("default", requestedModel, "/v1/images/generations", nil)
			require.NoError(t, err)
			require.Len(t, candidates, 1)
			require.Equal(t, 5302, candidates[0].Channel.Id)
		})
	}
}

func TestGetSatisfiedChannelCandidatesPreservesGlobalSelectionFilterSemantics(t *testing.T) {
	for _, memoryCacheEnabled := range []bool{false, true} {
		t.Run(fmt.Sprintf("memory_cache_%t", memoryCacheEnabled), func(t *testing.T) {
			setChannelCandidateCacheMode(t, memoryCacheEnabled)
			truncateTables(t)

			seedCandidateChannel(t, 5401, "not-applicable", "default", "spec-model", common.ChannelStatusEnabled, 100, 1, "", "")
			seedCandidateChannel(t, 5402, "matched", "default", "spec-model", common.ChannelStatusEnabled, 10, 1, "", "")
			seedCandidateChannel(t, 5403, "applicable-mismatch", "default", "spec-model", common.ChannelStatusEnabled, 10, 1, "", "")
			refreshChannelCandidateCache(memoryCacheEnabled)

			filter := func(channel *Channel) ChannelSelectionFilterResult {
				switch channel.Id {
				case 5402:
					return ChannelSelectionFilterResult{Applies: true, Match: true}
				case 5403:
					return ChannelSelectionFilterResult{Applies: true, Match: false}
				default:
					return ChannelSelectionFilterResult{}
				}
			}
			candidates, err := GetSatisfiedChannelCandidatesWithSelectionFilter("default", "spec-model", "", filter)
			require.NoError(t, err)
			require.Len(t, candidates, 1)
			require.Equal(t, 5402, candidates[0].Channel.Id)

			noApplicableFilter := func(_ *Channel) ChannelSelectionFilterResult {
				return ChannelSelectionFilterResult{}
			}
			candidates, err = GetSatisfiedChannelCandidatesWithSelectionFilter("default", "spec-model", "", noApplicableFilter)
			require.NoError(t, err)
			require.Len(t, candidates, 3)
		})
	}
}

func seedCandidateChannel(t *testing.T, id int, name string, group string, modelName string, status int, priority int64, weight uint, setting string, otherSettings string) {
	t.Helper()
	channel := Channel{
		Id:            id,
		Type:          constant.ChannelTypeOpenAI,
		Key:           fmt.Sprintf("sk-candidate-%d", id),
		Status:        status,
		Name:          name,
		Models:        modelName,
		Group:         group,
		Priority:      common.GetPointer(priority),
		Weight:        common.GetPointer(weight),
		OtherSettings: otherSettings,
	}
	if setting != "" {
		channel.Setting = common.GetPointer(setting)
	}
	require.NoError(t, DB.Create(&channel).Error)
	require.NoError(t, DB.Create(&Ability{
		Group:     group,
		Model:     modelName,
		ChannelId: id,
		Enabled:   status == common.ChannelStatusEnabled,
		Priority:  common.GetPointer(priority),
		Weight:    weight,
	}).Error)
}

func setChannelCandidateCacheMode(t *testing.T, enabled bool) {
	t.Helper()
	previous := common.MemoryCacheEnabled
	common.MemoryCacheEnabled = enabled
	t.Cleanup(func() {
		common.MemoryCacheEnabled = previous
		if previous {
			InitChannelCache()
		}
	})
}

func refreshChannelCandidateCache(enabled bool) {
	if enabled {
		InitChannelCache()
	}
}
