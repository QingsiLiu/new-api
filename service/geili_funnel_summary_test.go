package service

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/require"
)

func TestBuildFunnelSummaryStrictAndIndependentPolicy(t *testing.T) {
	query := FunnelSummaryQuery{
		Environment: model.FunnelEnvironmentProduction,
		Dimension:   "all",
		From:        100,
		To:          200,
		Now:         200,
	}
	facts := strictSummaryFacts()

	response, err := BuildFunnelSummary(query, facts)
	require.NoError(t, err)
	require.Equal(t, "full", response.Metrics.Strict.Coverage)
	require.Equal(t, []int64{6, 4, 3, 2, 1}, stagePeople(t, response.Metrics.Strict.Stages))
	require.Equal(t, 0.6667, *response.Metrics.Strict.Stages[1].RateEntry)
	require.Equal(t, 0.5, *response.Metrics.Strict.Stages[4].RatePrevious)
	require.Len(t, response.Segments, 1)
	require.Equal(t, "all", response.Segments[0].Dimension)
	require.Equal(t, "all", response.Segments[0].Value)

	require.Equal(t, []int64{5, 5, 5, 5}, milestonePeople(t, response.Metrics.Independent))
	for _, milestone := range response.Metrics.Independent {
		require.False(t, milestone.Ordered)
	}
	require.Equal(t, "business_only", response.Metrics.Independent[0].Coverage)
	require.Equal(t, "event_only", response.Metrics.Independent[3].Coverage)
}

func TestBuildFunnelSummaryAttributedMilestonesExcludeUnlinkedBusinessUsers(t *testing.T) {
	facts := strictSummaryFacts()
	user6 := 6
	facts.EntryVisitors[5].UserID = &user6
	facts.EntryVisitors[5].IdentityState = model.FunnelIdentityLinked
	facts.LinkedFirstTouches[user6] = facts.EntryVisitors[5]
	facts.IndependentRegistrations = append(facts.IndependentRegistrations, model.FunnelTimedUser{UserID: user6, At: 116})
	for index := range facts.EntryVisitors {
		facts.EntryVisitors[index].Locale = "zh"
		facts.EntryVisitors[index].ModelSlug = "gpt-image-2"
	}
	for userID, touch := range facts.LinkedFirstTouches {
		touch.Locale = "zh"
		touch.ModelSlug = "gpt-image-2"
		facts.LinkedFirstTouches[userID] = touch
	}
	query := FunnelSummaryQuery{Environment: model.FunnelEnvironmentProduction, Dimension: "model", From: 100, To: 200, Now: 200}

	response, err := BuildFunnelSummary(query, facts)
	require.NoError(t, err)
	require.Len(t, response.Segments, 1)
	milestones := response.Segments[0].Metrics.Independent
	for _, milestone := range milestones {
		require.Equal(t, "attributed_only", milestone.Coverage)
	}
	// User 9 has business milestones but no first touch, so it only appears in the overall totals.
	require.EqualValues(t, 6, *response.Metrics.Independent[0].People)
	require.EqualValues(t, 5, *milestones[0].People)
}

func TestBuildFunnelSummaryCoverageStates(t *testing.T) {
	base := FunnelSummaryQuery{Environment: model.FunnelEnvironmentProduction, Dimension: "all", From: 100, To: 200, Now: 200}
	cases := []struct {
		name       string
		query      FunnelSummaryQuery
		startedAt  int64
		coverage   string
		peopleNull bool
	}{
		{name: "not started", query: base, startedAt: 0, coverage: "not_started", peopleNull: true},
		{name: "pre collection", query: base, startedAt: 250, coverage: "pre_collection", peopleNull: true},
		{name: "collection limited", query: base, startedAt: 150, coverage: "collection_limited"},
		{name: "raw unavailable", query: FunnelSummaryQuery{Environment: model.FunnelEnvironmentProduction, Dimension: "all", From: 100, To: 200, Now: 181*86400 + 100}, startedAt: 90, coverage: "unavailable", peopleNull: true},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			facts := strictSummaryFacts()
			facts.CollectionStartedAt = test.startedAt
			response, err := BuildFunnelSummary(test.query, facts)
			require.NoError(t, err)
			require.Equal(t, test.coverage, response.Metrics.Strict.Coverage)
			if test.peopleNull {
				for _, stage := range response.Metrics.Strict.Stages {
					require.Nil(t, stage.People)
				}
			}
		})
	}
}

