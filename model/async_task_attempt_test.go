package model

import (
	"testing"

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
