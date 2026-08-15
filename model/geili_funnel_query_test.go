package model

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/require"
)

func seedFunnelUser(t *testing.T, id int, createdAt int64) {
	t.Helper()
	require.NoError(t, DB.Create(&User{
		Id: id, Username: fmt.Sprintf("funnel-user-%d", id), Password: strings.Repeat("p", 16),
		AffCode: fmt.Sprintf("funnel-aff-%d", id), CreatedAt: createdAt,
	}).Error)
}

func seedTopUp(t *testing.T, userID int, status string, completeAt int64) {
	t.Helper()
	tradeNo := fmt.Sprintf("funnel-%d-%s-%d", userID, status, completeAt)
	require.NoError(t, DB.Create(&TopUp{UserId: userID, TradeNo: tradeNo, Status: status, CompleteTime: completeAt}).Error)
}

func seedTask(t *testing.T, userID int, status TaskStatus, finishAt int64) {
	t.Helper()
	require.NoError(t, DB.Create(&Task{
		TaskID: fmt.Sprintf("funnel-%d-%s-%d", userID, status, finishAt), UserId: userID, Status: status, FinishTime: finishAt,
	}).Error)
}

func TestLoadFunnelMilestonesUsesGlobalFirstSuccessfulRows(t *testing.T) {
	setupFunnelTestDB(t)
	seedFunnelUser(t, 7, 100)
	seedTopUp(t, 7, "pending", 110)
	seedTopUp(t, 7, common.TopUpStatusSuccess, 120)
	seedTopUp(t, 7, common.TopUpStatusSuccess, 130)
	seedTask(t, 7, TaskStatusFailure, 140)
	seedTask(t, 7, TaskStatusSuccess, 150)
	seedTask(t, 7, TaskStatusSuccess, 160)

	topups, invalidTopups, err := LoadFirstSuccessfulTopUps(context.Background(), []int{7})
	require.NoError(t, err)
	require.EqualValues(t, 120, topups[7])
	require.Zero(t, invalidTopups)
	tasks, invalidTasks, err := LoadFirstSuccessfulTasks(context.Background(), []int{7})
	require.NoError(t, err)
	require.EqualValues(t, 150, tasks[7])
	require.Zero(t, invalidTasks)

	independent, _, err := LoadIndependentFirstTopUps(context.Background(), 125, 140)
	require.NoError(t, err)
	require.Empty(t, independent, "a later success must not repair a first success outside the window")
}

func TestLoadFirstAPIKeysUsesUserCreatedTokensAndKeepsDeletedHistory(t *testing.T) {
	setupFunnelTestDB(t)
	tokens := []Token{
		{UserId: 7, Name: GeiliStudioOnlineTokenName, Key: "studio-7", CreatedTime: 105},
		{UserId: 7, Name: "production", Key: "user-7-first", CreatedTime: 120},
		{UserId: 7, Name: "backup", Key: "user-7-second", CreatedTime: 130},
		{UserId: 8, Name: "outside", Key: "user-8", CreatedTime: 90},
		{UserId: 9, Name: "invalid", Key: "user-9", CreatedTime: 0},
		{UserId: 10, Name: "deleted", Key: "user-10", CreatedTime: 125},
	}
	for index := range tokens {
		require.NoError(t, DB.Create(&tokens[index]).Error)
	}
	require.NoError(t, DB.Delete(&tokens[5]).Error)

	rows, invalid, err := LoadIndependentFirstAPIKeys(context.Background(), 100, 130)
	require.NoError(t, err)
	require.Equal(t, []FunnelTimedUser{{UserID: 7, At: 120}, {UserID: 10, At: 125}}, rows)
	require.EqualValues(t, 1, invalid)
}