func TestBuildFunnelRetentionUsesExactUTCDaysAndSeparatesImmature(t *testing.T) {
	response, err := BuildFunnelSummary(retentionQueryFixture(t), retentionFactsFixture(t))
	require.NoError(t, err)
	require.EqualValues(t, 2, *response.Metrics.Retention[0].Eligible)
	require.EqualValues(t, 1, *response.Metrics.Retention[0].Retained)
	require.Equal(t, 0.5, *response.Metrics.Retention[0].Rate)
	require.EqualValues(t, 1, *response.Metrics.Retention[0].Immature)
	require.Equal(t, "web_linked_only", response.Metrics.Retention[0].Coverage)
	require.EqualValues(t, 2, *response.Metrics.Retention[1].Eligible)
	require.EqualValues(t, 1, *response.Metrics.Retention[1].Retained)
	require.EqualValues(t, 3, *response.Metrics.Retention[2].Immature)
}

func TestBuildFunnelSegmentsSuppressesCellsBelowFive(t *testing.T) {
	response, err := BuildFunnelSummary(modelDimensionQuery(), factsWithModelEntryCounts(map[string]int{"gpt-image-2": 6, "seedance-2-0": 4}))
	require.NoError(t, err)
	require.Len(t, response.Segments, 1)
	require.Equal(t, "gpt-image-2", response.Segments[0].Value)
	require.EqualValues(t, 1, response.Quality.SuppressedSegments)
	for index, stage := range response.Segments[0].Metrics.Strict.Stages {
		if index == 0 {
			require.EqualValues(t, 6, *stage.People)
			continue
		}
		require.Nil(t, stage.People)
		require.Nil(t, stage.RatePrevious)
		require.Nil(t, stage.RateEntry)
		require.True(t, stage.Suppressed)
	}
}

func TestBuildFunnelFailureMetricsAreFixedAndCannotLeakHiddenModels(t *testing.T) {
	facts := factsWithModelEntryCounts(map[string]int{"gpt-image-2": 6, "seedance-2-0": 4})
	facts.Failures = []model.FunnelFailureFact{
		{FailureCode: "submit", ModelSlug: "gpt-image-2", Count: 3},
		{FailureCode: "poll", ModelSlug: "seedance-2-0", Count: 100},
		{FailureCode: "private_dynamic_error", ModelSlug: "gpt-image-2", Count: 100},
	}
	facts.TaskStatuses = []model.FunnelTaskStatusFact{
		{Status: model.TaskStatusSuccess, Count: 7},
		{Status: model.TaskStatusFailure, Count: 2},
	}

	response, err := BuildFunnelSummary(modelDimensionQuery(), facts)
	require.NoError(t, err)
	encoded, err := json.Marshal(response)
	require.NoError(t, err)
	require.NotContains(t, string(encoded), "seedance-2-0")
	require.NotContains(t, string(encoded), "private_dynamic_error")
	require.Contains(t, failureCodes(response.Metrics.Failures), "task_success")
	require.Contains(t, failureCodes(response.Metrics.Failures), "task_failure")
	require.EqualValues(t, 7, *failureByCode(t, response.Metrics.Failures, "task_success").Count)
	require.EqualValues(t, 2, *failureByCode(t, response.Metrics.Failures, "task_failure").Count)

	segmentFailures := response.Segments[0].Metrics.Failures
	require.Len(t, segmentFailures, 1)
	require.Equal(t, "submit", segmentFailures[0].Code)
	require.Nil(t, segmentFailures[0].Count)
	require.True(t, segmentFailures[0].Suppressed)
}

