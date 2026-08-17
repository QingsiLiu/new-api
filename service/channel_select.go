package service

import (
	"errors"
	"math/rand"
	"sort"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/gin-gonic/gin"
)

type RetryParam struct {
	Ctx          *gin.Context
	TokenGroup   string
	ModelName    string
	RequestPath  string
	Retry        *int
	AsyncSpec    *AsyncSpecRouteConstraint
	resetNextTry bool
}

type AsyncSpecRouteConstraint struct {
	Kind       string
	Model      string
	Resolution string
}

const MaxAsyncChannelCandidateLimit = 3

// AsyncChannelAvailabilityFilter is evaluated after all static routing rules
// and exclusions, and outside the channel-cache lock. It is intended for
// request-scoped health/circuit checks; returning false skips the channel.
type AsyncChannelAvailabilityFilter func(group string, channel *model.Channel) bool

type AsyncChannelCandidateParam struct {
	Ctx                *gin.Context
	TokenGroup         string
	ModelName          string
	RequestPath        string
	AsyncSpec          *AsyncSpecRouteConstraint
	ExcludedChannelIDs map[int]struct{}
	Availability       AsyncChannelAvailabilityFilter
	Limit              int
}

type AsyncChannelCandidate struct {
	Channel *model.Channel
	Group   string
}

func (p *RetryParam) GetRetry() int {
	if p.Retry == nil {
		return 0
	}
	return *p.Retry
}

func (p *RetryParam) SetRetry(retry int) {
	p.Retry = &retry
}

func (p *RetryParam) IncreaseRetry() {
	if p.resetNextTry {
		p.resetNextTry = false
		return
	}
	if p.Retry == nil {
		p.Retry = new(int)
	}
	*p.Retry++
}

func (p *RetryParam) ResetRetryNextTry() {
	p.resetNextTry = true
}

// CacheGetRandomSatisfiedChannel tries to get a random channel that satisfies the requirements.
// 尝试获取一个满足要求的随机渠道。
//
// For "auto" tokenGroup with cross-group Retry enabled:
// 对于启用了跨分组重试的 "auto" tokenGroup：
//
//   - Each group will exhaust all its priorities before moving to the next group.
//     每个分组会用完所有优先级后才会切换到下一个分组。
//
//   - Uses ContextKeyAutoGroupIndex to track current group index.
//     使用 ContextKeyAutoGroupIndex 跟踪当前分组索引。
//
//   - Uses ContextKeyAutoGroupRetryIndex to track the global Retry count when current group started.
//     使用 ContextKeyAutoGroupRetryIndex 跟踪当前分组开始时的全局重试次数。
//
//   - priorityRetry = Retry - startRetryIndex, represents the priority level within current group.
//     priorityRetry = Retry - startRetryIndex，表示当前分组内的优先级级别。
//
//   - When GetRandomSatisfiedChannel returns nil (priorities exhausted), moves to next group.
//     当 GetRandomSatisfiedChannel 返回 nil（优先级用完）时，切换到下一个分组。
//
// Example flow (2 groups, each with 2 priorities, RetryTimes=3):
// 示例流程（2个分组，每个有2个优先级，RetryTimes=3）：
//
//	Retry=0: GroupA, priority0 (startRetryIndex=0, priorityRetry=0)
//	         分组A, 优先级0
//
//	Retry=1: GroupA, priority1 (startRetryIndex=0, priorityRetry=1)
//	         分组A, 优先级1
//
//	Retry=2: GroupA exhausted → GroupB, priority0 (startRetryIndex=2, priorityRetry=0)
//	         分组A用完 → 分组B, 优先级0
//
//	Retry=3: GroupB, priority1 (startRetryIndex=2, priorityRetry=1)
//	         分组B, 优先级1
func CacheGetRandomSatisfiedChannel(param *RetryParam) (*model.Channel, string, error) {
	var channel *model.Channel
	var err error
	selectGroup := param.TokenGroup
	userGroup := common.GetContextKeyString(param.Ctx, constant.ContextKeyUserGroup)
	selectionFilter := asyncSpecRouteSelectionFilter(param.AsyncSpec)

	if param.TokenGroup == "auto" {
		if len(setting.GetAutoGroups()) == 0 {
			return nil, selectGroup, errors.New("auto groups is not enabled")
		}
		autoGroups := GetUserAutoGroup(userGroup)

		// startGroupIndex: the group index to start searching from
		// startGroupIndex: 开始搜索的分组索引
		startGroupIndex := 0
		crossGroupRetry := common.GetContextKeyBool(param.Ctx, constant.ContextKeyTokenCrossGroupRetry)

		if lastGroupIndex, exists := common.GetContextKey(param.Ctx, constant.ContextKeyAutoGroupIndex); exists {
			if idx, ok := lastGroupIndex.(int); ok {
				startGroupIndex = idx
			}
		}

		for i := startGroupIndex; i < len(autoGroups); i++ {
			autoGroup := autoGroups[i]
			// Calculate priorityRetry for current group
			// 计算当前分组的 priorityRetry
			priorityRetry := param.GetRetry()
			// If moved to a new group, reset priorityRetry and update startRetryIndex
			// 如果切换到新分组，重置 priorityRetry 并更新 startRetryIndex
			if i > startGroupIndex {
				priorityRetry = 0
			}
			logger.LogDebug(param.Ctx, "Auto selecting group: %s, priorityRetry: %d", autoGroup, priorityRetry)

			channel, _ = model.GetRandomSatisfiedChannelWithSelectionFilter(autoGroup, param.ModelName, priorityRetry, param.RequestPath, selectionFilter)
			if channel == nil {
				// Current group has no available channel for this model, try next group
				// 当前分组没有该模型的可用渠道，尝试下一个分组
				logger.LogDebug(param.Ctx, "No available channel in group %s for model %s at priorityRetry %d, trying next group", autoGroup, param.ModelName, priorityRetry)
				// 重置状态以尝试下一个分组
				common.SetContextKey(param.Ctx, constant.ContextKeyAutoGroupIndex, i+1)
				common.SetContextKey(param.Ctx, constant.ContextKeyAutoGroupRetryIndex, 0)
				// Reset retry counter so outer loop can continue for next group
				// 重置重试计数器，以便外层循环可以为下一个分组继续
				param.SetRetry(0)
				continue
			}
			common.SetContextKey(param.Ctx, constant.ContextKeyAutoGroup, autoGroup)
			selectGroup = autoGroup
			logger.LogDebug(param.Ctx, "Auto selected group: %s", autoGroup)

			// Prepare state for next retry
			// 为下一次重试准备状态
			if crossGroupRetry && priorityRetry >= common.RetryTimes {
				// Current group has exhausted all retries, prepare to switch to next group
				// This request still uses current group, but next retry will use next group
				// 当前分组已用完所有重试次数，准备切换到下一个分组
				// 本次请求仍使用当前分组，但下次重试将使用下一个分组
				logger.LogDebug(param.Ctx, "Current group %s retries exhausted (priorityRetry=%d >= RetryTimes=%d), preparing switch to next group for next retry", autoGroup, priorityRetry, common.RetryTimes)
				common.SetContextKey(param.Ctx, constant.ContextKeyAutoGroupIndex, i+1)
				// Reset retry counter so outer loop can continue for next group
				// 重置重试计数器，以便外层循环可以为下一个分组继续
				param.SetRetry(0)
				param.ResetRetryNextTry()
			} else {
				// Stay in current group, save current state
				// 保持在当前分组，保存当前状态
				common.SetContextKey(param.Ctx, constant.ContextKeyAutoGroupIndex, i)
			}
			break
		}
	} else {
		channel, err = model.GetRandomSatisfiedChannelWithSelectionFilter(param.TokenGroup, param.ModelName, param.GetRetry(), param.RequestPath, selectionFilter)
		if err != nil {
			return nil, param.TokenGroup, err
		}
	}
	return channel, selectGroup, nil
}

