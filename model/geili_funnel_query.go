package model

import (
	"context"
	"errors"
	"sort"
	"strconv"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
)

type FunnelVisitorFact struct {
	VisitorID     int64
	UserID        *int
	IdentityState string
	FirstSLPAt    int64
	Locale        string
	ModelSlug     string
}

type FunnelTimedUser struct {
	UserID int
	At     int64
}

type FunnelActivityFact struct {
	UserID       int
	ActivityDate int64
}

type FunnelFailureFact struct {
	FailureCode string
	ModelSlug   string
	Count       int64
}

type FunnelTaskStatusFact struct {
	Status TaskStatus
	Count  int64
}

type FunnelEventCountFact struct {
	EventName string
	Count     int64
}

type FunnelIdentityCountFact struct {
	IdentityState string
	Count         int64
}

func LoadEntryVisitors(ctx context.Context, environment string, from, to int64) ([]FunnelVisitorFact, error) {
	rows := make([]FunnelVisitorFact, 0)
	err := DB.WithContext(ctx).Model(&FunnelVisitor{}).
		Select("id AS visitor_id, user_id, identity_state, first_slp_at, first_slp_locale AS locale, first_slp_model AS model_slug").
		Where("environment = ? AND first_slp_at >= ? AND first_slp_at < ?", environment, from, to).
		Order("first_slp_at ASC, id ASC").Scan(&rows).Error
	return rows, err
}

func LoadLinkedFirstTouches(ctx context.Context, environment string, userIDs []int) (map[int]FunnelVisitorFact, error) {
	result := map[int]FunnelVisitorFact{}
	userIDs = positiveUniqueUserIDs(userIDs)
	if len(userIDs) == 0 {
		return result, nil
	}
	rows := make([]FunnelVisitorFact, 0)
	err := DB.WithContext(ctx).Model(&FunnelVisitor{}).
		Select("id AS visitor_id, user_id, identity_state, first_slp_at, first_slp_locale AS locale, first_slp_model AS model_slug").
		Where("environment = ? AND identity_state = ? AND user_id IS NOT NULL AND user_id IN ? AND first_slp_at > 0", environment, FunnelIdentityLinked, userIDs).
		Order("first_slp_at ASC, id ASC").Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		if row.UserID == nil {
			continue
		}
		if _, exists := result[*row.UserID]; !exists {
			result[*row.UserID] = row
		}
	}
	return result, nil
}

func LoadUserCreatedTimes(ctx context.Context, userIDs []int) (map[int]int64, error) {
	result := map[int]int64{}
	userIDs = positiveUniqueUserIDs(userIDs)
	if len(userIDs) == 0 {
		return result, nil
	}
	var rows []struct {
		ID        int
		CreatedAt int64
	}
	if err := DB.WithContext(ctx).Model(&User{}).Select("id, created_at").Where("id IN ?", userIDs).Scan(&rows).Error; err != nil {
		return nil, err
	}
	for _, row := range rows {
		result[row.ID] = row.CreatedAt
	}
	return result, nil
}

func LoadFirstSuccessfulTopUps(ctx context.Context, userIDs []int) (map[int]int64, int64, error) {
	result := map[int]int64{}
	userIDs = positiveUniqueUserIDs(userIDs)
	if len(userIDs) == 0 {
		return result, 0, nil
	}
	var rows []FunnelTimedUser
	err := DB.WithContext(ctx).Model(&TopUp{}).
		Select("user_id, MIN(complete_time) AS at").
		Where("user_id IN ? AND status = ? AND complete_time > 0", userIDs, common.TopUpStatusSuccess).
		Group("user_id").Scan(&rows).Error
	if err != nil {
		return nil, 0, err
	}
	for _, row := range rows {
		result[row.UserID] = row.At
	}
	var invalid int64
	err = DB.WithContext(ctx).Model(&TopUp{}).
		Where("user_id IN ? AND status = ? AND complete_time <= 0", userIDs, common.TopUpStatusSuccess).
		Count(&invalid).Error
	return result, invalid, err
}

