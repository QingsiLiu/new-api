package controller

import (
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
)

type asyncChannelHealthKey struct {
	ChannelID int
	Model     string
	Kind      string
	Action    string
}

type asyncChannelHealthItem struct {
	ChannelID       int                        `json:"channel_id"`
	ChannelName     string                     `json:"channel_name"`
	Model           string                     `json:"model"`
	Kind            string                     `json:"kind"`
	Action          string                     `json:"action"`
	Attempts        int                        `json:"attempts"`
	ScoredAttempts  int                        `json:"scored_attempts"`
	Successes       int                        `json:"successes"`
	Failures        int                        `json:"failures"`
	SuccessRate     float64                    `json:"success_rate"`
	P95LatencyMS    int64                      `json:"p95_latency_ms"`
	LastFailure     string                     `json:"last_failure,omitempty"`
	LastFailureAt   int64                      `json:"last_failure_at,omitempty"`
	LastHTTPStatus  int                        `json:"last_http_status,omitempty"`
	Circuit         service.AsyncCircuitStatus `json:"circuit"`
	durations       []int64
	latestFailureID int64
}

type asyncChannelHealthSummary struct {
	Tasks             int     `json:"tasks"`
	SuccessfulTasks   int     `json:"successful_tasks"`
	FailedTasks       int     `json:"failed_tasks"`
	RecoveredTasks    int     `json:"recovered_tasks"`
	TaskSuccessRate   float64 `json:"task_success_rate"`
	FailoverRecovery  float64 `json:"failover_recovery_rate"`
	AverageAttempts   float64 `json:"average_attempts"`
	ProviderAttempts  int     `json:"provider_attempts"`
	ScoredAttempts    int     `json:"scored_attempts"`
	SuccessfulAttempt int     `json:"successful_attempts"`
}

func GetAsyncChannelHealth(c *gin.Context) {
	hours := parseAsyncHealthHours(c.Query("hours"))
	channelID, _ := strconv.Atoi(c.Query("channel_id"))
	modelName := strings.TrimSpace(c.Query("model"))
	kind := strings.TrimSpace(c.Query("kind"))
	action := strings.TrimSpace(c.Query("action"))
	now := time.Now().Unix()
	attempts, err := model.QueryAsyncTaskAttempts(model.AsyncTaskAttemptQuery{
		StartedAfter:  now - int64(hours)*3600,
		StartedBefore: now,
		ChannelID:     channelID,
		Model:         modelName,
		Kind:          kind,
		Action:        action,
	})
	if err != nil {
		common.ApiError(c, err)
		return
	}
	items, summary := aggregateAsyncChannelHealth(attempts)
	if err := hydrateAsyncTaskOutcomeSummary(attempts, &summary); err != nil {
		common.ApiError(c, err)
		return
	}
	coverage, err := model.GetAsyncMediaChannelCoverage()
	if err != nil {
		common.ApiError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"hours":        hours,
			"generated_at": now,
			"summary":      summary,
			"items":        items,
			"coverage":     coverage,
		},
	})
}

func GetAdminAsyncTaskAttempts(c *gin.Context) {
	taskID := strings.TrimSpace(c.Param("task_id"))
	if taskID == "" {
		common.ApiErrorMsg(c, "task_id is required")
		return
	}
	attempts, err := model.GetAsyncTaskAttempts(taskID)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, attempts)
}

func ResetAsyncChannelCircuit(c *gin.Context) {
	channelID, err := strconv.Atoi(c.Param("id"))
	if err != nil || channelID <= 0 {
		common.ApiErrorMsg(c, "invalid channel id")
		return
	}
	deleted, err := service.ResetAsyncCircuitByChannel(channelID)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, gin.H{"channel_id": channelID, "deleted_keys": deleted})
}

func parseAsyncHealthHours(raw string) int {
	hours, _ := strconv.Atoi(strings.TrimSpace(raw))
	switch hours {
	case 1, 24, 24 * 7, 24 * 30:
		return hours
	default:
		return 24
	}
}

