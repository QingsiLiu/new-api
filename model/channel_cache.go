package model

import (
	"errors"
	"fmt"
	"math/rand"
	"sort"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
)

var group2model2channels map[string]map[string][]int // enabled channel
var channelsIDM map[int]*Channel                     // all channels include disabled
// channel2advancedCustomConfig caches parsed Advanced Custom (type 58) configs so
// path-aware selection avoids re-parsing JSON per request. Refreshed on full sync.
var channel2advancedCustomConfig map[int]*dto.AdvancedCustomConfig
var channelSyncLock sync.RWMutex

type ChannelSelectionFilterResult struct {
	Applies bool
	Match   bool
}

type ChannelSelectionFilter func(*Channel) ChannelSelectionFilterResult

// ChannelCandidate is a detached selection snapshot used by the async media
// failover planner. Channel is copied from the cache (or loaded from the DB), so
// callers never retain a pointer to the shared channel cache after its lock is
// released.
type ChannelCandidate struct {
	Channel  *Channel
	Priority int64
	Weight   int
}

func InitChannelCache() {
	if !common.MemoryCacheEnabled {
		InvalidatePricingCache()
		return
	}
	newChannelId2channel := make(map[int]*Channel)
	newChannel2advancedCustomConfig := make(map[int]*dto.AdvancedCustomConfig)
	var channels []*Channel
	DB.Find(&channels)
	for _, channel := range channels {
		newChannelId2channel[channel.Id] = channel
		if channel.Type == constant.ChannelTypeAdvancedCustom {
			if config := channel.GetOtherSettings().AdvancedCustom; config != nil {
				newChannel2advancedCustomConfig[channel.Id] = config
			}
		}
	}
	newGroup2model2channels := make(map[string]map[string][]int)
	var abilities []*Ability
	DB.Find(&abilities, "enabled = ?", true)
	for _, ability := range abilities {
		channel, ok := newChannelId2channel[ability.ChannelId]
		if !ok || channel.Status != common.ChannelStatusEnabled {
			continue
		}
		if _, ok := newGroup2model2channels[ability.Group]; !ok {
			newGroup2model2channels[ability.Group] = make(map[string][]int)
		}
		if _, ok := newGroup2model2channels[ability.Group][ability.Model]; !ok {
			newGroup2model2channels[ability.Group][ability.Model] = make([]int, 0)
		}
		newGroup2model2channels[ability.Group][ability.Model] = append(newGroup2model2channels[ability.Group][ability.Model], channel.Id)
	}

	// sort by priority
	for group, model2channels := range newGroup2model2channels {
		for model, channels := range model2channels {
			sort.Slice(channels, func(i, j int) bool {
				return newChannelId2channel[channels[i]].GetPriority() > newChannelId2channel[channels[j]].GetPriority()
			})
			newGroup2model2channels[group][model] = channels
		}
	}

	channelSyncLock.Lock()
	group2model2channels = newGroup2model2channels
	//channelsIDM = newChannelId2channel
	for i, channel := range newChannelId2channel {
		if channel.ChannelInfo.IsMultiKey {
			channel.Keys = channel.GetKeys()
			if channel.ChannelInfo.MultiKeyMode == constant.MultiKeyModePolling {
				if oldChannel, ok := channelsIDM[i]; ok {
					// 存在旧的渠道，如果是多key且轮询，保留轮询索引信息
					if oldChannel.ChannelInfo.IsMultiKey && oldChannel.ChannelInfo.MultiKeyMode == constant.MultiKeyModePolling {
						channel.ChannelInfo.MultiKeyPollingIndex = oldChannel.ChannelInfo.MultiKeyPollingIndex
					}
				}
			}
		}
	}
	channelsIDM = newChannelId2channel
	channel2advancedCustomConfig = newChannel2advancedCustomConfig
	channelSyncLock.Unlock()
	// Lock ordering: InvalidatePricingCache acquires updatePricingLock, and
	// GetPricing (holding updatePricingLock) nests channelSyncLock.RLock via
	// loadPricingAdvancedCustomConfigs. channelSyncLock MUST be released before
	// invalidating the pricing cache, otherwise the reversed order deadlocks.
	InvalidatePricingCache()
	common.SysLog("channels synced from database")
}