func LoadFirstSuccessfulTasks(ctx context.Context, userIDs []int) (map[int]int64, int64, error) {
	result := map[int]int64{}
	userIDs = positiveUniqueUserIDs(userIDs)
	if len(userIDs) == 0 {
		return result, 0, nil
	}
	var rows []FunnelTimedUser
	err := DB.WithContext(ctx).Model(&Task{}).
		Select("user_id, MIN(finish_time) AS at").
		Where("user_id IN ? AND status = ? AND finish_time > 0", userIDs, TaskStatusSuccess).
		Group("user_id").Scan(&rows).Error
	if err != nil {
		return nil, 0, err
	}
	for _, row := range rows {
		result[row.UserID] = row.At
	}
	var invalid int64
	err = DB.WithContext(ctx).Model(&Task{}).
		Where("user_id IN ? AND status = ? AND finish_time <= 0", userIDs, TaskStatusSuccess).
		Count(&invalid).Error
	return result, invalid, err
}

func LoadStudioOpenTimes(ctx context.Context, environment string, userIDs []int) (map[int][]int64, error) {
	result := map[int][]int64{}
	userIDs = positiveUniqueUserIDs(userIDs)
	if len(userIDs) == 0 {
		return result, nil
	}
	var rows []FunnelTimedUser
	err := DB.WithContext(ctx).Table("funnel_events AS events").
		Select("visitors.user_id AS user_id, events.received_at AS at").
		Joins("JOIN funnel_visitors AS visitors ON visitors.id = events.visitor_id").
		Where("events.environment = ? AND events.event_name = ? AND visitors.environment = ? AND visitors.identity_state = ? AND visitors.user_id IS NOT NULL AND visitors.user_id IN ?", environment, FunnelEventOpenStudio, environment, FunnelIdentityLinked, userIDs).
		Order("visitors.user_id ASC, events.received_at ASC, events.id ASC").Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		result[row.UserID] = append(result[row.UserID], row.At)
	}
	return result, nil
}

func LoadIndependentRegistrations(ctx context.Context, from, to int64) ([]FunnelTimedUser, error) {
	rows := make([]FunnelTimedUser, 0)
	err := DB.WithContext(ctx).Model(&User{}).Select("id AS user_id, created_at AS at").
		Where("created_at >= ? AND created_at < ?", from, to).Order("created_at ASC, id ASC").Scan(&rows).Error
	return rows, err
}

func LoadIndependentFirstAPIKeys(ctx context.Context, from, to int64) ([]FunnelTimedUser, int64, error) {
	first := DB.WithContext(ctx).Unscoped().Model(&Token{}).
		Select("user_id, MIN(created_time) AS at").
		Where("user_id > 0 AND created_time > 0 AND name <> ?", GeiliStudioOnlineTokenName).
		Group("user_id")
	rows := make([]FunnelTimedUser, 0)
	err := DB.WithContext(ctx).Table("(?) AS first_api_keys", first).
		Where("at >= ? AND at < ?", from, to).Order("at ASC, user_id ASC").Scan(&rows).Error
	if err != nil {
		return nil, 0, err
	}
	var invalid int64
	err = DB.WithContext(ctx).Unscoped().Model(&Token{}).
		Where("user_id > 0 AND created_time <= 0 AND name <> ?", GeiliStudioOnlineTokenName).
		Count(&invalid).Error
	return rows, invalid, err
}

func LoadIndependentFirstSuccessfulTextCalls(ctx context.Context, from, to int64) ([]FunnelTimedUser, int64, error) {
	modelNames := make([]string, 0)
	if err := DB.WithContext(ctx).Model(&ModelRegistry{}).
		Where("modality = ?", "text").
		Distinct().
		Order("model_name ASC").
		Pluck("model_name", &modelNames).Error; err != nil {
		return nil, 0, err
	}
	if len(modelNames) == 0 {
		return []FunnelTimedUser{}, 0, nil
	}
	first := LOG_DB.WithContext(ctx).Model(&Log{}).
		Select("user_id, MIN(created_at) AS at").
		Where("user_id > 0 AND type = ? AND created_at > 0 AND model_name IN ? AND (prompt_tokens > 0 OR completion_tokens > 0)", LogTypeConsume, modelNames).
		Group("user_id")
	rows := make([]FunnelTimedUser, 0)
	err := LOG_DB.WithContext(ctx).Table("(?) AS first_text_calls", first).
		Where("at >= ? AND at < ?", from, to).Order("at ASC, user_id ASC").Scan(&rows).Error
	if err != nil {
		return nil, 0, err
	}
	var invalid int64
	err = LOG_DB.WithContext(ctx).Model(&Log{}).
		Where("user_id > 0 AND type = ? AND created_at <= 0 AND model_name IN ? AND (prompt_tokens > 0 OR completion_tokens > 0)", LogTypeConsume, modelNames).
		Count(&invalid).Error
	return rows, invalid, err
}