func aggregateAsyncChannelHealth(attempts []model.AsyncTaskAttempt) ([]asyncChannelHealthItem, asyncChannelHealthSummary) {
	itemsByKey := make(map[asyncChannelHealthKey]*asyncChannelHealthItem)
	taskAttempts := make(map[string]int)
	for _, attempt := range attempts {
		key := asyncChannelHealthKey{ChannelID: attempt.ChannelID, Model: attempt.Model, Kind: attempt.Kind, Action: attempt.Action}
		item := itemsByKey[key]
		if item == nil {
			item = &asyncChannelHealthItem{ChannelID: attempt.ChannelID, Model: attempt.Model, Kind: attempt.Kind, Action: attempt.Action}
			if channel, err := model.CacheGetChannel(attempt.ChannelID); err == nil && channel != nil {
				item.ChannelName = channel.Name
			}
			itemsByKey[key] = item
		}
		item.Attempts++
		taskAttempts[attempt.TaskID]++
		if attempt.Status == model.AsyncTaskAttemptStatusSucceeded {
			item.Successes++
			item.ScoredAttempts++
		} else if attempt.Retryable {
			item.Failures++
			item.ScoredAttempts++
		}
		if attempt.DurationMS > 0 {
			item.durations = append(item.durations, attempt.DurationMS)
		}
		if attempt.Status == model.AsyncTaskAttemptStatusFailed && attempt.ID >= item.latestFailureID {
			item.latestFailureID = attempt.ID
			item.LastFailure = attempt.FailureClass
			item.LastFailureAt = attempt.CompletedAt
			item.LastHTTPStatus = attempt.HTTPStatus
		}
	}

	items := make([]asyncChannelHealthItem, 0, len(itemsByKey))
	summary := asyncChannelHealthSummary{ProviderAttempts: len(attempts)}
	for key, item := range itemsByKey {
		if item.ScoredAttempts > 0 {
			item.SuccessRate = roundAsyncPercent(float64(item.Successes) / float64(item.ScoredAttempts) * 100)
		}
		item.P95LatencyMS = asyncP95(item.durations)
		item.Circuit = service.GetAsyncCircuitStatus(service.AsyncCircuitKey{ChannelID: key.ChannelID, Model: key.Model, Kind: key.Kind, Action: key.Action})
		item.durations = nil
		item.latestFailureID = 0
		summary.ScoredAttempts += item.ScoredAttempts
		summary.SuccessfulAttempt += item.Successes
		items = append(items, *item)
	}
	summary.Tasks = len(taskAttempts)
	if summary.Tasks > 0 {
		summary.AverageAttempts = float64(summary.ProviderAttempts) / float64(summary.Tasks)
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].Attempts == items[j].Attempts {
			if items[i].ChannelID == items[j].ChannelID {
				return items[i].Model < items[j].Model
			}
			return items[i].ChannelID < items[j].ChannelID
		}
		return items[i].Attempts > items[j].Attempts
	})
	return items, summary
}

func hydrateAsyncTaskOutcomeSummary(attempts []model.AsyncTaskAttempt, summary *asyncChannelHealthSummary) error {
	if summary == nil {
		return nil
	}
	attemptCountByTask := make(map[string]int)
	for _, attempt := range attempts {
		attemptCountByTask[attempt.TaskID]++
	}
	taskIDs := make([]string, 0, len(attemptCountByTask))
	for taskID := range attemptCountByTask {
		taskIDs = append(taskIDs, taskID)
	}
	facts, err := model.GetTaskStatusFacts(taskIDs)
	if err != nil {
		return err
	}
	failoverTriggered := 0
	for _, count := range attemptCountByTask {
		if count > 1 {
			failoverTriggered++
		}
	}
	for _, fact := range facts {
		switch fact.Status {
		case model.TaskStatusSuccess:
			summary.SuccessfulTasks++
			if attemptCountByTask[fact.TaskID] > 1 {
				summary.RecoveredTasks++
			}
		case model.TaskStatusFailure:
			summary.FailedTasks++
		}
	}
	terminalTasks := summary.SuccessfulTasks + summary.FailedTasks
	if terminalTasks > 0 {
		summary.TaskSuccessRate = roundAsyncPercent(float64(summary.SuccessfulTasks) / float64(terminalTasks) * 100)
	}
	if failoverTriggered > 0 {
		summary.FailoverRecovery = roundAsyncPercent(float64(summary.RecoveredTasks) / float64(failoverTriggered) * 100)
	}
	return nil
}

func asyncP95(values []int64) int64 {
	if len(values) == 0 {
		return 0
	}
	sort.Slice(values, func(i, j int) bool { return values[i] < values[j] })
	index := (95*len(values) + 99) / 100
	if index <= 0 {
		index = 1
	}
	return values[index-1]
}

func roundAsyncPercent(value float64) float64 {
	return float64(int64(value*100+0.5)) / 100
}
