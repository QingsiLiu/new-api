package service

import (
	"fmt"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestCacheGetAsyncChannelCandidatesUsesDistinctChannelsAndPriorityOrder(t *testing.T) {
	prepareAsyncCandidateServiceTest(t)
	seedAsyncCandidateServiceChannel(t, 6101, "high-a", "default", "async-candidate-model", common.ChannelStatusEnabled, 100, 1, "")
	seedAsyncCandidateServiceChannel(t, 6102, "high-b", "default", "async-candidate-model", common.ChannelStatusEnabled, 100, 10, "")
	seedAsyncCandidateServiceChannel(t, 6103, "low", "default", "async-candidate-model", common.ChannelStatusEnabled, 10, 1, "")
	seedAsyncCandidateServiceChannel(t, 6104, "disabled", "default", "async-candidate-model", common.ChannelStatusManuallyDisabled, 1000, 100, "")
	model.InitChannelCache()

	candidates, err := CacheGetAsyncChannelCandidates(&AsyncChannelCandidateParam{
		TokenGroup: "default",
		ModelName:  "async-candidate-model",
		Limit:      99,
	})
	require.NoError(t, err)
	require.Len(t, candidates, MaxAsyncChannelCandidateLimit)
	require.ElementsMatch(t, []int{6101, 6102}, []int{candidates[0].Channel.Id, candidates[1].Channel.Id})
	require.Equal(t, 6103, candidates[2].Channel.Id)
	assertDistinctAsyncCandidateIDs(t, candidates)

	candidates, err = CacheGetAsyncChannelCandidates(&AsyncChannelCandidateParam{
		TokenGroup:         "default",
		ModelName:          "async-candidate-model",
		ExcludedChannelIDs: map[int]struct{}{6101: {}},
		Availability: func(_ string, channel *model.Channel) bool {
			return channel.Id != 6102
		},
		Limit: 3,
	})
	require.NoError(t, err)
	require.Len(t, candidates, 1)
	require.Equal(t, 6103, candidates[0].Channel.Id)
}

func TestCacheGetAsyncChannelCandidatesPreservesAsyncSpecRouteSemantics(t *testing.T) {
	prepareAsyncCandidateServiceTest(t)
	matchingSetting := `{"async_spec_routes":[{"kind":"image","models":["async-spec-model"],"resolutions":["2k"]}]}`
	mismatchSetting := `{"async_spec_routes":[{"kind":"image","models":["async-spec-model"],"resolutions":["1k"]}]}`
	seedAsyncCandidateServiceChannel(t, 6201, "high-not-applicable", "default", "async-spec-model", common.ChannelStatusEnabled, 100, 1, "")
	seedAsyncCandidateServiceChannel(t, 6202, "low-match", "default", "async-spec-model", common.ChannelStatusEnabled, 10, 1, matchingSetting)
	seedAsyncCandidateServiceChannel(t, 6203, "low-mismatch", "default", "async-spec-model", common.ChannelStatusEnabled, 10, 1, mismatchSetting)
	model.InitChannelCache()

	candidates, err := CacheGetAsyncChannelCandidates(&AsyncChannelCandidateParam{
		TokenGroup: "default",
		ModelName:  "async-spec-model",
		AsyncSpec: &AsyncSpecRouteConstraint{
			Kind:       "image",
			Model:      "async-spec-model",
			Resolution: "2k",
		},
		Limit: 3,
	})
	require.NoError(t, err)
	require.Len(t, candidates, 1)
	require.Equal(t, 6202, candidates[0].Channel.Id)

	// If no channel declares a route for this kind, existing selection behavior
	// keeps every otherwise-compatible channel.
	candidates, err = CacheGetAsyncChannelCandidates(&AsyncChannelCandidateParam{
		TokenGroup: "default",
		ModelName:  "async-spec-model",
		AsyncSpec: &AsyncSpecRouteConstraint{
			Kind:       "video",
			Model:      "async-spec-model",
			Resolution: "2k",
		},
		Limit: 3,
	})
	require.NoError(t, err)
	require.Len(t, candidates, 3)
	assertDistinctAsyncCandidateIDs(t, candidates)
}

func TestCacheGetAsyncChannelCandidatesDeduplicatesAcrossAutoGroups(t *testing.T) {
	prepareAsyncCandidateServiceTest(t)
	previousAutoGroups := setting.AutoGroups2JsonString()
	require.NoError(t, setting.UpdateAutoGroupsByJsonString(`["default","vip"]`))
	t.Cleanup(func() {
		require.NoError(t, setting.UpdateAutoGroupsByJsonString(previousAutoGroups))
	})

	seedAsyncCandidateServiceChannel(t, 6301, "shared", "default", "auto-candidate-model", common.ChannelStatusEnabled, 100, 1, "")
	require.NoError(t, model.DB.Create(&model.Ability{
		Group:     "vip",
		Model:     "auto-candidate-model",
		ChannelId: 6301,
		Enabled:   true,
		Priority:  common.GetPointer[int64](100),
		Weight:    1,
	}).Error)
	seedAsyncCandidateServiceChannel(t, 6302, "vip-only", "vip", "auto-candidate-model", common.ChannelStatusEnabled, 10, 1, "")
	model.InitChannelCache()

	ctx, _ := gin.CreateTestContext(nil)
	common.SetContextKey(ctx, constant.ContextKeyUserGroup, "default")
	common.SetContextKey(ctx, constant.ContextKeyTokenCrossGroupRetry, true)
	candidates, err := CacheGetAsyncChannelCandidates(&AsyncChannelCandidateParam{
		Ctx:        ctx,
		TokenGroup: "auto",
		ModelName:  "auto-candidate-model",
		Limit:      3,
	})
	require.NoError(t, err)
	require.Len(t, candidates, 2)
	require.Equal(t, 6301, candidates[0].Channel.Id)
	require.Equal(t, "default", candidates[0].Group)
	require.Equal(t, 6302, candidates[1].Channel.Id)
	require.Equal(t, "vip", candidates[1].Group)
	assertDistinctAsyncCandidateIDs(t, candidates)
}

func prepareAsyncCandidateServiceTest(t *testing.T) {
	t.Helper()
	require.NoError(t, model.DB.AutoMigrate(&model.Ability{}))
	require.NoError(t, model.DB.Exec("DELETE FROM abilities").Error)
	require.NoError(t, model.DB.Exec("DELETE FROM channels").Error)
	previousMemoryCacheEnabled := common.MemoryCacheEnabled
	common.MemoryCacheEnabled = true
	t.Cleanup(func() {
		_ = model.DB.Exec("DELETE FROM abilities").Error
		_ = model.DB.Exec("DELETE FROM channels").Error
		common.MemoryCacheEnabled = previousMemoryCacheEnabled
		if previousMemoryCacheEnabled {
			model.InitChannelCache()
		}
	})
}

func seedAsyncCandidateServiceChannel(t *testing.T, id int, name string, group string, modelName string, status int, priority int64, weight uint, channelSetting string) {
	t.Helper()
	channel := model.Channel{
		Id:       id,
		Type:     constant.ChannelTypeOpenAI,
		Key:      fmt.Sprintf("test-key-async-candidate-%d", id),
		Status:   status,
		Name:     name,
		Models:   modelName,
		Group:    group,
		Priority: common.GetPointer(priority),
		Weight:   common.GetPointer(weight),
	}
	if channelSetting != "" {
		channel.Setting = common.GetPointer(channelSetting)
	}
	require.NoError(t, model.DB.Create(&channel).Error)
	require.NoError(t, model.DB.Create(&model.Ability{
		Group:     group,
		Model:     modelName,
		ChannelId: id,
		Enabled:   status == common.ChannelStatusEnabled,
		Priority:  common.GetPointer(priority),
		Weight:    weight,
	}).Error)
}

func assertDistinctAsyncCandidateIDs(t *testing.T, candidates []AsyncChannelCandidate) {
	t.Helper()
	seen := make(map[int]struct{}, len(candidates))
	for _, candidate := range candidates {
		require.NotNil(t, candidate.Channel)
		_, duplicate := seen[candidate.Channel.Id]
		require.False(t, duplicate, "channel %d appeared more than once", candidate.Channel.Id)
		seen[candidate.Channel.Id] = struct{}{}
	}
}