// CacheGetAsyncChannelCandidates builds an async-only, bounded failover plan.
// Existing RetryParam/GetRandomSatisfiedChannel behavior is deliberately left
// untouched. Within each priority, candidates are sampled by weight without
// replacement; priorities and auto groups remain ordered.
func CacheGetAsyncChannelCandidates(param *AsyncChannelCandidateParam) ([]AsyncChannelCandidate, error) {
	if param == nil {
		return nil, errors.New("async channel candidate parameters are required")
	}
	modelName := strings.TrimSpace(param.ModelName)
	if modelName == "" {
		return nil, errors.New("model is required")
	}
	limit := normalizeAsyncChannelCandidateLimit(param.Limit)
	tokenGroup := strings.TrimSpace(param.TokenGroup)
	if tokenGroup == "" {
		tokenGroup = "default"
	}

	groups, crossGroupRetry, err := asyncCandidateGroups(param.Ctx, tokenGroup)
	if err != nil {
		return nil, err
	}
	selectionFilter := asyncSpecRouteSelectionFilter(param.AsyncSpec)
	result := make([]AsyncChannelCandidate, 0, limit)
	seenChannelIDs := make(map[int]struct{}, limit)

	for _, group := range groups {
		modelCandidates, err := model.GetSatisfiedChannelCandidatesWithSelectionFilter(
			group,
			modelName,
			param.RequestPath,
			selectionFilter,
		)
		if err != nil {
			return nil, err
		}
		available := make([]model.ChannelCandidate, 0, len(modelCandidates))
		for _, candidate := range modelCandidates {
			if candidate.Channel == nil {
				continue
			}
			channelID := candidate.Channel.Id
			if _, excluded := param.ExcludedChannelIDs[channelID]; excluded {
				continue
			}
			if _, duplicate := seenChannelIDs[channelID]; duplicate {
				continue
			}
			if param.Availability != nil && !param.Availability(group, candidate.Channel) {
				continue
			}
			seenChannelIDs[channelID] = struct{}{}
			available = append(available, candidate)
		}

		ordered := weightedAsyncCandidatesWithoutReplacement(available)
		for _, candidate := range ordered {
			result = append(result, AsyncChannelCandidate{Channel: candidate.Channel, Group: group})
			if len(result) >= limit {
				return result, nil
			}
		}

		// Existing auto-group behavior stays in the first group that can serve
		// the request unless the token explicitly enables cross-group retry.
		if tokenGroup == "auto" && len(available) > 0 && !crossGroupRetry {
			break
		}
	}
	return result, nil
}