func TestQueryGeiliFunnelSummaryUsesAuthoritativeJourney(t *testing.T) {
	setupFunnelServiceTestDB(t)
	require.NoError(t, model.DB.Create(&model.User{
		Id: 7, Username: "summary-user", Password: strings.Repeat("p", 16), AffCode: "summary-aff", CreatedAt: 105,
	}).Error)
	require.NoError(t, model.DB.Create(&model.TopUp{UserId: 7, TradeNo: "summary-pending", Status: "pending", CompleteTime: 115}).Error)
	require.NoError(t, model.DB.Create(&model.TopUp{UserId: 7, TradeNo: "summary-success", Status: common.TopUpStatusSuccess, CompleteTime: 120}).Error)
	require.NoError(t, model.DB.Create(&model.Task{TaskID: "summary-failed", UserId: 7, Status: model.TaskStatusFailure, FinishTime: 130}).Error)
	require.NoError(t, model.DB.Create(&model.Task{TaskID: "summary-success", UserId: 7, Status: model.TaskStatusSuccess, FinishTime: 140}).Error)

	inputs := []model.FunnelEventInput{
		summaryEvent("00000000-0000-4000-8000-000000000001", model.FunnelEventSLPView, 100),
		summaryEvent("00000000-0000-4000-8000-000000000002", model.FunnelEventIdentityLink, 110),
		summaryEvent("00000000-0000-4000-8000-000000000003", model.FunnelEventOpenStudio, 150),
	}
	inputs[1].Locale, inputs[1].ModelSlug, inputs[1].UserID = "", "", 7
	inputs[2].Locale, inputs[2].ModelSlug, inputs[2].UserID = "", "", 7
	for _, input := range inputs {
		_, err := IngestFunnelEvent(context.Background(), input)
		require.NoError(t, err)
	}
	_, err := IngestFunnelEvent(context.Background(), inputs[0])
	require.NoError(t, err)

	response, err := QueryGeiliFunnelSummary(context.Background(), FunnelSummaryQuery{
		Environment: model.FunnelEnvironmentProduction, Dimension: "all", From: 100, To: 200, Now: 200,
	})
	require.NoError(t, err)
	require.Equal(t, []int64{1, 1, 1, 1, 1}, stagePeople(t, response.Metrics.Strict.Stages))
	require.EqualValues(t, 1, *response.Metrics.Independent[0].People)
	require.GreaterOrEqual(t, response.Quality.DuplicateSinceStart, uint64(1))

	require.NoError(t, model.DB.Migrator().DropTable(&model.FunnelEvent{}))
	_, err = QueryGeiliFunnelSummary(context.Background(), FunnelSummaryQuery{
		Environment: model.FunnelEnvironmentProduction, Dimension: "all", From: 100, To: 200, Now: 200,
	})
	require.Error(t, err)
}

func strictSummaryFacts() FunnelSummaryFacts {
	entries := make([]model.FunnelVisitorFact, 0, 6)
	touches := make(map[int]model.FunnelVisitorFact)
	for userID := 1; userID <= 5; userID++ {
		id := userID
		touch := model.FunnelVisitorFact{
			VisitorID: int64(userID), UserID: &id, IdentityState: model.FunnelIdentityLinked,
			FirstSLPAt: int64(99 + userID), Locale: "zh", ModelSlug: "gpt-image-2",
		}
		entries = append(entries, touch)
		touches[userID] = touch
	}
	entries = append(entries, model.FunnelVisitorFact{
		VisitorID: 6, IdentityState: model.FunnelIdentityAnonymous, FirstSLPAt: 106, Locale: "en", ModelSlug: "seedance-2-0",
	})
	return FunnelSummaryFacts{
		EntryVisitors:            entries,
		LinkedFirstTouches:       touches,
		UserCreated:              map[int]int64{1: 110, 2: 111, 3: 112, 4: 113, 5: 90},
		FirstTopUps:              map[int]int64{1: 120, 2: 121, 3: 122, 5: 123},
		FirstTasks:               map[int]int64{1: 130, 2: 131, 4: 125, 5: 124},
		StudioOpens:              map[int][]int64{1: {140}, 3: {150}, 4: {151}, 5: {152}},
		IndependentRegistrations: []model.FunnelTimedUser{{UserID: 1, At: 110}, {UserID: 2, At: 111}, {UserID: 3, At: 112}, {UserID: 4, At: 113}, {UserID: 9, At: 115}},
		IndependentTopUps:        []model.FunnelTimedUser{{UserID: 1, At: 120}, {UserID: 2, At: 121}, {UserID: 3, At: 122}, {UserID: 5, At: 123}, {UserID: 9, At: 126}},
		IndependentTasks:         []model.FunnelTimedUser{{UserID: 1, At: 130}, {UserID: 2, At: 131}, {UserID: 4, At: 125}, {UserID: 5, At: 124}, {UserID: 9, At: 127}},
		IndependentStudio:        []model.FunnelTimedUser{{UserID: 1, At: 140}, {UserID: 3, At: 150}, {UserID: 4, At: 151}, {UserID: 5, At: 152}, {UserID: 9, At: 153}},
		CollectionStartedAt:      90,
	}
}