func SyncChannelCache(frequency int) {
	for {
		time.Sleep(time.Duration(frequency) * time.Second)
		common.SysLog("syncing channels from database")
		InitChannelCache()
	}
}

func GetRandomSatisfiedChannel(group string, model string, retry int, requestPath string) (*Channel, error) {
	return GetRandomSatisfiedChannelWithSelectionFilter(group, model, retry, requestPath, nil)
}

// GetSatisfiedChannelCandidatesWithSelectionFilter returns every enabled
// channel that can serve group/model/requestPath, ordered by descending
// priority. It intentionally does not randomize candidates; async-only callers
// perform weighted sampling without replacement after applying request-scoped
// exclusions and health filters.
//
// Exact model abilities take precedence. The normalized model name is consulted
// only when the exact name has no enabled, path-compatible channels, matching
// the existing single-channel selection behavior.
func GetSatisfiedChannelCandidatesWithSelectionFilter(group string, modelName string, requestPath string, filter ChannelSelectionFilter) ([]ChannelCandidate, error) {
	if !common.MemoryCacheEnabled {
		return getSatisfiedChannelCandidatesFromDB(group, modelName, requestPath, filter)
	}
	return getSatisfiedChannelCandidatesFromCache(group, modelName, requestPath, filter)
}

func getSatisfiedChannelCandidatesFromCache(group string, modelName string, requestPath string, filter ChannelSelectionFilter) ([]ChannelCandidate, error) {
	channelSyncLock.RLock()
	defer channelSyncLock.RUnlock()

	channelIDs := cachedCandidateChannelIDs(group, modelName, requestPath)
	if len(channelIDs) == 0 {
		return []ChannelCandidate{}, nil
	}

	filteredIDs, err := applyChannelSelectionFilter(channelIDs, filter, func(channelID int) (*Channel, bool) {
		channel, ok := channelsIDM[channelID]
		return channel, ok
	})
	if err != nil {
		return nil, err
	}

	candidates := make([]ChannelCandidate, 0, len(filteredIDs))
	seen := make(map[int]struct{}, len(filteredIDs))
	for _, channelID := range filteredIDs {
		if _, ok := seen[channelID]; ok {
			continue
		}
		channel, ok := channelsIDM[channelID]
		if !ok {
			return nil, fmt.Errorf("数据库一致性错误，渠道# %d 不存在，请联系管理员修复", channelID)
		}
		if channel.Status != common.ChannelStatusEnabled {
			continue
		}
		seen[channelID] = struct{}{}
		channelCopy := cloneChannelForSelection(channel)
		candidates = append(candidates, ChannelCandidate{
			Channel:  channelCopy,
			Priority: channelCopy.GetPriority(),
			Weight:   channelCopy.GetWeight(),
		})
	}
	orderChannelCandidates(candidates)
	return candidates, nil
}

func cachedCandidateChannelIDs(group string, modelName string, requestPath string) []int {
	if group2model2channels == nil {
		return nil
	}
	channelIDs := filterChannelsByRequestPathAndModel(group2model2channels[group][modelName], requestPath, modelName)
	if len(channelIDs) > 0 {
		return channelIDs
	}
	normalizedModelName := ratio_setting.FormatMatchingModelName(modelName)
	if normalizedModelName == "" || normalizedModelName == modelName {
		return nil
	}
	return filterChannelsByRequestPathAndModel(group2model2channels[group][normalizedModelName], requestPath, modelName)
}