func LoadIndependentFirstActivations(ctx context.Context, from, to int64) ([]FunnelTimedUser, error) {
	textRows, _, err := LoadIndependentFirstSuccessfulTextCalls(ctx, 1, to)
	if err != nil {
		return nil, err
	}
	taskRows, _, err := LoadIndependentFirstTasks(ctx, 1, to)
	if err != nil {
		return nil, err
	}
	firstByUser := make(map[int]int64, len(textRows)+len(taskRows))
	for _, row := range append(textRows, taskRows...) {
		if existing, ok := firstByUser[row.UserID]; !ok || row.At < existing {
			firstByUser[row.UserID] = row.At
		}
	}
	rows := make([]FunnelTimedUser, 0, len(firstByUser))
	for userID, at := range firstByUser {
		if at >= from && at < to {
			rows = append(rows, FunnelTimedUser{UserID: userID, At: at})
		}
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].At == rows[j].At {
			return rows[i].UserID < rows[j].UserID
		}
		return rows[i].At < rows[j].At
	})
	return rows, nil
}

func LoadIndependentFirstTopUps(ctx context.Context, from, to int64) ([]FunnelTimedUser, int64, error) {
	first := DB.WithContext(ctx).Model(&TopUp{}).
		Select("user_id, MIN(complete_time) AS at").
		Where("status = ? AND complete_time > 0", common.TopUpStatusSuccess).Group("user_id")
	rows := make([]FunnelTimedUser, 0)
	err := DB.WithContext(ctx).Table("(?) AS first_topups", first).
		Where("at >= ? AND at < ?", from, to).Order("at ASC, user_id ASC").Scan(&rows).Error
	if err != nil {
		return nil, 0, err
	}
	var invalid int64
	err = DB.WithContext(ctx).Model(&TopUp{}).Where("status = ? AND complete_time <= 0", common.TopUpStatusSuccess).Count(&invalid).Error
	return rows, invalid, err
}

func LoadIndependentFirstTasks(ctx context.Context, from, to int64) ([]FunnelTimedUser, int64, error) {
	first := DB.WithContext(ctx).Model(&Task{}).
		Select("user_id, MIN(finish_time) AS at").
		Where("status = ? AND finish_time > 0", TaskStatusSuccess).Group("user_id")
	rows := make([]FunnelTimedUser, 0)
	err := DB.WithContext(ctx).Table("(?) AS first_tasks", first).
		Where("at >= ? AND at < ?", from, to).Order("at ASC, user_id ASC").Scan(&rows).Error
	if err != nil {
		return nil, 0, err
	}
	var invalid int64
	err = DB.WithContext(ctx).Model(&Task{}).Where("status = ? AND finish_time <= 0", TaskStatusSuccess).Count(&invalid).Error
	return rows, invalid, err
}

func LoadIndependentFirstStudio(ctx context.Context, environment string, from, to int64) ([]FunnelTimedUser, error) {
	first := DB.WithContext(ctx).Table("funnel_events AS events").
		Select("visitors.user_id AS user_id, MIN(events.received_at) AS at").
		Joins("JOIN funnel_visitors AS visitors ON visitors.id = events.visitor_id").
		Where("events.environment = ? AND events.event_name = ? AND visitors.environment = ? AND visitors.identity_state = ? AND visitors.user_id IS NOT NULL", environment, FunnelEventOpenStudio, environment, FunnelIdentityLinked).
		Group("visitors.user_id")
	rows := make([]FunnelTimedUser, 0)
	err := DB.WithContext(ctx).Table("(?) AS first_studio", first).
		Where("at >= ? AND at < ?", from, to).Order("at ASC, user_id ASC").Scan(&rows).Error
	return rows, err
}

func LoadFunnelActivityDays(ctx context.Context, environment string, userIDs []int, fromDay, toDay int64) ([]FunnelActivityFact, error) {
	rows := make([]FunnelActivityFact, 0)
	userIDs = positiveUniqueUserIDs(userIDs)
	if len(userIDs) == 0 {
		return rows, nil
	}
	err := DB.WithContext(ctx).Model(&FunnelActivityDay{}).Select("user_id, activity_date").
		Where("environment = ? AND user_id IN ? AND activity_date >= ? AND activity_date < ?", environment, userIDs, fromDay, toDay).
		Order("user_id ASC, activity_date ASC").Scan(&rows).Error
	return rows, err
}

