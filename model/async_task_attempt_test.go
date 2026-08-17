package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestAsyncTaskAttemptLifecycle(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:async-task-attempt?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&AsyncTaskAttempt{}))

	previous := DB
	DB = db
	t.Cleanup(func() { DB = previous })

	attempt := &AsyncTaskAttempt{
		TaskID:          "task_attempt_test",
		UserID:          7,
		AttemptNo:       1,
		ChannelID:       11,
		Group:           "default",
		Model:           "gpt-image-2",
		Kind:            "image",
		Action:          "generate",
		Status:          AsyncTaskAttemptStatusRunning,
		AcceptanceState: AsyncAttemptAcceptanceNotAccepted,
	}
	require.NoError(t, CreateAsyncTaskAttempt(attempt))
	require.NotZero(t, attempt.ID)
	require.NotZero(t, attempt.StartedAt)

	attempt.Status = AsyncTaskAttemptStatusSucceeded
	attempt.AcceptanceState = AsyncAttemptAcceptanceAccepted
	attempt.CompletedAt = attempt.StartedAt + 1
	require.NoError(t, UpdateAsyncTaskAttempt(attempt))

	attempts, err := GetAsyncTaskAttempts(attempt.TaskID)
	require.NoError(t, err)
	require.Len(t, attempts, 1)
	require.Equal(t, AsyncTaskAttemptStatusSucceeded, attempts[0].Status)
	require.Equal(t, AsyncAttemptAcceptanceAccepted, attempts[0].AcceptanceState)
}

func TestAsyncTaskAttemptNumberIsUniquePerTask(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:async-task-attempt-unique?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&AsyncTaskAttempt{}))

	previous := DB
	DB = db
	t.Cleanup(func() { DB = previous })

	first := &AsyncTaskAttempt{TaskID: "task_unique", AttemptNo: 1, ChannelID: 1, Status: AsyncTaskAttemptStatusRunning}
	second := &AsyncTaskAttempt{TaskID: "task_unique", AttemptNo: 1, ChannelID: 2, Status: AsyncTaskAttemptStatusRunning}
	require.NoError(t, CreateAsyncTaskAttempt(first))
	require.Error(t, CreateAsyncTaskAttempt(second))
}

func TestAsyncMediaChannelCoverageCountsDistinctEnabledChannels(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:async-media-coverage?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&Channel{}, &Ability{}, &ModelRegistry{}))

	previous := DB
	DB = db
	t.Cleanup(func() { DB = previous })

	require.NoError(t, db.Create(&ModelRegistry{ModelName: "gpt-image-2", Slug: "gpt-image-2", Modality: "image", Enabled: true}).Error)
	require.NoError(t, db.Create(&ModelRegistry{ModelName: "text-only", Slug: "text-only", Modality: "text", Enabled: true}).Error)
	for _, channelID := range []int{1, 2} {
		require.NoError(t, db.Create(&Channel{Id: channelID, Key: "test-key", Status: common.ChannelStatusEnabled, Group: "default"}).Error)
		require.NoError(t, db.Create(&Ability{Group: "default", Model: "gpt-image-2", ChannelId: channelID, Enabled: true}).Error)
	}
	require.NoError(t, db.Create(&Ability{Group: "default", Model: "text-only", ChannelId: 1, Enabled: true}).Error)

	coverage, err := GetAsyncMediaChannelCoverage()
	require.NoError(t, err)
	require.Equal(t, []AsyncMediaChannelCoverage{{Group: "default", Model: "gpt-image-2", ChannelCount: 2}}, coverage)
}
