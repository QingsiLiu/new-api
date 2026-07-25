package controller

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/require"
)

func TestCreditsV1PricingUsesExactIntegerTenths(t *testing.T) {
	request := asyncTaskRequest{
		Kind: asyncTaskKindVideo,
		Parameters: map[string]interface{}{
			"resolution": "720p",
			"duration":   3,
		},
	}
	spec, ok, err := resolveCreditsV1AsyncSpec(request, "seedance-2.0-mini")
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, 205*common.CreditsQuotaUnit/10, spec.UnitQuota)
	require.Equal(t, 3*205*common.CreditsQuotaUnit/10, spec.TotalQuota)
}

func TestAsyncTaskCreditsResponseDistinguishesChargeAndReservation(t *testing.T) {
	t.Setenv(common.CreditsFeatureFlagEnv, "true")
	success := asyncTaskModelToResponse(&model.Task{
		TaskID: "task-success",
		Status: model.TaskStatusSuccess,
		Quota:  7200,
	})
	require.Equal(t, 7200, *success.Quota)
	require.Equal(t, "2", *success.Credits)
	require.Equal(t, 7200, *success.ReservedQuota)
	require.Equal(t, "2", *success.ReservedCredits)
	require.Equal(t, "settled", success.BillingState)

	failed := asyncTaskModelToResponse(&model.Task{
		TaskID:     "task-failed",
		Status:     model.TaskStatusFailure,
		FailReason: "upstream failed",
		Quota:      7200,
	})
	require.Equal(t, 7200, *failed.Quota)
	require.Equal(t, "2", *failed.Credits)
	require.Equal(t, 7200, *failed.ReservedQuota)
	require.Equal(t, "2", *failed.ReservedCredits)
	require.Equal(t, "refund_requested", failed.BillingState)

	t.Setenv(common.CreditsFeatureFlagEnv, "false")
	legacy := asyncTaskModelToResponse(&model.Task{TaskID: "task-legacy", Quota: 7200})
	require.Nil(t, legacy.Quota)
	require.Nil(t, legacy.Credits)
	require.Nil(t, legacy.ReservedQuota)
	require.Nil(t, legacy.ReservedCredits)
	require.Empty(t, legacy.BillingState)
}

func TestCreditsV1PricingFallsBackForUnprovenModels(t *testing.T) {
	image := asyncTaskRequest{
		Kind:       asyncTaskKindImage,
		Parameters: map[string]interface{}{"resolution": "1k"},
	}
	_, ok, err := resolveCreditsV1AsyncSpec(image, "seedream-5.0")
	require.NoError(t, err)
	require.False(t, ok)

	video := asyncTaskRequest{
		Kind: asyncTaskKindVideo,
		Parameters: map[string]interface{}{
			"resolution": "720p",
			"duration":   5,
		},
	}
	_, ok, err = resolveCreditsV1AsyncSpec(video, "seedance-1.5-pro")
	require.NoError(t, err)
	require.False(t, ok)
}

func TestCreditsV1PricingRejectsQuantityOverflow(t *testing.T) {
	request := asyncTaskRequest{
		Kind: asyncTaskKindImage,
		Parameters: map[string]interface{}{
			"resolution": "4k",
			"n":          int(^uint(0) >> 1),
		},
	}
	_, ok, err := resolveCreditsV1AsyncSpec(request, "gpt-image-2")
	require.True(t, ok)
	require.Error(t, err)

	request = asyncTaskRequest{
		Kind: asyncTaskKindVideo,
		Parameters: map[string]interface{}{
			"resolution": "720p",
			"duration":   int(^uint(0) >> 1),
		},
	}
	_, ok, err = resolveCreditsV1AsyncSpec(request, "seedance-2.0")
	require.True(t, ok)
	require.Error(t, err)
}