func getSatisfiedChannelCandidatesFromDB(group string, modelName string, requestPath string, filter ChannelSelectionFilter) ([]ChannelCandidate, error) {
	candidates, err := getExactSatisfiedChannelCandidatesFromDB(group, modelName, modelName, requestPath)
	if err != nil {
		return nil, err
	}
	if len(candidates) == 0 {
		normalizedModelName := ratio_setting.FormatMatchingModelName(modelName)
		if normalizedModelName != "" && normalizedModelName != modelName {
			candidates, err = getExactSatisfiedChannelCandidatesFromDB(group, normalizedModelName, modelName, requestPath)
			if err != nil {
				return nil, err
			}
		}
	}
	if len(candidates) == 0 {
		return []ChannelCandidate{}, nil
	}

	channelsByID := make(map[int]*Channel, len(candidates))
	channelIDs := make([]int, 0, len(candidates))
	for _, candidate := range candidates {
		if candidate.Channel == nil {
			continue
		}
		channelsByID[candidate.Channel.Id] = candidate.Channel
		channelIDs = append(channelIDs, candidate.Channel.Id)
	}
	filteredIDs, err := applyChannelSelectionFilter(channelIDs, filter, func(channelID int) (*Channel, bool) {
		channel, ok := channelsByID[channelID]
		return channel, ok
	})
	if err != nil {
		return nil, err
	}
	allowed := make(map[int]struct{}, len(filteredIDs))
	for _, channelID := range filteredIDs {
		allowed[channelID] = struct{}{}
	}
	filtered := make([]ChannelCandidate, 0, len(filteredIDs))
	for _, candidate := range candidates {
		if candidate.Channel == nil {
			continue
		}
		if _, ok := allowed[candidate.Channel.Id]; ok {
			filtered = append(filtered, candidate)
		}
	}
	orderChannelCandidates(filtered)
	return filtered, nil
}

func getExactSatisfiedChannelCandidatesFromDB(group string, abilityModelName string, requestModelName string, requestPath string) ([]ChannelCandidate, error) {
	abilities := make([]Ability, 0)
	if err := DB.Find(&abilities, commonGroupCol+" = ? and model = ? and enabled = ?", group, abilityModelName, true).Error; err != nil {
		return nil, err
	}
	if len(abilities) == 0 {
		return []ChannelCandidate{}, nil
	}

	channelIDs := make([]int, 0, len(abilities))
	seen := make(map[int]struct{}, len(abilities))
	for _, ability := range abilities {
		if _, ok := seen[ability.ChannelId]; ok {
			continue
		}
		seen[ability.ChannelId] = struct{}{}
		channelIDs = append(channelIDs, ability.ChannelId)
	}
	channels := make([]Channel, 0, len(channelIDs))
	if err := DB.Where("id IN ?", channelIDs).Find(&channels).Error; err != nil {
		return nil, err
	}

	candidates := make([]ChannelCandidate, 0, len(channels))
	for i := range channels {
		channel := &channels[i]
		if channel.Status != common.ChannelStatusEnabled {
			continue
		}
		if requestPath != "" && channel.Type == constant.ChannelTypeAdvancedCustom {
			config := channel.GetOtherSettings().AdvancedCustom
			if config == nil || !config.SupportsPathForModel(requestPath, requestModelName) {
				continue
			}
		}
		channelCopy := cloneChannelForSelection(channel)
		candidates = append(candidates, ChannelCandidate{
			Channel:  channelCopy,
			Priority: channelCopy.GetPriority(),
			Weight:   channelCopy.GetWeight(),
		})
	}
	return candidates, nil
}

func orderChannelCandidates(candidates []ChannelCandidate) {
	sort.SliceStable(candidates, func(i, j int) bool {
		if candidates[i].Priority == candidates[j].Priority {
			return candidates[i].Channel.Id < candidates[j].Channel.Id
		}
		return candidates[i].Priority > candidates[j].Priority
	})
}