func LoadFunnelFailures(ctx context.Context, environment string, from, to int64) ([]FunnelFailureFact, error) {
	rows := make([]FunnelFailureFact, 0)
	err := DB.WithContext(ctx).Model(&FunnelEvent{}).
		Select("failure_code, model_slug, COUNT(*) AS count").
		Where("environment = ? AND event_name = ? AND received_at >= ? AND received_at < ?", environment, FunnelEventPlaygroundFail, from, to).
		Group("failure_code, model_slug").Order("failure_code ASC, model_slug ASC").Scan(&rows).Error
	return rows, err
}

func LoadTaskStatusFacts(ctx context.Context, from, to int64) ([]FunnelTaskStatusFact, error) {
	rows := make([]FunnelTaskStatusFact, 0)
	err := DB.WithContext(ctx).Model(&Task{}).Select("status, COUNT(*) AS count").
		Where("status IN ? AND finish_time >= ? AND finish_time < ?", []TaskStatus{TaskStatusSuccess, TaskStatusFailure}, from, to).
		Group("status").Order("status ASC").Scan(&rows).Error
	return rows, err
}

func LoadFunnelEventCounts(ctx context.Context, environment string, from, to int64) ([]FunnelEventCountFact, error) {
	names := []string{FunnelEventSLPView, FunnelEventIdentityLink, FunnelEventAccountActive, FunnelEventOpenStudio, FunnelEventPlaygroundFail}
	var rows []FunnelEventCountFact
	err := DB.WithContext(ctx).Model(&FunnelEvent{}).Select("event_name, COUNT(*) AS count").
		Where("environment = ? AND received_at >= ? AND received_at < ? AND event_name IN ?", environment, from, to, names).
		Group("event_name").Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	counts := map[string]int64{}
	for _, row := range rows {
		counts[row.EventName] = row.Count
	}
	result := make([]FunnelEventCountFact, 0, len(names))
	for _, name := range names {
		result = append(result, FunnelEventCountFact{EventName: name, Count: counts[name]})
	}
	return result, nil
}

func LoadFunnelIdentityCounts(ctx context.Context, environment string, activeSince int64) ([]FunnelIdentityCountFact, error) {
	rows := make([]FunnelIdentityCountFact, 0)
	err := DB.WithContext(ctx).Model(&FunnelVisitor{}).Select("identity_state, COUNT(*) AS count").
		Where("environment = ? AND last_seen_at >= ? AND identity_state IN ?", environment, activeSince, []string{FunnelIdentityLinked, FunnelIdentityAmbiguous}).
		Group("identity_state").Order("identity_state ASC").Scan(&rows).Error
	return rows, err
}

func LoadInvalidFunnelBusinessTimes(ctx context.Context) (invalidTopUps, invalidTasks int64, err error) {
	if err = DB.WithContext(ctx).Model(&TopUp{}).Where("status = ? AND complete_time <= 0", common.TopUpStatusSuccess).Count(&invalidTopUps).Error; err != nil {
		return 0, 0, err
	}
	err = DB.WithContext(ctx).Model(&Task{}).Where("status = ? AND finish_time <= 0", TaskStatusSuccess).Count(&invalidTasks).Error
	return invalidTopUps, invalidTasks, err
}

func LoadFunnelCollectionStart(ctx context.Context, environment string) (int64, error) {
	var option Option
	err := DB.WithContext(ctx).Where("key = ?", "GeiliFunnelCollectionStartedAt."+environment).First(&option).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	value, err := strconv.ParseInt(option.Value, 10, 64)
	if err != nil || value <= 0 {
		return 0, nil
	}
	return value, nil
}

func LoadFunnelLastEventAt(ctx context.Context, environment string) (int64, error) {
	var row struct{ LastEventAt *int64 }
	err := DB.WithContext(ctx).Model(&FunnelEvent{}).Select("MAX(received_at) AS last_event_at").Where("environment = ?", environment).Scan(&row).Error
	if err != nil || row.LastEventAt == nil {
		return 0, err
	}
	return *row.LastEventAt, nil
}

func positiveUniqueUserIDs(userIDs []int) []int {
	set := map[int]struct{}{}
	for _, userID := range userIDs {
		if userID > 0 {
			set[userID] = struct{}{}
		}
	}
	result := make([]int, 0, len(set))
	for userID := range set {
		result = append(result, userID)
	}
	sort.Ints(result)
	return result
}
