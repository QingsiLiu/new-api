package service

import (
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/alicebob/miniredis/v2"
	"github.com/go-redis/redis/v8"
	"github.com/stretchr/testify/require"
)

func TestAsyncCircuitOpensAndAllowsOnlyOneHalfOpenProbe(t *testing.T) {
	setupAsyncCircuitRedisTest(t)
	configureAsyncCircuitTest(t, 2, 100, 40, 60, 30)
	now, advance := setAsyncCircuitTestClock(t)
	key := AsyncCircuitKey{ChannelID: 7101, Model: "gpt-image-2", Kind: "image", Action: "generate"}

	decision := AcquireAsyncCircuit(key)
	require.True(t, decision.Allowed)
	require.Equal(t, AsyncCircuitStateClosed, decision.State)

	status := RecordAsyncCircuitFailure(key, "", false)
	require.Equal(t, AsyncCircuitStateClosed, status.State)
	require.EqualValues(t, 1, status.ConsecutiveFailures)
	status = RecordAsyncCircuitFailure(key, "", false)
	require.Equal(t, AsyncCircuitStateOpen, status.State)
	require.EqualValues(t, 1, status.BackoffLevel)
	require.EqualValues(t, 60, status.RetryAfterSeconds)

	decision = AcquireAsyncCircuit(key)
	require.False(t, decision.Allowed)
	require.Equal(t, AsyncCircuitStateOpen, decision.State)

	advance(61 * time.Second)
	const workers = 24
	decisions := make(chan AsyncCircuitDecision, workers)
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			decisions <- AcquireAsyncCircuit(key)
		}()
	}
	wg.Wait()
	close(decisions)

	allowed := 0
	for candidate := range decisions {
		if candidate.Allowed {
			allowed++
			require.Equal(t, AsyncCircuitStateHalfOpen, candidate.State)
			require.NotEmpty(t, candidate.ProbeToken)
		} else {
			require.Equal(t, AsyncCircuitStateHalfOpen, candidate.State)
		}
	}
	require.Equal(t, 1, allowed)
	require.Equal(t, time.Unix(1_800_000_000, 0).Add(61*time.Second), now())
}

func TestAsyncCircuitProbeOwnerSuccessRecovers(t *testing.T) {
	setupAsyncCircuitRedisTest(t)
	configureAsyncCircuitTest(t, 1, 100, 40, 10, 30)
	_, advance := setAsyncCircuitTestClock(t)
	key := AsyncCircuitKey{ChannelID: 7102, Model: "seedance-2.0", Kind: "video", Action: "generate"}

	status := RecordAsyncCircuitFailure(key, "", false)
	require.Equal(t, AsyncCircuitStateOpen, status.State)
	advance(11 * time.Second)
	probe := AcquireAsyncCircuit(key)
	require.True(t, probe.Allowed)
	require.Equal(t, AsyncCircuitStateHalfOpen, probe.State)
	require.NotEmpty(t, probe.ProbeToken)

	status = RecordAsyncCircuitSuccess(key, "not-the-owner")
	require.Equal(t, AsyncCircuitStateHalfOpen, status.State)
	require.True(t, status.ProbeActive)

	status = RecordAsyncCircuitSuccess(key, probe.ProbeToken)
	require.Equal(t, AsyncCircuitStateClosed, status.State)
	require.False(t, status.ProbeActive)
	require.Zero(t, status.BackoffLevel)
	require.True(t, AcquireAsyncCircuit(key).Allowed)
}

func TestAsyncCircuitHalfOpenFailureUsesFiveAndFifteenStepBackoff(t *testing.T) {
	setupAsyncCircuitRedisTest(t)
	configureAsyncCircuitTest(t, 1, 100, 40, 10, 30)
	_, advance := setAsyncCircuitTestClock(t)
	key := AsyncCircuitKey{ChannelID: 7103, Model: "gemini-image", Kind: "image", Action: "generate"}

	status := RecordAsyncCircuitFailure(key, "", false)
	require.Equal(t, AsyncCircuitStateOpen, status.State)
	require.EqualValues(t, 10, status.RetryAfterSeconds)
	require.EqualValues(t, 1, status.BackoffLevel)

	advance(11 * time.Second)
	probe := AcquireAsyncCircuit(key)
	require.True(t, probe.Allowed)
	status = RecordAsyncCircuitFailure(key, "not-the-owner", false)
	require.Equal(t, AsyncCircuitStateHalfOpen, status.State)
	require.EqualValues(t, 1, status.BackoffLevel)
	require.True(t, status.ProbeActive)
	status = RecordAsyncCircuitFailure(key, probe.ProbeToken, false)
	require.Equal(t, AsyncCircuitStateOpen, status.State)
	require.EqualValues(t, 50, status.RetryAfterSeconds)
	require.EqualValues(t, 2, status.BackoffLevel)

	advance(51 * time.Second)
	probe = AcquireAsyncCircuit(key)
	require.True(t, probe.Allowed)
	status = RecordAsyncCircuitFailure(key, probe.ProbeToken, false)
	require.Equal(t, AsyncCircuitStateOpen, status.State)
	require.EqualValues(t, 150, status.RetryAfterSeconds)
	require.EqualValues(t, 3, status.BackoffLevel)
}