func TestLoadFirstSuccessfulTextCallsUsesConsumeLogsAndGlobalFirstTime(t *testing.T) {
	setupFunnelTestDB(t)
	require.NoError(t, DB.Create(&ModelRegistry{ModelName: "gpt-5.5", Slug: "gpt-5-5", Modality: "text"}).Error)
	require.NoError(t, DB.Create(&ModelRegistry{ModelName: "gpt-image-2", Slug: "gpt-image-2", Modality: "image"}).Error)
	logs := []Log{
		{UserId: 7, Type: LogTypeConsume, CreatedAt: 105, ModelName: "gpt-image-2", PromptTokens: 1, RequestId: "image-7"},
		{UserId: 7, Type: LogTypeConsume, CreatedAt: 120, ModelName: "gpt-5.5", PromptTokens: 1, RequestId: "text-7-first"},
		{UserId: 7, Type: LogTypeConsume, CreatedAt: 130, ModelName: "gpt-5.5", CompletionTokens: 1, RequestId: "text-7-second"},
		{UserId: 8, Type: LogTypeConsume, CreatedAt: 90, ModelName: "gpt-5.5", PromptTokens: 1, RequestId: "text-8-first"},
		{UserId: 8, Type: LogTypeConsume, CreatedAt: 125, ModelName: "gpt-5.5", CompletionTokens: 1, RequestId: "text-8-second"},
		{UserId: 9, Type: LogTypeError, CreatedAt: 125, ModelName: "gpt-5.5", RequestId: "text-9-error"},
		{UserId: 10, Type: LogTypeConsume, CreatedAt: -1, ModelName: "gpt-5.5", PromptTokens: 1, RequestId: "text-10-invalid"},
		{UserId: 11, Type: LogTypeConsume, CreatedAt: 125, ModelName: "gpt-5.5", CompletionTokens: 1, RequestId: "text-11"},
		{UserId: 12, Type: LogTypeConsume, CreatedAt: 126, ModelName: "gpt-5.5", RequestId: "text-12-zero-token"},
	}
	for index := range logs {
		require.NoError(t, LOG_DB.Create(&logs[index]).Error)
	}
	require.NoError(t, DB.Create(&Task{TaskID: "activation-7-media", UserId: 7, Status: TaskStatusSuccess, FinishTime: 121}).Error)
	require.NoError(t, DB.Create(&Task{TaskID: "activation-8-media", UserId: 8, Status: TaskStatusSuccess, FinishTime: 125}).Error)
	require.NoError(t, DB.Create(&Task{TaskID: "activation-13-media", UserId: 13, Status: TaskStatusSuccess, FinishTime: 126}).Error)

	rows, invalid, err := LoadIndependentFirstSuccessfulTextCalls(context.Background(), 100, 130)
	require.NoError(t, err)
	require.Equal(t, []FunnelTimedUser{{UserID: 7, At: 120}, {UserID: 11, At: 125}}, rows)
	require.EqualValues(t, 1, invalid)

	activations, err := LoadIndependentFirstActivations(context.Background(), 100, 130)
	require.NoError(t, err)
	require.Equal(t, []FunnelTimedUser{{UserID: 7, At: 120}, {UserID: 11, At: 125}, {UserID: 13, At: 126}}, activations)
}

func TestLoadFunnelMilestonesExcludesInvalidSuccessTimes(t *testing.T) {
	setupFunnelTestDB(t)
	seedFunnelUser(t, 8, 100)
	seedTopUp(t, 8, common.TopUpStatusSuccess, 0)
	seedTask(t, 8, TaskStatusSuccess, 0)
	topups, invalidTopups, err := LoadFirstSuccessfulTopUps(context.Background(), []int{8})
	require.NoError(t, err)
	require.Empty(t, topups)
	require.EqualValues(t, 1, invalidTopups)
	tasks, invalidTasks, err := LoadFirstSuccessfulTasks(context.Background(), []int{8})
	require.NoError(t, err)
	require.Empty(t, tasks)
	require.EqualValues(t, 1, invalidTasks)
	globalTopUps, globalTasks, err := LoadInvalidFunnelBusinessTimes(context.Background())
	require.NoError(t, err)
	require.EqualValues(t, 1, globalTopUps)
	require.EqualValues(t, 1, globalTasks)
}

