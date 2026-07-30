package model

import (
	"errors"
	"sort"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
)

const settledUsageSearchLimit = logSearchCountLimit

func nonTaskUsageScope(tx *gorm.DB) *gorm.DB {
	return tx.Where(
		"(other IS NULL OR (other NOT LIKE ? AND other NOT LIKE ?))",
		`%"is_task":true%`,
		`%"task_id":%`,
	)
}

func usageTextMatches(value string, filter string) bool {
	if filter == "" {
		return true
	}
	if !strings.Contains(filter, "%") {
		return value == filter
	}

	parts := strings.Split(filter, "%")
	offset := 0
	for index, part := range parts {
		if part == "" {
			continue
		}
		found := strings.Index(value[offset:], part)
		if found < 0 {
			return false
		}
		if index == 0 && !strings.HasPrefix(filter, "%") && found != 0 {
			return false
		}
		offset += found + len(part)
	}
	return strings.HasSuffix(filter, "%") || parts[len(parts)-1] == "" || strings.HasSuffix(value, parts[len(parts)-1])
}

func taskIDFromBillingOther(raw string) string {
	other, _ := common.StrToMap(raw)
	if other == nil {
		return ""
	}
	taskID, _ := other["task_id"].(string)
	return strings.TrimSpace(taskID)
}

func settledTaskRefunds(userID int, tasks []*Task, startTimestamp int64) (map[string]int, error) {
	taskIDs := make(map[string]struct{}, len(tasks))
	for _, task := range tasks {
		if task != nil && task.Status == TaskStatusFailure && task.TaskID != "" {
			taskIDs[task.TaskID] = struct{}{}
		}
	}
	refunds := make(map[string]int, len(taskIDs))
	if len(taskIDs) == 0 {
		return refunds, nil
	}

	query := LOG_DB.Where("user_id = ? AND type = ?", userID, LogTypeRefund).
		Where("other LIKE ?", `%"task_id":%`)
	if startTimestamp != 0 {
		query = query.Where("created_at >= ?", startTimestamp)
	}
	var logs []*Log
	order := "id desc"
	if common.UsingLogDatabase(common.DatabaseTypeClickHouse) {
		order = clickHouseLogOrder("")
	}
	if err := query.Order(order).Limit(settledUsageSearchLimit + 1).Find(&logs).Error; err != nil {
		return nil, err
	}
	if len(logs) > settledUsageSearchLimit {
		return nil, errors.New("usage range contains too many refund records")
	}
	for _, log := range logs {
		taskID := taskIDFromBillingOther(log.Other)
		if _, ok := taskIDs[taskID]; ok && log.Quota > 0 {
			refunds[taskID] += log.Quota
		}
	}
	return refunds, nil
}

func settledTaskTokenNames(tasks []*Task) (map[int]string, error) {
	tokenIDs := make([]int, 0, len(tasks))
	seen := make(map[int]struct{}, len(tasks))
	for _, task := range tasks {
		tokenID := task.PrivateData.TokenId
		if tokenID <= 0 {
			continue
		}
		if _, ok := seen[tokenID]; ok {
			continue
		}
		seen[tokenID] = struct{}{}
		tokenIDs = append(tokenIDs, tokenID)
	}
	names := make(map[int]string, len(tokenIDs))
	if len(tokenIDs) == 0 {
		return names, nil
	}
	var tokens []Token
	if err := DB.Select("id", "name").Where("id IN ?", tokenIDs).Find(&tokens).Error; err != nil {
		return nil, err
	}
	for _, token := range tokens {
		names[token.Id] = token.Name
	}
	return names, nil
}

func settledTaskLogs(
	userID int,
	startTimestamp int64,
	endTimestamp int64,
	modelName string,
	tokenName string,
	group string,
	requestID string,
	upstreamRequestID string,
) ([]*Log, error) {
	if upstreamRequestID != "" {
		return []*Log{}, nil
	}

	query := DB.Where("user_id = ?", userID)
	if startTimestamp != 0 {
		query = query.Where("submit_time >= ?", startTimestamp)
	}
	if endTimestamp != 0 {
		query = query.Where("submit_time <= ?", endTimestamp)
	}
	if group != "" {
		query = query.Where(commonGroupCol+" = ?", group)
	}
	if requestID != "" {
		query = query.Where("task_id = ?", requestID)
	}
	var tasks []*Task
	if err := query.Order("id desc").Limit(settledUsageSearchLimit + 1).Find(&tasks).Error; err != nil {
		return nil, err
	}
	if len(tasks) > settledUsageSearchLimit {
		return nil, errors.New("usage range contains too many task records")
	}

	refunds, err := settledTaskRefunds(userID, tasks, startTimestamp)
	if err != nil {
		return nil, err
	}
	tokenNames, err := settledTaskTokenNames(tasks)
	if err != nil {
		return nil, err
	}

	logs := make([]*Log, 0, len(tasks))
	for _, task := range tasks {
		if task == nil || task.Quota <= 0 {
			continue
		}
		model := strings.TrimSpace(task.Properties.OriginModelName)
		if model == "" {
			model = strings.TrimSpace(task.Properties.UpstreamModelName)
		}
		if !usageTextMatches(model, modelName) {
			continue
		}
		currentTokenName := tokenNames[task.PrivateData.TokenId]
		if tokenName != "" && currentTokenName != tokenName {
			continue
		}

		netQuota := 0
		switch task.Status {
		case TaskStatusSuccess:
			netQuota = task.Quota
		case TaskStatusFailure:
			netQuota = task.Quota - refunds[task.TaskID]
		default:
			continue
		}
		if netQuota <= 0 {
			continue
		}

		createdAt := task.SubmitTime
		if createdAt == 0 {
			createdAt = task.CreatedAt
		}
		useTime := 0
		if task.StartTime > 0 && task.FinishTime >= task.StartTime {
			useTime = int(task.FinishTime - task.StartTime)
		}
		logs = append(logs, &Log{
			Id:        int(task.ID),
			UserId:    userID,
			CreatedAt: createdAt,
			Type:      LogTypeConsume,
			TokenName: currentTokenName,
			ModelName: model,
			Quota:     netQuota,
			UseTime:   useTime,
			ChannelId: task.ChannelId,
			TokenId:   task.PrivateData.TokenId,
			Group:     task.Group,
			RequestId: task.TaskID,
		})
	}
	return logs, nil
}

