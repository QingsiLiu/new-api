package service

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"sync"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupFunnelServiceTestDB(t *testing.T) {
	t.Helper()
	common.SetMainDatabaseType(common.DatabaseTypeSQLite)
	common.RedisEnabled = false
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared&_busy_timeout=5000", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	model.DB = db
	require.NoError(t, db.AutoMigrate(&model.Option{}, &model.User{}, &model.TopUp{}, &model.Task{}, &model.FunnelVisitor{}, &model.FunnelEvent{}, &model.FunnelActivityDay{}))
	t.Cleanup(func() {
		sqlDB, err := db.DB()
		if err == nil {
			_ = sqlDB.Close()
		}
	})
}

func countFunnelRows[T any](t *testing.T) int64 {
	t.Helper()
	var count int64
	require.NoError(t, model.DB.Model(new(T)).Count(&count).Error)
	return count
}

func seedLinkedFunnelVisitor(t *testing.T, userID int, hash string) {
	t.Helper()
	require.NoError(t, model.DB.Create(&model.FunnelVisitor{
		Environment:   model.FunnelEnvironmentProduction,
		VisitorHMAC:   &hash,
		IdentityState: model.FunnelIdentityLinked,
		UserID:        &userID,
		FirstSeenAt:   1,
		LastSeenAt:    1,
	}).Error)
}

func validFunnelInput(eventID, event string) model.FunnelEventInput {
	return model.FunnelEventInput{
		Environment:  model.FunnelEnvironmentProduction,
		EventID:      eventID,
		EventName:    event,
		EventVersion: 1,
		VisitorHMAC:  strings.Repeat("a", 64),
		Locale:       "zh",
		ModelSlug:    "gpt-image-2",
		ReceivedAt:   100,
	}
}

func TestIngestFunnelEventIsIdempotentAndFirstTouchIsImmutable(t *testing.T) {
	setupFunnelServiceTestDB(t)
	input := validFunnelInput("7dfb2d2c-7f40-4f39-b8f4-5fb27db06041", model.FunnelEventSLPView)
	first, err := IngestFunnelEvent(context.Background(), input)
	require.NoError(t, err)
	require.False(t, first.Duplicate)

	replay, err := IngestFunnelEvent(context.Background(), input)
	require.NoError(t, err)
	require.True(t, replay.Duplicate)
	require.Equal(t, first.VisitorID, replay.VisitorID)

	input.EventID = "eafac9bf-6be7-4a1d-8436-a6804d8152cc"
	input.ReceivedAt = 200
	input.Locale = "en"
	input.ModelSlug = "seedance-2-0"
	_, err = IngestFunnelEvent(context.Background(), input)
	require.NoError(t, err)

	var visitor model.FunnelVisitor
	require.NoError(t, model.DB.First(&visitor, first.VisitorID).Error)
	require.EqualValues(t, 100, visitor.FirstSLPAt)
	require.Equal(t, "zh", visitor.FirstSLPLocale)
	require.Equal(t, "gpt-image-2", visitor.FirstSLPModel)
	require.EqualValues(t, 2, countFunnelRows[model.FunnelEvent](t))
}

func TestIngestFunnelIdentityBecomesPermanentlyAmbiguous(t *testing.T) {
	setupFunnelServiceTestDB(t)
	hash := strings.Repeat("b", 64)
	for i, userID := range []int{7, 7, 8, 7} {
		input := model.FunnelEventInput{
			Environment:  model.FunnelEnvironmentProduction,
			EventID:      fmt.Sprintf("00000000-0000-4000-8000-%012d", i+1),
			EventName:    model.FunnelEventIdentityLink,
			EventVersion: 1,
			VisitorHMAC:  hash,
			UserID:       userID,
			ReceivedAt:   int64(100 + i),
		}
		_, err := IngestFunnelEvent(context.Background(), input)
		require.NoError(t, err)
	}
	var visitor model.FunnelVisitor
	require.NoError(t, model.DB.Where("visitor_hmac = ?", hash).First(&visitor).Error)
	require.Equal(t, model.FunnelIdentityAmbiguous, visitor.IdentityState)
	require.Nil(t, visitor.UserID)
}