func TestAsyncCircuitUsesWindowSuccessRateThreshold(t *testing.T) {
	setupAsyncCircuitRedisTest(t)
	configureAsyncCircuitTest(t, 100, 5, 50, 60, 30)
	setAsyncCircuitTestClock(t)
	key := AsyncCircuitKey{ChannelID: 7104, Model: "window-model", Kind: "image", Action: "generate"}

	require.Equal(t, AsyncCircuitStateClosed, RecordAsyncCircuitSuccess(key, "").State)
	require.Equal(t, AsyncCircuitStateClosed, RecordAsyncCircuitSuccess(key, "").State)
	require.Equal(t, AsyncCircuitStateClosed, RecordAsyncCircuitFailure(key, "", false).State)
	require.Equal(t, AsyncCircuitStateClosed, RecordAsyncCircuitFailure(key, "", false).State)
	status := RecordAsyncCircuitFailure(key, "", false)

	require.Equal(t, AsyncCircuitStateOpen, status.State)
	require.EqualValues(t, 5, status.WindowSamples)
	require.EqualValues(t, 3, status.WindowFailures)
}

func TestAsyncCircuitImmediateFailureAndResetByChannel(t *testing.T) {
	setupAsyncCircuitRedisTest(t)
	configureAsyncCircuitTest(t, 100, 100, 40, 60, 30)
	setAsyncCircuitTestClock(t)
	first := AsyncCircuitKey{ChannelID: 7105, Model: "model-a", Kind: "image", Action: "generate"}
	second := AsyncCircuitKey{ChannelID: 7105, Model: "model-b", Kind: "video", Action: "generate"}
	other := AsyncCircuitKey{ChannelID: 7106, Model: "model-c", Kind: "image", Action: "edit"}

	require.Equal(t, AsyncCircuitStateOpen, RecordAsyncCircuitFailure(first, "", true).State)
	require.Equal(t, AsyncCircuitStateOpen, RecordAsyncCircuitFailure(second, "", true).State)
	require.Equal(t, AsyncCircuitStateOpen, RecordAsyncCircuitFailure(other, "", true).State)

	deleted, err := ResetAsyncCircuitByChannel(7105)
	require.NoError(t, err)
	require.Positive(t, deleted)
	require.Equal(t, AsyncCircuitStateClosed, GetAsyncCircuitStatus(first).State)
	require.Equal(t, AsyncCircuitStateClosed, GetAsyncCircuitStatus(second).State)
	require.Equal(t, AsyncCircuitStateOpen, GetAsyncCircuitStatus(other).State)
}

func TestAsyncCircuitRedisFailureIsFailOpen(t *testing.T) {
	configureAsyncCircuitTest(t, 1, 1, 40, 60, 30)
	previousRedis := common.RDB
	previousEnabled := common.RedisEnabled
	broken := redis.NewClient(&redis.Options{
		Addr:         "127.0.0.1:1",
		DialTimeout:  5 * time.Millisecond,
		ReadTimeout:  5 * time.Millisecond,
		WriteTimeout: 5 * time.Millisecond,
		MaxRetries:   0,
	})
	common.RDB = broken
	common.RedisEnabled = true
	t.Cleanup(func() {
		_ = broken.Close()
		common.RDB = previousRedis
		common.RedisEnabled = previousEnabled
	})
	key := AsyncCircuitKey{ChannelID: 7107, Model: "fail-open-model", Kind: "image", Action: "generate"}

	decision := AcquireAsyncCircuit(key)
	require.True(t, decision.Allowed)
	require.Equal(t, AsyncCircuitStateUnknown, decision.State)
	require.True(t, decision.Degraded)
	status := RecordAsyncCircuitFailure(key, "", true)
	require.Equal(t, AsyncCircuitStateUnknown, status.State)
	require.True(t, status.Degraded)
}

