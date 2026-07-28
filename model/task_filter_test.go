package model

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestResolveTaskStatusFilter(t *testing.T) {
	require.Equal(t, []TaskStatus{
		TaskStatusNotStart,
		TaskStatusSubmitted,
		TaskStatusQueued,
		TaskStatusInProgress,
		TaskStatusUnknown,
	}, ResolveTaskStatusFilter("running"))
	require.Equal(t, []TaskStatus{TaskStatusSuccess}, ResolveTaskStatusFilter("success"))
	require.Equal(t, []TaskStatus{TaskStatusFailure}, ResolveTaskStatusFilter("failure"))
	require.Equal(t, []TaskStatus{TaskStatusSuccess}, ResolveTaskStatusFilter("SUCCESS"))
	require.Nil(t, ResolveTaskStatusFilter("  "))
}

func TestCustomerTaskStatusFiltersApplyToListsAndCounts(t *testing.T) {
	truncateTables(t)

	statuses := []TaskStatus{
		TaskStatusNotStart,
		TaskStatusSubmitted,
		TaskStatusQueued,
		TaskStatusInProgress,
		TaskStatusUnknown,
		TaskStatusSuccess,
		TaskStatusFailure,
	}
	for index, status := range statuses {
		require.NoError(t, DB.Create(&Task{
			TaskID:     fmt.Sprintf("task_filter_%d", index),
			UserId:     101,
			Status:     status,
			SubmitTime: int64(100 + index),
		}).Error)
	}
	require.NoError(t, DB.Create(&Task{
		TaskID:     "task_filter_other_user",
		UserId:     202,
		Status:     TaskStatusInProgress,
		SubmitTime: 200,
	}).Error)

	running := SyncTaskQueryParams{Status: "running"}
	require.Len(t, TaskGetAllUserTask(101, 0, 25, running), 5)
	require.EqualValues(t, 5, TaskCountAllUserTask(101, running))
	require.Len(t, TaskGetAllTasks(0, 25, running), 6)
	require.EqualValues(t, 6, TaskCountAllTasks(running))

	success := SyncTaskQueryParams{Status: "success"}
	require.Len(t, TaskGetAllUserTask(101, 0, 25, success), 1)
	require.EqualValues(t, 1, TaskCountAllUserTask(101, success))

	raw := SyncTaskQueryParams{Status: "FAILURE"}
	require.Len(t, TaskGetAllUserTask(101, 0, 25, raw), 1)
	require.EqualValues(t, 1, TaskCountAllUserTask(101, raw))
}