func cloneChannelForSelection(channel *Channel) *Channel {
	if channel == nil {
		return nil
	}
	cloned := *channel
	cloned.OpenAIOrganization = cloneSelectionPointer(channel.OpenAIOrganization)
	cloned.TestModel = cloneSelectionPointer(channel.TestModel)
	cloned.Weight = cloneSelectionPointer(channel.Weight)
	cloned.BaseURL = cloneSelectionPointer(channel.BaseURL)
	cloned.ModelMapping = cloneSelectionPointer(channel.ModelMapping)
	cloned.StatusCodeMapping = cloneSelectionPointer(channel.StatusCodeMapping)
	cloned.Priority = cloneSelectionPointer(channel.Priority)
	cloned.AutoBan = cloneSelectionPointer(channel.AutoBan)
	cloned.Tag = cloneSelectionPointer(channel.Tag)
	cloned.Setting = cloneSelectionPointer(channel.Setting)
	cloned.ParamOverride = cloneSelectionPointer(channel.ParamOverride)
	cloned.HeaderOverride = cloneSelectionPointer(channel.HeaderOverride)
	cloned.Remark = cloneSelectionPointer(channel.Remark)
	cloned.Keys = append([]string(nil), channel.Keys...)
	cloned.ChannelInfo.MultiKeyStatusList = cloneIntMap(channel.ChannelInfo.MultiKeyStatusList)
	cloned.ChannelInfo.MultiKeyDisabledReason = cloneStringMap(channel.ChannelInfo.MultiKeyDisabledReason)
	cloned.ChannelInfo.MultiKeyDisabledTime = cloneInt64Map(channel.ChannelInfo.MultiKeyDisabledTime)
	return &cloned
}

func cloneSelectionPointer[T any](source *T) *T {
	if source == nil {
		return nil
	}
	cloned := *source
	return &cloned
}

func cloneIntMap(source map[int]int) map[int]int {
	if source == nil {
		return nil
	}
	cloned := make(map[int]int, len(source))
	for key, value := range source {
		cloned[key] = value
	}
	return cloned
}

func cloneStringMap(source map[int]string) map[int]string {
	if source == nil {
		return nil
	}
	cloned := make(map[int]string, len(source))
	for key, value := range source {
		cloned[key] = value
	}
	return cloned
}

func cloneInt64Map(source map[int]int64) map[int]int64 {
	if source == nil {
		return nil
	}
	cloned := make(map[int]int64, len(source))
	for key, value := range source {
		cloned[key] = value
	}
	return cloned
}