// GetUserSettledUsageLogs returns final customer spend. Task pre-consume rows and
// task adjustment/refund rows are replaced with one final task row, while normal
// synchronous consume logs retain their existing ledger semantics.
func GetUserSettledUsageLogs(
	userID int,
	startTimestamp int64,
	endTimestamp int64,
	modelName string,
	tokenName string,
	startIdx int,
	num int,
	group string,
	requestID string,
	upstreamRequestID string,
) (logs []*Log, total int64, err error) {
	query := LOG_DB.Where("logs.user_id = ? AND logs.type = ?", userID, LogTypeConsume)
	query = nonTaskUsageScope(query)
	if query, err = applyExplicitLogTextFilter(query, "logs.model_name", modelName); err != nil {
		return nil, 0, err
	}
	if tokenName != "" {
		query = query.Where("logs.token_name = ?", tokenName)
	}
	if requestID != "" {
		query = query.Where("logs.request_id = ?", requestID)
	}
	if upstreamRequestID != "" {
		query = query.Where("logs.upstream_request_id = ?", upstreamRequestID)
	}
	if startTimestamp != 0 {
		query = query.Where("logs.created_at >= ?", startTimestamp)
	}
	if endTimestamp != 0 {
		query = query.Where("logs.created_at <= ?", endTimestamp)
	}
	if group != "" {
		query = query.Where("logs."+logGroupCol+" = ?", group)
	}

	var synchronous []*Log
	order := "logs.id desc"
	if common.UsingLogDatabase(common.DatabaseTypeClickHouse) {
		order = clickHouseLogOrder("logs.")
	}
	if err = query.Order(order).Limit(settledUsageSearchLimit + 1).Find(&synchronous).Error; err != nil {
		return nil, 0, err
	}
	if len(synchronous) > settledUsageSearchLimit {
		return nil, 0, errors.New("usage range contains too many synchronous records")
	}

	tasks, err := settledTaskLogs(
		userID,
		startTimestamp,
		endTimestamp,
		modelName,
		tokenName,
		group,
		requestID,
		upstreamRequestID,
	)
	if err != nil {
		return nil, 0, err
	}
	logs = append(synchronous, tasks...)
	sort.SliceStable(logs, func(i int, j int) bool {
		if logs[i].CreatedAt == logs[j].CreatedAt {
			return logs[i].Id > logs[j].Id
		}
		return logs[i].CreatedAt > logs[j].CreatedAt
	})

	total = int64(len(logs))
	if startIdx < 0 {
		startIdx = 0
	}
	if startIdx >= len(logs) || num <= 0 {
		return []*Log{}, total, nil
	}
	end := startIdx + num
	if end > len(logs) {
		end = len(logs)
	}
	logs = logs[startIdx:end]
	formatUserLogs(logs, startIdx)
	return logs, total, nil
}

func SumUserSettledUsage(
	userID int,
	startTimestamp int64,
	endTimestamp int64,
	modelName string,
	tokenName string,
	group string,
) (Stat, error) {
	logs, total, err := GetUserSettledUsageLogs(
		userID,
		startTimestamp,
		endTimestamp,
		modelName,
		tokenName,
		0,
		settledUsageSearchLimit,
		group,
		"",
		"",
	)
	if err != nil {
		return Stat{}, err
	}
	if total > settledUsageSearchLimit {
		return Stat{}, errors.New("usage range contains too many settled records")
	}
	stat := Stat{}
	maxInt := int(^uint(0) >> 1)
	for _, log := range logs {
		if log.Quota > 0 && stat.Quota > maxInt-log.Quota {
			return Stat{}, errors.New("settled usage quota overflow")
		}
		stat.Quota += log.Quota
		stat.Rpm++
		stat.Tpm += log.PromptTokens + log.CompletionTokens
	}
	return stat, nil
}