func TestAsyncCircuitKeysHashModelAndOperation(t *testing.T) {
	keys, ok := buildAsyncCircuitRedisKeys(AsyncCircuitKey{
		ChannelID: 7108,
		Model:     "private-provider-model-name",
		Kind:      "image",
		Action:    "generate",
	})
	require.True(t, ok)
	require.NotContains(t, keys.base, "private-provider-model-name")
	require.Contains(t, keys.base, asyncCircuitChannelPrefix(7108))
}

func setupAsyncCircuitRedisTest(t *testing.T) *miniredis.Miniredis {
	t.Helper()
	server := miniredis.RunT(t)
	previousRedis := common.RDB
	previousEnabled := common.RedisEnabled
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	common.RDB = client
	common.RedisEnabled = true
	t.Cleanup(func() {
		_ = client.Close()
		common.RDB = previousRedis
		common.RedisEnabled = previousEnabled
	})
	return server
}

func configureAsyncCircuitTest(t *testing.T, failureThreshold int, minimumSamples int, successRateThreshold int, initialOpenSeconds int, probeLeaseSeconds int) {
	t.Helper()
	previous := operation_setting.GetAsyncFailoverSetting()
	require.NoError(t, operation_setting.UpdateAsyncFailoverOption(operation_setting.AsyncCircuitEnabledOption, "true"))
	require.NoError(t, operation_setting.UpdateAsyncFailoverOption(operation_setting.AsyncCircuitFailureThresholdOption, strconv.Itoa(failureThreshold)))
	require.NoError(t, operation_setting.UpdateAsyncFailoverOption(operation_setting.AsyncCircuitWindowSecondsOption, "300"))
	require.NoError(t, operation_setting.UpdateAsyncFailoverOption(operation_setting.AsyncCircuitMinimumSamplesOption, strconv.Itoa(minimumSamples)))
	require.NoError(t, operation_setting.UpdateAsyncFailoverOption(operation_setting.AsyncCircuitSuccessRateThresholdOption, strconv.Itoa(successRateThreshold)))
	require.NoError(t, operation_setting.UpdateAsyncFailoverOption(operation_setting.AsyncCircuitInitialOpenSecondsOption, strconv.Itoa(initialOpenSeconds)))
	require.NoError(t, operation_setting.UpdateAsyncFailoverOption(operation_setting.AsyncCircuitProbeLeaseSecondsOption, strconv.Itoa(probeLeaseSeconds)))
	t.Cleanup(func() {
		_ = operation_setting.UpdateAsyncFailoverOption(operation_setting.AsyncCircuitEnabledOption, strconv.FormatBool(previous.CircuitEnabled))
		_ = operation_setting.UpdateAsyncFailoverOption(operation_setting.AsyncCircuitFailureThresholdOption, strconv.Itoa(previous.CircuitFailureThreshold))
		_ = operation_setting.UpdateAsyncFailoverOption(operation_setting.AsyncCircuitWindowSecondsOption, strconv.Itoa(previous.CircuitWindowSeconds))
		_ = operation_setting.UpdateAsyncFailoverOption(operation_setting.AsyncCircuitMinimumSamplesOption, strconv.Itoa(previous.CircuitMinimumSamples))
		_ = operation_setting.UpdateAsyncFailoverOption(operation_setting.AsyncCircuitSuccessRateThresholdOption, strconv.Itoa(previous.CircuitSuccessRateThreshold))
		_ = operation_setting.UpdateAsyncFailoverOption(operation_setting.AsyncCircuitInitialOpenSecondsOption, strconv.Itoa(previous.CircuitInitialOpenSeconds))
		_ = operation_setting.UpdateAsyncFailoverOption(operation_setting.AsyncCircuitProbeLeaseSecondsOption, strconv.Itoa(previous.CircuitProbeLeaseSeconds))
	})
}

func setAsyncCircuitTestClock(t *testing.T) (func() time.Time, func(time.Duration)) {
	t.Helper()
	previous := asyncCircuitNow
	current := time.Unix(1_800_000_000, 0)
	asyncCircuitNow = func() time.Time { return current }
	t.Cleanup(func() {
		asyncCircuitNow = previous
	})
	return func() time.Time { return current }, func(duration time.Duration) { current = current.Add(duration) }
}