func GetRandomSatisfiedChannelWithSelectionFilter(group string, model string, retry int, requestPath string, filter ChannelSelectionFilter) (*Channel, error) {
	// if memory cache is disabled, get channel directly from database
	if !common.MemoryCacheEnabled {
		if filter == nil {
			return GetChannel(group, model, retry, requestPath)
		}
		return getChannelWithSelectionFilter(group, model, retry, filter)
	}

	channelSyncLock.RLock()
	defer channelSyncLock.RUnlock()

	// First, try to find channels with the exact model name.
	channels := filterChannelsByRequestPathAndModel(group2model2channels[group][model], requestPath, model)

	// If no channels found, try to find channels with the normalized model name.
	if len(channels) == 0 {
		normalizedModel := ratio_setting.FormatMatchingModelName(model)
		channels = filterChannelsByRequestPathAndModel(group2model2channels[group][normalizedModel], requestPath, model)
	}

	if len(channels) == 0 {
		return nil, nil
	}

	var err error
	channels, err = applyChannelSelectionFilter(channels, filter, func(channelID int) (*Channel, bool) {
		channel, ok := channelsIDM[channelID]
		return channel, ok
	})
	if err != nil {
		return nil, err
	}
	if len(channels) == 0 {
		return nil, nil
	}

	if len(channels) == 1 {
		if channel, ok := channelsIDM[channels[0]]; ok {
			return channel, nil
		}
		return nil, fmt.Errorf("数据库一致性错误，渠道# %d 不存在，请联系管理员修复", channels[0])
	}

	uniquePriorities := make(map[int]bool)
	for _, channelId := range channels {
		if channel, ok := channelsIDM[channelId]; ok {
			uniquePriorities[int(channel.GetPriority())] = true
		} else {
			return nil, fmt.Errorf("数据库一致性错误，渠道# %d 不存在，请联系管理员修复", channelId)
		}
	}
	var sortedUniquePriorities []int
	for priority := range uniquePriorities {
		sortedUniquePriorities = append(sortedUniquePriorities, priority)
	}
	sort.Sort(sort.Reverse(sort.IntSlice(sortedUniquePriorities)))

	if retry >= len(uniquePriorities) {
		retry = len(uniquePriorities) - 1
	}
	targetPriority := int64(sortedUniquePriorities[retry])

	// get the priority for the given retry number
	var sumWeight = 0
	var targetChannels []*Channel
	for _, channelId := range channels {
		if channel, ok := channelsIDM[channelId]; ok {
			if channel.GetPriority() == targetPriority {
				sumWeight += channel.GetWeight()
				targetChannels = append(targetChannels, channel)
			}
		} else {
			return nil, fmt.Errorf("数据库一致性错误，渠道# %d 不存在，请联系管理员修复", channelId)
		}
	}

	if len(targetChannels) == 0 {
		return nil, errors.New(fmt.Sprintf("no channel found, group: %s, model: %s, priority: %d", group, model, targetPriority))
	}

	// smoothing factor and adjustment
	smoothingFactor := 1
	smoothingAdjustment := 0

	if sumWeight == 0 {
		// when all channels have weight 0, set sumWeight to the number of channels and set smoothing adjustment to 100
		// each channel's effective weight = 100
		sumWeight = len(targetChannels) * 100
		smoothingAdjustment = 100
	} else if sumWeight/len(targetChannels) < 10 {
		// when the average weight is less than 10, set smoothing factor to 100
		smoothingFactor = 100
	}

	// Calculate the total weight of all channels up to endIdx
	totalWeight := sumWeight * smoothingFactor

	// Generate a random value in the range [0, totalWeight)
	randomWeight := rand.Intn(totalWeight)

	// Find a channel based on its weight
	for _, channel := range targetChannels {
		randomWeight -= channel.GetWeight()*smoothingFactor + smoothingAdjustment
		if randomWeight < 0 {
			return channel, nil
		}
	}
	// return null if no channel is not found
	return nil, errors.New("channel not found")
}

func applyChannelSelectionFilter(channelIDs []int, filter ChannelSelectionFilter, lookup func(int) (*Channel, bool)) ([]int, error) {
	if filter == nil {
		return channelIDs, nil
	}
	hasApplicableRoute := false
	matchedChannelIDs := make([]int, 0, len(channelIDs))
	for _, channelID := range channelIDs {
		channel, ok := lookup(channelID)
		if !ok {
			return nil, fmt.Errorf("数据库一致性错误，渠道# %d 不存在，请联系管理员修复", channelID)
		}
		result := filter(channel)
		if !result.Applies {
			continue
		}
		hasApplicableRoute = true
		if result.Match {
			matchedChannelIDs = append(matchedChannelIDs, channelID)
		}
	}
	if !hasApplicableRoute {
		return channelIDs, nil
	}
	return matchedChannelIDs, nil
}

