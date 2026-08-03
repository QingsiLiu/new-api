package model

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRunFunnelMaintenanceReconcilesThenAppliesExactRetentionBoundaries(t *testing.T) {
	setupFunnelTestDB(t)
	const day = int64(86400)
	now := int64(800) * day
	rawCutoff := int64(620) * day
	activityCutoff := int64(70) * day

	recentUser := 7
	recentHash := strings.Repeat("a", 64)
	recent := FunnelVisitor{
		Environment: FunnelEnvironmentProduction, VisitorHMAC: &recentHash,
		IdentityState: FunnelIdentityLinked, UserID: &recentUser,
		FirstSeenAt: rawCutoff, LastSeenAt: rawCutoff + day, FirstSLPAt: rawCutoff,
	}
	require.NoError(t, DB.Create(&recent).Error)
	for index, at := range []int64{rawCutoff + 10, rawCutoff + day + 10, rawCutoff + 2*day + 10} {
		require.NoError(t, DB.Create(&FunnelEvent{
			Environment: FunnelEnvironmentProduction,
			EventID:     fmt.Sprintf("00000000-0000-4000-8000-%012d", index+1),
			VisitorID:   recent.ID, EventName: FunnelEventAccountActive, EventVersion: 1, ReceivedAt: at,
		}).Error)
	}

	ancientLinked := seedMaintenanceVisitor(t, "b", FunnelIdentityLinked, 8, rawCutoff-10, activityCutoff-1)
	noTouchLinked := seedMaintenanceVisitor(t, "c", FunnelIdentityLinked, 9, rawCutoff-10, 0)
	preservedLinked := seedMaintenanceVisitor(t, "d", FunnelIdentityLinked, 10, rawCutoff-10, activityCutoff)
	oldAnonymous := seedMaintenanceVisitor(t, "e", FunnelIdentityAnonymous, 0, rawCutoff-10, 0)
	oldAmbiguous := seedMaintenanceVisitor(t, "f", FunnelIdentityAmbiguous, 0, rawCutoff-10, 0)
	boundaryAnonymous := seedMaintenanceVisitor(t, "1", FunnelIdentityAnonymous, 0, rawCutoff, 0)

	oldVisitors := []FunnelVisitor{ancientLinked, oldAnonymous, oldAmbiguous}
	for index, visitor := range oldVisitors {
		require.NoError(t, DB.Create(&FunnelEvent{
			Environment: FunnelEnvironmentProduction,
			EventID:     fmt.Sprintf("10000000-0000-4000-8000-%012d", index+1),
			VisitorID:   visitor.ID, EventName: FunnelEventSLPView, EventVersion: 1, ReceivedAt: rawCutoff - 1,
		}).Error)
	}
	require.NoError(t, DB.Create(&FunnelEvent{
		Environment: FunnelEnvironmentProduction, EventID: "20000000-0000-4000-8000-000000000001",
		VisitorID: boundaryAnonymous.ID, EventName: FunnelEventSLPView, EventVersion: 1, ReceivedAt: rawCutoff,
	}).Error)

	for index, activityDate := range []int64{activityCutoff - day, activityCutoff - 2*day, activityCutoff - 3*day, activityCutoff} {
		require.NoError(t, DB.Create(&FunnelActivityDay{
			Environment: FunnelEnvironmentProduction, UserID: 100 + index,
			ActivityDate: activityDate, FirstSeenAt: activityDate + 1, LastSeenAt: activityDate + 1,
		}).Error)
	}

	result, err := RunFunnelMaintenance(context.Background(), now, 2)
	require.NoError(t, err)
	require.EqualValues(t, 3, result.ReconciledActivityDays)
	require.EqualValues(t, 3, result.DeletedEvents)
	require.EqualValues(t, 5, result.AnonymizedVisitors)
	require.EqualValues(t, 4, result.DeletedVisitors)
	require.EqualValues(t, 3, result.DeletedActivityDays)
	require.EqualValues(t, rawCutoff, result.RawCutoff)
	require.EqualValues(t, activityCutoff, result.ActivityCutoff)

	requireMaintenanceVisitorMissing(t, ancientLinked.ID)
	requireMaintenanceVisitorMissing(t, noTouchLinked.ID)
	requireMaintenanceVisitorMissing(t, oldAnonymous.ID)
	requireMaintenanceVisitorMissing(t, oldAmbiguous.ID)
	var preserved FunnelVisitor
	require.NoError(t, DB.First(&preserved, preservedLinked.ID).Error)
	require.Nil(t, preserved.VisitorHMAC)
	require.NoError(t, DB.First(&FunnelVisitor{}, boundaryAnonymous.ID).Error)

	var eventCount int64
	require.NoError(t, DB.Model(&FunnelEvent{}).Where("received_at = ?", rawCutoff).Count(&eventCount).Error)
	require.EqualValues(t, 1, eventCount)
	var boundaryDays int64
	require.NoError(t, DB.Model(&FunnelActivityDay{}).Where("activity_date = ?", activityCutoff).Count(&boundaryDays).Error)
	require.EqualValues(t, 1, boundaryDays)
	for _, at := range []int64{rawCutoff + 10, rawCutoff + day + 10, rawCutoff + 2*day + 10} {
		var count int64
		require.NoError(t, DB.Model(&FunnelActivityDay{}).
			Where("environment = ? AND user_id = ? AND activity_date = ?", FunnelEnvironmentProduction, recentUser, utcFunnelDay(at)).
			Count(&count).Error)
		require.EqualValues(t, 1, count)
	}

	encoded, err := json.Marshal(result)
	require.NoError(t, err)
	for _, forbidden := range []string{"visitor_hmac", "user_id", recentHash} {
		require.NotContains(t, string(encoded), forbidden)
	}

	second, err := RunFunnelMaintenance(context.Background(), now, 2)
	require.NoError(t, err)
	require.Zero(t, second.ReconciledActivityDays)
	require.Zero(t, second.DeletedEvents)
	require.Zero(t, second.AnonymizedVisitors)
	require.Zero(t, second.DeletedVisitors)
	require.Zero(t, second.DeletedActivityDays)
}

