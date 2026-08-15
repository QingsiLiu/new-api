package service

import (
	"context"
	"errors"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
)

const funnelMaintenanceFreshSeconds int64 = 36 * 3600

var ErrGeiliFunnelHealthUnavailable = errors.New("geili funnel health unavailable")

func GetGeiliFunnelHealth(ctx context.Context, now int64, environment string) (dto.GeiliFunnelHealthResponse, error) {
	response := dto.GeiliFunnelHealthResponse{
		Environment: environment,
		Events:      make([]dto.FunnelHealthEvent, 0, 5),
	}
	config, err := LoadGeiliFunnelConfig()
	if err != nil || !config.Enabled || now <= 0 || (environment != model.FunnelEnvironmentProduction && environment != model.FunnelEnvironmentStaging) {
		return response, ErrGeiliFunnelHealthUnavailable
	}
	response.Enabled = true
	if !model.DB.Migrator().HasTable(&model.FunnelVisitor{}) ||
		!model.DB.Migrator().HasTable(&model.FunnelEvent{}) ||
		!model.DB.Migrator().HasTable(&model.FunnelActivityDay{}) {
		return response, ErrGeiliFunnelHealthUnavailable
	}
	response.SchemaVersion = 2

	response.CollectionStartedAt, err = model.LoadFunnelCollectionStart(ctx, environment)
	if err != nil {
		return response, ErrGeiliFunnelHealthUnavailable
	}
	response.LastEventAt, err = model.LoadFunnelLastEventAt(ctx, environment)
	if err != nil {
		return response, ErrGeiliFunnelHealthUnavailable
	}
	eventCounts, err := model.LoadFunnelEventCounts(ctx, environment, now-86400, now)
	if err != nil {
		return response, ErrGeiliFunnelHealthUnavailable
	}
	for _, count := range eventCounts {
		response.Events = append(response.Events, dto.FunnelHealthEvent{Name: count.EventName, Last24h: count.Count})
	}

	counters := GetFunnelIngestCounters()
	response.Counters = dto.FunnelHealthCounters{
		Accepted: counters.Accepted, Duplicate: counters.Duplicate, Rejected: counters.Rejected,
		Failed: counters.Failed, Since: counters.Since,
	}
	identityCounts, err := model.LoadFunnelIdentityCounts(ctx, environment, now-funnelRawRetentionSeconds)
	if err != nil {
		return response, ErrGeiliFunnelHealthUnavailable
	}
	for _, count := range identityCounts {
		switch count.IdentityState {
		case model.FunnelIdentityLinked:
			response.Identity.Linked = count.Count
		case model.FunnelIdentityAmbiguous:
			response.Identity.Ambiguous = count.Count
		}
	}
	response.Identity.AmbiguousRate = ratioPointer(
		response.Identity.Ambiguous,
		response.Identity.Linked+response.Identity.Ambiguous,
	)

	response.Business.InvalidTopUpTimes, response.Business.InvalidTaskTimes, err = model.LoadInvalidFunnelBusinessTimes(ctx)
	if err != nil {
		return response, ErrGeiliFunnelHealthUnavailable
	}
	apiKeys, invalidTokenTimes, err := model.LoadIndependentFirstAPIKeys(ctx, now-86400, now)
	if err != nil {
		return response, ErrGeiliFunnelHealthUnavailable
	}
	textSuccesses, invalidTextLogTimes, err := model.LoadIndependentFirstSuccessfulTextCalls(ctx, now-86400, now)
	if err != nil {
		return response, ErrGeiliFunnelHealthUnavailable
	}
	activations, err := model.LoadIndependentFirstActivations(ctx, now-86400, now)
	if err != nil {
		return response, ErrGeiliFunnelHealthUnavailable
	}
	response.Business.InvalidTokenTimes = invalidTokenTimes
	response.Business.InvalidTextLogTimes = invalidTextLogTimes
	response.Business.FirstAPIKeysLast24h = int64(len(apiKeys))
	response.Business.FirstActivatedLast24h = int64(len(activations))
	response.Business.FirstSuccessfulTextLast24h = int64(len(textSuccesses))
	taskStatuses, err := model.LoadTaskStatusFacts(ctx, now-86400, now)
	if err != nil {
		return response, ErrGeiliFunnelHealthUnavailable
	}
	for _, count := range taskStatuses {
		switch count.Status {
		case model.TaskStatusSuccess:
			response.Business.TaskSuccessLast24h = count.Count
		case model.TaskStatusFailure:
			response.Business.TaskFailureLast24h = count.Count
		}
	}

	latest, err := model.GetLatestSystemTask(model.SystemTaskTypeFunnelMaintenance)
	if err != nil {
		return response, ErrGeiliFunnelHealthUnavailable
	}
	latestSuccessful, err := model.GetLatestSuccessfulSystemTask(model.SystemTaskTypeFunnelMaintenance)
	if err != nil {
		return response, ErrGeiliFunnelHealthUnavailable
	}
	applyFunnelMaintenanceHealth(&response, now, latest, latestSuccessful)
	return response, nil
}