func getChannelWithSelectionFilter(group string, modelName string, retry int, filter ChannelSelectionFilter) (*Channel, error) {
	abilities, err := getAbilitiesForChannelSelection(group, modelName)
	if err != nil {
		return nil, err
	}
	if len(abilities) == 0 {
		return nil, nil
	}
	abilities, channelsByID, err := applyAbilitySelectionFilter(abilities, filter)
	if err != nil {
		return nil, err
	}
	if len(abilities) == 0 {
		return nil, nil
	}

	uniquePriorities := make(map[int64]bool)
	for _, ability := range abilities {
		uniquePriorities[abilityPriority(ability)] = true
	}
	sortedUniquePriorities := make([]int64, 0, len(uniquePriorities))
	for priority := range uniquePriorities {
		sortedUniquePriorities = append(sortedUniquePriorities, priority)
	}
	sort.Slice(sortedUniquePriorities, func(i, j int) bool {
		return sortedUniquePriorities[i] > sortedUniquePriorities[j]
	})
	if retry >= len(sortedUniquePriorities) {
		retry = len(sortedUniquePriorities) - 1
	}
	targetPriority := sortedUniquePriorities[retry]

	targetAbilities := make([]Ability, 0)
	weightSum := 0
	for _, ability := range abilities {
		if abilityPriority(ability) == targetPriority {
			targetAbilities = append(targetAbilities, ability)
			weightSum += int(ability.Weight) + 10
		}
	}
	if len(targetAbilities) == 0 {
		return nil, fmt.Errorf("no channel found, group: %s, model: %s, priority: %d", group, modelName, targetPriority)
	}

	weight := common.GetRandomInt(weightSum)
	selectedChannelID := targetAbilities[0].ChannelId
	for _, ability := range targetAbilities {
		weight -= int(ability.Weight) + 10
		if weight <= 0 {
			selectedChannelID = ability.ChannelId
			break
		}
	}
	if channel, ok := channelsByID[selectedChannelID]; ok {
		return channel, nil
	}
	channel := Channel{}
	if err := DB.First(&channel, "id = ?", selectedChannelID).Error; err != nil {
		return nil, err
	}
	return &channel, nil
}

func getAbilitiesForChannelSelection(group string, modelName string) ([]Ability, error) {
	abilities := make([]Ability, 0)
	if err := DB.Find(&abilities, commonGroupCol+" = ? and model = ? and enabled = ?", group, modelName, true).Error; err != nil {
		return nil, err
	}
	if len(abilities) > 0 {
		return abilities, nil
	}
	normalizedModelName := ratio_setting.FormatMatchingModelName(modelName)
	if normalizedModelName == modelName {
		return abilities, nil
	}
	if err := DB.Find(&abilities, commonGroupCol+" = ? and model = ? and enabled = ?", group, normalizedModelName, true).Error; err != nil {
		return nil, err
	}
	return abilities, nil
}

func applyAbilitySelectionFilter(abilities []Ability, filter ChannelSelectionFilter) ([]Ability, map[int]*Channel, error) {
	channelIDs := make([]int, 0, len(abilities))
	seenChannelIDs := map[int]bool{}
	for _, ability := range abilities {
		if seenChannelIDs[ability.ChannelId] {
			continue
		}
		seenChannelIDs[ability.ChannelId] = true
		channelIDs = append(channelIDs, ability.ChannelId)
	}
	channels := make([]Channel, 0, len(channelIDs))
	if err := DB.Find(&channels, "id in ?", channelIDs).Error; err != nil {
		return nil, nil, err
	}
	channelsByID := make(map[int]*Channel, len(channels))
	for i := range channels {
		channel := channels[i]
		channelsByID[channel.Id] = &channel
	}
	filteredChannelIDs, err := applyChannelSelectionFilter(channelIDs, filter, func(channelID int) (*Channel, bool) {
		channel, ok := channelsByID[channelID]
		return channel, ok
	})
	if err != nil {
		return nil, nil, err
	}
	if filter == nil || len(filteredChannelIDs) == len(channelIDs) {
		return abilities, channelsByID, nil
	}
	filteredSet := make(map[int]bool, len(filteredChannelIDs))
	for _, channelID := range filteredChannelIDs {
		filteredSet[channelID] = true
	}
	filteredAbilities := make([]Ability, 0, len(abilities))
	for _, ability := range abilities {
		if filteredSet[ability.ChannelId] {
			filteredAbilities = append(filteredAbilities, ability)
		}
	}
	return filteredAbilities, channelsByID, nil
}

func abilityPriority(ability Ability) int64 {
	if ability.Priority == nil {
		return 0
	}
	return *ability.Priority
}