func TestRunFunnelMaintenanceStopsBeforeDestructivePhasesWhenCanceled(t *testing.T) {
	setupFunnelTestDB(t)
	now := int64(800 * 86400)
	visitor := seedMaintenanceVisitor(t, "a", FunnelIdentityAnonymous, 0, now-181*86400, 0)
	require.NoError(t, DB.Create(&FunnelEvent{
		Environment: FunnelEnvironmentProduction, EventID: "30000000-0000-4000-8000-000000000001",
		VisitorID: visitor.ID, EventName: FunnelEventSLPView, EventVersion: 1, ReceivedAt: now - 181*86400,
	}).Error)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	result, err := RunFunnelMaintenance(ctx, now, 2)
	require.ErrorIs(t, err, context.Canceled)
	require.Zero(t, result.DeletedEvents)
	require.NoError(t, DB.First(&FunnelEvent{}, "visitor_id = ?", visitor.ID).Error)
	require.NoError(t, DB.First(&FunnelVisitor{}, visitor.ID).Error)
}

func TestRunFunnelMaintenanceRejectsInvalidInputs(t *testing.T) {
	setupFunnelTestDB(t)
	_, err := RunFunnelMaintenance(context.Background(), 0, 2)
	require.ErrorIs(t, err, ErrInvalidFunnelMaintenanceInput)
	_, err = RunFunnelMaintenance(context.Background(), 1, 0)
	require.ErrorIs(t, err, ErrInvalidFunnelMaintenanceInput)
}

func seedMaintenanceVisitor(t *testing.T, hashChar, state string, userID int, lastSeenAt, firstSLPAt int64) FunnelVisitor {
	t.Helper()
	hash := strings.Repeat(hashChar, 64)
	visitor := FunnelVisitor{
		Environment: FunnelEnvironmentProduction, VisitorHMAC: &hash, IdentityState: state,
		FirstSeenAt: lastSeenAt, LastSeenAt: lastSeenAt, FirstSLPAt: firstSLPAt,
	}
	if userID > 0 {
		visitor.UserID = &userID
	}
	require.NoError(t, DB.Create(&visitor).Error)
	return visitor
}

func requireMaintenanceVisitorMissing(t *testing.T, id int64) {
	t.Helper()
	var count int64
	require.NoError(t, DB.Model(&FunnelVisitor{}).Where("id = ?", id).Count(&count).Error)
	require.Zero(t, count)
}