func TestLoadFunnelEventFactsKeepEnvironmentAndIdentityBoundaries(t *testing.T) {
	setupFunnelTestDB(t)
	user7 := 7
	user8 := 8
	hashA := strings.Repeat("a", 64)
	hashB := strings.Repeat("b", 64)
	visitors := []FunnelVisitor{
		{Environment: FunnelEnvironmentProduction, VisitorHMAC: &hashA, IdentityState: FunnelIdentityLinked, UserID: &user7, FirstSeenAt: 90, LastSeenAt: 170, FirstSLPAt: 100, FirstSLPLocale: "zh", FirstSLPModel: "gpt-image-2"},
		{Environment: FunnelEnvironmentProduction, VisitorHMAC: &hashB, IdentityState: FunnelIdentityAmbiguous, FirstSeenAt: 100, LastSeenAt: 170, FirstSLPAt: 110, FirstSLPLocale: "en", FirstSLPModel: "seedance-2-0"},
		{Environment: FunnelEnvironmentStaging, IdentityState: FunnelIdentityLinked, UserID: &user8, FirstSeenAt: 90, LastSeenAt: 170, FirstSLPAt: 105, FirstSLPLocale: "en", FirstSLPModel: "gpt-image-2"},
	}
	for i := range visitors {
		require.NoError(t, DB.Create(&visitors[i]).Error)
	}
	events := []FunnelEvent{
		{Environment: FunnelEnvironmentProduction, EventID: "00000000-0000-4000-8000-000000000001", VisitorID: visitors[0].ID, EventName: FunnelEventOpenStudio, EventVersion: 1, ReceivedAt: 150},
		{Environment: FunnelEnvironmentProduction, EventID: "00000000-0000-4000-8000-000000000002", VisitorID: visitors[0].ID, EventName: FunnelEventOpenStudio, EventVersion: 1, ReceivedAt: 140},
		{Environment: FunnelEnvironmentProduction, EventID: "00000000-0000-4000-8000-000000000003", VisitorID: visitors[1].ID, EventName: FunnelEventOpenStudio, EventVersion: 1, ReceivedAt: 145},
		{Environment: FunnelEnvironmentProduction, EventID: "00000000-0000-4000-8000-000000000004", VisitorID: visitors[1].ID, EventName: FunnelEventPlaygroundFail, EventVersion: 1, ReceivedAt: 160, FailureCode: "submit", ModelSlug: "seedance-2-0"},
	}
	for i := range events {
		require.NoError(t, DB.Create(&events[i]).Error)
	}

	entries, err := LoadEntryVisitors(context.Background(), FunnelEnvironmentProduction, 100, 120)
	require.NoError(t, err)
	require.Len(t, entries, 2)
	touches, err := LoadLinkedFirstTouches(context.Background(), FunnelEnvironmentProduction, []int{7, 8})
	require.NoError(t, err)
	require.Len(t, touches, 1)
	require.EqualValues(t, 100, touches[7].FirstSLPAt)
	opens, err := LoadStudioOpenTimes(context.Background(), FunnelEnvironmentProduction, []int{7, 8})
	require.NoError(t, err)
	require.Equal(t, []int64{140, 150}, opens[7])
	require.NotContains(t, opens, 8)
	failures, err := LoadFunnelFailures(context.Background(), FunnelEnvironmentProduction, 100, 200)
	require.NoError(t, err)
	require.Equal(t, []FunnelFailureFact{{FailureCode: "submit", ModelSlug: "seedance-2-0", Count: 1}}, failures)
}

func TestLoadFunnelAggregateFactsAreStable(t *testing.T) {
	setupFunnelTestDB(t)
	counts, err := LoadFunnelEventCounts(context.Background(), FunnelEnvironmentProduction, 0, 100)
	require.NoError(t, err)
	require.Len(t, counts, 5)
	for _, count := range counts {
		require.Zero(t, count.Count)
	}

	linked := 7
	visitors := []FunnelVisitor{
		{Environment: FunnelEnvironmentProduction, IdentityState: FunnelIdentityLinked, UserID: &linked, FirstSeenAt: 1, LastSeenAt: 100},
		{Environment: FunnelEnvironmentProduction, IdentityState: FunnelIdentityAmbiguous, FirstSeenAt: 1, LastSeenAt: 101},
		{Environment: FunnelEnvironmentProduction, IdentityState: FunnelIdentityLinked, UserID: &linked, FirstSeenAt: 1, LastSeenAt: 10},
	}
	for i := range visitors {
		require.NoError(t, DB.Create(&visitors[i]).Error)
	}
	identities, err := LoadFunnelIdentityCounts(context.Background(), FunnelEnvironmentProduction, 50)
	require.NoError(t, err)
	require.Equal(t, []FunnelIdentityCountFact{{IdentityState: FunnelIdentityAmbiguous, Count: 1}, {IdentityState: FunnelIdentityLinked, Count: 1}}, identities)

	require.NoError(t, DB.Create(&FunnelActivityDay{Environment: FunnelEnvironmentProduction, UserID: 7, ActivityDate: 86400, FirstSeenAt: 90000, LastSeenAt: 90000}).Error)
	activity, err := LoadFunnelActivityDays(context.Background(), FunnelEnvironmentProduction, []int{7}, 86400, 172800)
	require.NoError(t, err)
	require.Equal(t, []FunnelActivityFact{{UserID: 7, ActivityDate: 86400}}, activity)
}