// filterChannelsByRequestPathAndModel restricts candidates by request path and
// model. Only Advanced Custom (type 58) channels are path-checked: they are kept
// only when one of their configured routes matches requestPath and model. All
// other channel types always pass. When requestPath is empty, filtering is skipped.
// Caller must hold channelSyncLock (read lock). The cached slice is never mutated.
func filterChannelsByRequestPathAndModel(channels []int, requestPath string, model string) []int {
	if requestPath == "" || len(channels) == 0 {
		return channels
	}
	filtered := make([]int, 0, len(channels))
	for _, channelId := range channels {
		channel, ok := channelsIDM[channelId]
		if !ok {
			// keep it so the downstream consistency error is raised as before
			filtered = append(filtered, channelId)
			continue
		}
		if channel.Type != constant.ChannelTypeAdvancedCustom {
			filtered = append(filtered, channelId)
			continue
		}
		if config := channel2advancedCustomConfig[channelId]; config != nil && config.SupportsPathForModel(requestPath, model) {
			filtered = append(filtered, channelId)
		}
	}
	return filtered
}

func CacheGetChannel(id int) (*Channel, error) {
	if !common.MemoryCacheEnabled {
		return GetChannelById(id, true)
	}
	channelSyncLock.RLock()
	defer channelSyncLock.RUnlock()

	c, ok := channelsIDM[id]
	if !ok {
		return nil, fmt.Errorf("渠道# %d，已不存在", id)
	}
	return c, nil
}

func CacheGetChannelInfo(id int) (*ChannelInfo, error) {
	if !common.MemoryCacheEnabled {
		channel, err := GetChannelById(id, true)
		if err != nil {
			return nil, err
		}
		return &channel.ChannelInfo, nil
	}
	channelSyncLock.RLock()
	defer channelSyncLock.RUnlock()

	c, ok := channelsIDM[id]
	if !ok {
		return nil, fmt.Errorf("渠道# %d，已不存在", id)
	}
	return &c.ChannelInfo, nil
}

func CacheUpdateChannelStatus(id int, status int) {
	if !common.MemoryCacheEnabled {
		return
	}
	channelSyncLock.Lock()
	defer channelSyncLock.Unlock()
	if channel, ok := channelsIDM[id]; ok {
		channel.Status = status
	}
	if status != common.ChannelStatusEnabled {
		// delete the channel from group2model2channels
		for group, model2channels := range group2model2channels {
			for model, channels := range model2channels {
				for i, channelId := range channels {
					if channelId == id {
						// remove the channel from the slice
						group2model2channels[group][model] = append(channels[:i], channels[i+1:]...)
						break
					}
				}
			}
		}
	}
}

func CacheUpdateChannel(channel *Channel) {
	if !common.MemoryCacheEnabled {
		return
	}
	channelSyncLock.Lock()
	if channel == nil {
		channelSyncLock.Unlock()
		return
	}

	if channelsIDM == nil {
		channelsIDM = make(map[int]*Channel)
	}
	if oldChannel, ok := channelsIDM[channel.Id]; ok {
		logger.LogDebug(nil, "CacheUpdateChannel before: id=%d, name=%s, status=%d, polling_index=%d", channel.Id, channel.Name, channel.Status, oldChannel.ChannelInfo.MultiKeyPollingIndex)
	}
	channelsIDM[channel.Id] = channel
	if channel2advancedCustomConfig == nil {
		channel2advancedCustomConfig = make(map[int]*dto.AdvancedCustomConfig)
	}
	delete(channel2advancedCustomConfig, channel.Id)
	if channel.Type == constant.ChannelTypeAdvancedCustom {
		if config := channel.GetOtherSettings().AdvancedCustom; config != nil {
			channel2advancedCustomConfig[channel.Id] = config
		}
	}
	logger.LogDebug(nil, "CacheUpdateChannel after: id=%d, name=%s, status=%d, polling_index=%d", channel.Id, channel.Name, channel.Status, channel.ChannelInfo.MultiKeyPollingIndex)
	// Lock ordering: do NOT hold channelSyncLock while calling
	// InvalidatePricingCache. GetPricing acquires updatePricingLock first and then
	// channelSyncLock.RLock (via loadPricingAdvancedCustomConfigs); acquiring
	// updatePricingLock while holding channelSyncLock would be an AB-BA deadlock.
	channelSyncLock.Unlock()
	InvalidatePricingCache()
}
