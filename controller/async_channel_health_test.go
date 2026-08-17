package controller

import (
	"testing"

	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/require"
)

func TestAsyncChannelHealthUsesTaskTerminalStatusForUserSuccess(t *testing.T) {
	db := setupAsyncTaskTestDB(t)
	task := model.Task{
		TaskID:   "task_archive_failed",
		Platform: asyncTaskPlatformOpenAI,
		Status:   model.TaskStatusFailure,
	}
	require.NoError(t, db.Create(&task).Error)
	attempts := []model.AsyncTaskAttempt{
		{
			ID:         1,
			TaskID:     task.TaskID,
			AttemptNo:  1,
			ChannelID:  9,
			Model:      "gpt-image-2",
			Kind:       "image",
			Action:     "generate",
			Status:     model.AsyncTaskAttemptStatusSucceeded,
			DurationMS: 120,
		},
	}
	items, summary := aggregateAsyncChannelHealth(attempts)
	require.Len(t, items, 1)
	require.Equal(t, 1, items[0].Successes)
	require.NoError(t, hydrateAsyncTaskOutcomeSummary(attempts, &summary))
	require.Zero(t, summary.SuccessfulTasks)
	require.Equal(t, 1, summary.FailedTasks)
	require.Zero(t, summary.TaskSuccessRate)
}

func TestAsyncChannelHealthCountsRecoveredFinalSuccess(t *testing.T) {
	db := setupAsyncTaskTestDB(t)
	task := model.Task{
		TaskID:   "task_recovered",
		Platform: asyncTaskPlatformOpenAI,
		Status:   model.TaskStatusSuccess,
	}
	require.NoError(t, db.Create(&task).Error)
	attempts := []model.AsyncTaskAttempt{
		{ID: 1, TaskID: task.TaskID, AttemptNo: 1, ChannelID: 1, Model: "gpt-image-2", Kind: "image", Action: "generate", Status: model.AsyncTaskAttemptStatusFailed, Retryable: true},
		{ID: 2, TaskID: task.TaskID, AttemptNo: 2, ChannelID: 2, Model: "gpt-image-2", Kind: "image", Action: "generate", Status: model.AsyncTaskAttemptStatusSucceeded},
	}
	_, summary := aggregateAsyncChannelHealth(attempts)
	require.NoError(t, hydrateAsyncTaskOutcomeSummary(attempts, &summary))
	require.Equal(t, 1, summary.SuccessfulTasks)
	require.Equal(t, 1, summary.RecoveredTasks)
	require.Equal(t, 100.0, summary.FailoverRecovery)
}