func asyncCandidateGroups(ctx *gin.Context, tokenGroup string) ([]string, bool, error) {
	if tokenGroup != "auto" {
		return []string{tokenGroup}, false, nil
	}
	if len(setting.GetAutoGroups()) == 0 {
		return nil, false, errors.New("auto groups is not enabled")
	}
	userGroup := ""
	crossGroupRetry := false
	if ctx != nil {
		userGroup = common.GetContextKeyString(ctx, constant.ContextKeyUserGroup)
		crossGroupRetry = common.GetContextKeyBool(ctx, constant.ContextKeyTokenCrossGroupRetry)
	}
	return GetUserAutoGroup(userGroup), crossGroupRetry, nil
}

func normalizeAsyncChannelCandidateLimit(limit int) int {
	if limit <= 0 || limit > MaxAsyncChannelCandidateLimit {
		return MaxAsyncChannelCandidateLimit
	}
	return limit
}

func weightedAsyncCandidatesWithoutReplacement(candidates []model.ChannelCandidate) []model.ChannelCandidate {
	if len(candidates) <= 1 {
		return append([]model.ChannelCandidate(nil), candidates...)
	}
	pool := append([]model.ChannelCandidate(nil), candidates...)
	sort.SliceStable(pool, func(i, j int) bool {
		if pool[i].Priority == pool[j].Priority {
			return pool[i].Channel.Id < pool[j].Channel.Id
		}
		return pool[i].Priority > pool[j].Priority
	})

	ordered := make([]model.ChannelCandidate, 0, len(pool))
	for len(pool) > 0 {
		priority := pool[0].Priority
		bucketEnd := 1
		for bucketEnd < len(pool) && pool[bucketEnd].Priority == priority {
			bucketEnd++
		}
		bucket := append([]model.ChannelCandidate(nil), pool[:bucketEnd]...)
		for len(bucket) > 0 {
			selected := weightedAsyncCandidateIndex(bucket)
			ordered = append(ordered, bucket[selected])
			bucket = append(bucket[:selected], bucket[selected+1:]...)
		}
		pool = pool[bucketEnd:]
	}
	return ordered
}

func weightedAsyncCandidateIndex(candidates []model.ChannelCandidate) int {
	var total int64
	for _, candidate := range candidates {
		if candidate.Weight > 0 {
			total += int64(candidate.Weight)
		}
	}
	if total <= 0 {
		return rand.Intn(len(candidates))
	}
	draw := rand.Int63n(total)
	for index, candidate := range candidates {
		if candidate.Weight <= 0 {
			continue
		}
		draw -= int64(candidate.Weight)
		if draw < 0 {
			return index
		}
	}
	return len(candidates) - 1
}

func asyncSpecRouteSelectionFilter(constraint *AsyncSpecRouteConstraint) model.ChannelSelectionFilter {
	if constraint == nil {
		return nil
	}
	kind := strings.ToLower(strings.TrimSpace(constraint.Kind))
	modelName := strings.TrimSpace(constraint.Model)
	resolution := operation_setting.ResolveImageSpecKey(strings.TrimSpace(constraint.Resolution), "")
	if kind == "" || modelName == "" {
		return nil
	}
	return func(channel *model.Channel) model.ChannelSelectionFilterResult {
		if channel == nil {
			return model.ChannelSelectionFilterResult{}
		}
		settings := channel.GetSetting()
		applies := false
		for _, route := range settings.AsyncSpecRoutes {
			if strings.ToLower(strings.TrimSpace(route.Kind)) != kind {
				continue
			}
			if !asyncSpecRouteContainsModel(route.Models, modelName) {
				continue
			}
			applies = true
			if resolution != "" && asyncSpecRouteContainsResolution(route.Resolutions, resolution) {
				return model.ChannelSelectionFilterResult{Applies: true, Match: true}
			}
		}
		return model.ChannelSelectionFilterResult{Applies: applies, Match: false}
	}
}

func asyncSpecRouteContainsModel(models []string, modelName string) bool {
	for _, candidate := range models {
		if strings.EqualFold(strings.TrimSpace(candidate), modelName) {
			return true
		}
	}
	return false
}

func asyncSpecRouteContainsResolution(resolutions []string, resolution string) bool {
	for _, candidate := range resolutions {
		if operation_setting.ResolveImageSpecKey(strings.TrimSpace(candidate), "") == resolution {
			return true
		}
	}
	return false
}