func retentionQueryFixture(t *testing.T) FunnelSummaryQuery {
	return FunnelSummaryQuery{
		Environment: model.FunnelEnvironmentProduction,
		Dimension:   "all",
		From:        mustFunnelDay(t, "2026-07-01"),
		To:          mustFunnelDay(t, "2026-07-23"),
		Now:         mustFunnelDay(t, "2026-07-22") + 12*3600,
	}
}

func retentionFactsFixture(t *testing.T) FunnelSummaryFacts {
	jul1 := mustFunnelDay(t, "2026-07-01")
	jul10 := mustFunnelDay(t, "2026-07-10")
	jul21 := mustFunnelDay(t, "2026-07-21")
	created := map[int]int64{1: jul1 + 3600, 2: jul10 + 3600, 3: jul21 + 3600}
	touches := make(map[int]model.FunnelVisitorFact)
	registrations := make([]model.FunnelTimedUser, 0, 3)
	for _, id := range []int{1, 2, 3} {
		userID := id
		touches[id] = model.FunnelVisitorFact{
			VisitorID: int64(id), UserID: &userID, IdentityState: model.FunnelIdentityLinked,
			FirstSLPAt: created[id] - 60, Locale: "zh", ModelSlug: "gpt-image-2",
		}
		registrations = append(registrations, model.FunnelTimedUser{UserID: id, At: created[id]})
	}
	return FunnelSummaryFacts{
		LinkedFirstTouches:       touches,
		UserCreated:              created,
		IndependentRegistrations: registrations,
		ActivityDays: []model.FunnelActivityFact{
			{UserID: 1, ActivityDate: mustFunnelDay(t, "2026-07-02")},
			{UserID: 1, ActivityDate: mustFunnelDay(t, "2026-07-08")},
		},
		CollectionStartedAt: jul1 - 86400,
	}
}

func modelDimensionQuery() FunnelSummaryQuery {
	return FunnelSummaryQuery{Environment: model.FunnelEnvironmentProduction, Dimension: "model", From: 100, To: 200, Now: 200}
}

func factsWithModelEntryCounts(counts map[string]int) FunnelSummaryFacts {
	models := make([]string, 0, len(counts))
	for modelSlug := range counts {
		models = append(models, modelSlug)
	}
	sort.Strings(models)
	visitors := make([]model.FunnelVisitorFact, 0)
	var id int64
	for _, modelSlug := range models {
		for i := 0; i < counts[modelSlug]; i++ {
			id++
			visitors = append(visitors, model.FunnelVisitorFact{
				VisitorID: id, IdentityState: model.FunnelIdentityAnonymous,
				FirstSLPAt: 100 + id, Locale: "zh", ModelSlug: modelSlug,
			})
		}
	}
	return FunnelSummaryFacts{EntryVisitors: visitors, CollectionStartedAt: 90}
}

func summaryEvent(eventID, eventName string, at int64) model.FunnelEventInput {
	return model.FunnelEventInput{
		Environment: model.FunnelEnvironmentProduction, EventID: eventID, EventName: eventName,
		EventVersion: 1, VisitorHMAC: strings.Repeat("a", 64), Locale: "zh",
		ModelSlug: "gpt-image-2", ReceivedAt: at,
	}
}

func stagePeople(t *testing.T, stages []dto.FunnelStage) []int64 {
	t.Helper()
	result := make([]int64, 0, len(stages))
	for _, stage := range stages {
		require.NotNil(t, stage.People)
		result = append(result, *stage.People)
	}
	return result
}

func milestonePeople(t *testing.T, milestones []dto.FunnelMilestone) []int64 {
	t.Helper()
	result := make([]int64, 0, len(milestones))
	for _, milestone := range milestones {
		require.NotNil(t, milestone.People)
		result = append(result, *milestone.People)
	}
	return result
}

func mustFunnelDay(t *testing.T, value string) int64 {
	t.Helper()
	parsed, err := time.Parse("2006-01-02", value)
	require.NoError(t, err)
	return parsed.Unix()
}

func failureCodes(rows []dto.FunnelFailure) []string {
	result := make([]string, 0, len(rows))
	for _, row := range rows {
		result = append(result, row.Code)
	}
	return result
}

func failureByCode(t *testing.T, rows []dto.FunnelFailure, code string) dto.FunnelFailure {
	t.Helper()
	for _, row := range rows {
		if row.Code == code {
			return row
		}
	}
	require.FailNow(t, fmt.Sprintf("missing failure code %q", code))
	return dto.FunnelFailure{}
}