func TestAccountActiveUpsertsOneUTCUserDay(t *testing.T) {
	setupFunnelServiceTestDB(t)
	hash := strings.Repeat("c", 64)
	seedLinkedFunnelVisitor(t, 7, hash)
	for i, at := range []int64{100000, 100100} {
		input := model.FunnelEventInput{
			Environment:  model.FunnelEnvironmentProduction,
			EventID:      fmt.Sprintf("10000000-0000-4000-8000-%012d", i+1),
			EventName:    model.FunnelEventAccountActive,
			EventVersion: 1,
			VisitorHMAC:  hash,
			UserID:       7,
			ReceivedAt:   at,
		}
		_, err := IngestFunnelEvent(context.Background(), input)
		require.NoError(t, err)
	}
	var days []model.FunnelActivityDay
	require.NoError(t, model.DB.Find(&days).Error)
	require.Len(t, days, 1)
	require.EqualValues(t, 86400, days[0].ActivityDate)
	require.EqualValues(t, 100000, days[0].FirstSeenAt)
	require.EqualValues(t, 100100, days[0].LastSeenAt)
}

func TestIngestFunnelConcurrentReplayCreatesOneRecord(t *testing.T) {
	setupFunnelServiceTestDB(t)
	input := validFunnelInput("9dfb2d2c-7f40-4f39-b8f4-5fb27db06041", model.FunnelEventAccountActive)
	input.Locale = ""
	input.ModelSlug = ""
	input.UserID = 7
	results := make(chan model.FunnelIngestResult, 2)
	errors := make(chan error, 2)
	var wg sync.WaitGroup
	for range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			result, err := IngestFunnelEvent(context.Background(), input)
			results <- result
			errors <- err
		}()
	}
	wg.Wait()
	close(results)
	close(errors)

	var accepted, duplicate int
	for result := range results {
		if result.Duplicate {
			duplicate++
		} else {
			accepted++
		}
	}
	for err := range errors {
		require.NoError(t, err)
	}
	require.Equal(t, 1, accepted)
	require.Equal(t, 1, duplicate)
	require.EqualValues(t, 1, countFunnelRows[model.FunnelVisitor](t))
	require.EqualValues(t, 1, countFunnelRows[model.FunnelEvent](t))
	require.EqualValues(t, 1, countFunnelRows[model.FunnelActivityDay](t))
	require.EqualValues(t, 1, countFunnelRows[model.Option](t))
}

func TestIngestFunnelRollsBackVisitorAndCollectionStartOnEventFailure(t *testing.T) {
	setupFunnelServiceTestDB(t)
	callbackName := "funnel_test_fail_event_insert"
	require.NoError(t, model.DB.Callback().Create().Before("gorm:create").Register(callbackName, func(tx *gorm.DB) {
		if tx.Statement.Schema != nil && tx.Statement.Schema.Name == "FunnelEvent" {
			tx.AddError(fmt.Errorf("intentional funnel event failure"))
		}
	}))
	t.Cleanup(func() { _ = model.DB.Callback().Create().Remove(callbackName) })

	input := validFunnelInput("adfb2d2c-7f40-4f39-b8f4-5fb27db06041", model.FunnelEventAccountActive)
	input.Locale = ""
	input.ModelSlug = ""
	input.UserID = 7
	_, err := IngestFunnelEvent(context.Background(), input)
	require.Error(t, err)
	require.EqualValues(t, 0, countFunnelRows[model.FunnelVisitor](t))
	require.EqualValues(t, 0, countFunnelRows[model.FunnelEvent](t))
	require.EqualValues(t, 0, countFunnelRows[model.FunnelActivityDay](t))
	require.EqualValues(t, 0, countFunnelRows[model.Option](t))
}

func TestCollectionStartIsPerEnvironmentAndKeepsEarliestAcceptedTime(t *testing.T) {
	setupFunnelServiceTestDB(t)
	first := validFunnelInput("bdfb2d2c-7f40-4f39-b8f4-5fb27db06041", model.FunnelEventSLPView)
	first.ReceivedAt = 200
	require.NoError(t, func() error { _, err := IngestFunnelEvent(context.Background(), first); return err }())

	earlier := validFunnelInput("bdfb2d2c-7f40-4f39-b8f4-5fb27db06042", model.FunnelEventSLPView)
	earlier.ReceivedAt = 100
	earlier.VisitorHMAC = strings.Repeat("d", 64)
	require.NoError(t, func() error { _, err := IngestFunnelEvent(context.Background(), earlier); return err }())

	staging := earlier
	staging.EventID = "bdfb2d2c-7f40-4f39-b8f4-5fb27db06043"
	staging.Environment = model.FunnelEnvironmentStaging
	staging.VisitorHMAC = strings.Repeat("e", 64)
	staging.ReceivedAt = 300
	require.NoError(t, func() error { _, err := IngestFunnelEvent(context.Background(), staging); return err }())

	var options []model.Option
	require.NoError(t, model.DB.Order("key").Find(&options).Error)
	require.Len(t, options, 2)
	require.Equal(t, "GeiliFunnelCollectionStartedAt.production", options[0].Key)
	require.Equal(t, "100", options[0].Value)
	require.Equal(t, "GeiliFunnelCollectionStartedAt.staging", options[1].Key)
	require.Equal(t, "300", options[1].Value)
}