func applyFunnelMaintenanceHealth(response *dto.GeiliFunnelHealthResponse, now int64, latest, latestSuccessful *model.SystemTask) {
	response.Healthy = true
	response.Maintenance.Status = "pending_initial_run"
	if latest != nil {
		response.Maintenance.LastRunAt = latest.UpdatedAt
	}
	if response.CollectionStartedAt == 0 {
		return
	}
	if latestSuccessful == nil {
		if now-response.CollectionStartedAt <= funnelMaintenanceFreshSeconds {
			return
		}
		response.Healthy = false
		response.Maintenance.Status = latestFunnelMaintenanceStatus(latest)
		return
	}

	response.Maintenance.LastSuccessfulAt = latestSuccessful.UpdatedAt
	var result model.FunnelMaintenanceResult
	if err := common.UnmarshalJsonStr(latestSuccessful.Result, &result); err != nil || !validFunnelMaintenanceCutoffs(result, latestSuccessful.UpdatedAt) {
		response.Healthy = false
		response.Maintenance.Status = "stale"
		return
	}
	response.Maintenance.RawCutoff = result.RawCutoff
	response.Maintenance.ActivityCutoff = result.ActivityCutoff
	response.Maintenance.Status = "succeeded"
	if now-latestSuccessful.UpdatedAt > funnelMaintenanceFreshSeconds {
		response.Healthy = false
		response.Maintenance.Status = "stale"
	}
	if latest != nil && latest.ID > latestSuccessful.ID {
		switch latest.Status {
		case model.SystemTaskStatusFailed:
			response.Healthy = false
			response.Maintenance.Status = "failed"
		case model.SystemTaskStatusPending, model.SystemTaskStatusRunning:
			response.Maintenance.Status = "pending"
		}
	}
}

func latestFunnelMaintenanceStatus(latest *model.SystemTask) string {
	if latest == nil {
		return "stale"
	}
	switch latest.Status {
	case model.SystemTaskStatusFailed:
		return "failed"
	case model.SystemTaskStatusPending, model.SystemTaskStatusRunning:
		return "pending"
	default:
		return "stale"
	}
}

func validFunnelMaintenanceCutoffs(result model.FunnelMaintenanceResult, finishedAt int64) bool {
	if result.RawCutoff <= 0 || result.ActivityCutoff <= 0 {
		return false
	}
	maintenanceAt := result.RawCutoff + funnelRawRetentionSeconds
	expectedActivityCutoff := utcDay(maintenanceAt) - 730*86400
	completionDelay := finishedAt - maintenanceAt
	return result.ActivityCutoff == expectedActivityCutoff && completionDelay >= 0 && completionDelay <= 6*3600
}