func TestFunnelIngestValidationMatrix(t *testing.T) {
	setupFunnelServiceTestDB(t)
	cases := []struct {
		name   string
		input  model.FunnelEventInput
		status int
	}{
		{name: "bad uuid", input: func() model.FunnelEventInput {
			in := validFunnelInput("not-a-uuid", model.FunnelEventSLPView)
			return in
		}(), status: 400},
		{name: "bad environment", input: func() model.FunnelEventInput {
			in := validFunnelInput("7dfb2d2c-7f40-4f39-b8f4-5fb27db06042", model.FunnelEventSLPView)
			in.Environment = "qa"
			return in
		}(), status: 400},
		{name: "unknown event", input: func() model.FunnelEventInput {
			in := validFunnelInput("7dfb2d2c-7f40-4f39-b8f4-5fb27db06043", "signup")
			return in
		}(), status: 422},
		{name: "public user forbidden", input: func() model.FunnelEventInput {
			in := validFunnelInput("7dfb2d2c-7f40-4f39-b8f4-5fb27db06044", model.FunnelEventSLPView)
			in.UserID = 7
			return in
		}(), status: 422},
		{name: "trusted user required", input: func() model.FunnelEventInput {
			in := validFunnelInput("7dfb2d2c-7f40-4f39-b8f4-5fb27db06045", model.FunnelEventIdentityLink)
			in.Locale = ""
			in.ModelSlug = ""
			return in
		}(), status: 422},
		{name: "failure code whitelist", input: func() model.FunnelEventInput {
			in := validFunnelInput("7dfb2d2c-7f40-4f39-b8f4-5fb27db06046", model.FunnelEventPlaygroundFail)
			in.FailureCode = "raw-error"
			return in
		}(), status: 422},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := IngestFunnelEvent(context.Background(), tc.input)
			require.Error(t, err)
			var inputErr *FunnelInputError
			require.ErrorAs(t, err, &inputErr)
			require.Equal(t, tc.status, inputErr.Status)
		})
	}
}

func TestFunnelModelsContainNoSensitiveNamedColumns(t *testing.T) {
	for _, typ := range []reflect.Type{reflect.TypeOf(model.FunnelVisitor{}), reflect.TypeOf(model.FunnelEvent{}), reflect.TypeOf(model.FunnelActivityDay{})} {
		for i := 0; i < typ.NumField(); i++ {
			name := strings.ToLower(typ.Field(i).Name)
			for _, forbidden := range []string{"ip", "agent", "referer", "email", "username", "prompt", "url", "error", "metadata"} {
				require.NotContains(t, name, forbidden, "%s contains forbidden field fragment %s", typ.Field(i).Name, forbidden)
			}
		}
	}
}

func TestFunnelIngestCountersRecordExactlyOneOutcome(t *testing.T) {
	before := GetFunnelIngestCounters()
	recordFunnelIngestOutcome(model.FunnelIngestResult{}, nil)
	recordFunnelIngestOutcome(model.FunnelIngestResult{Duplicate: true}, nil)
	recordFunnelIngestOutcome(model.FunnelIngestResult{}, fmt.Errorf("write failed"))
	RecordFunnelRejectedRequest()
	after := GetFunnelIngestCounters()

	require.Equal(t, before.Accepted+1, after.Accepted)
	require.Equal(t, before.Duplicate+1, after.Duplicate)
	require.Equal(t, before.Failed+1, after.Failed)
	require.Equal(t, before.Rejected+1, after.Rejected)
	require.Equal(t, before.Since, after.Since)
}
